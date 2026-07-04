# Spec: `contact_crawler` plugin — extract contact traces from crawled pages

Status: implemented (`internal/pkg/plugins/contact_crawler/`). Companion to `spec/troubles.md` (bug list) — this is a new-feature spec.

**Post-implementation correction**: an earlier version of this code scoped `sameSite` by DNS IP-address overlap instead of registrable-domain comparison. That let unrelated domains sharing infrastructure (e.g. sites behind the same CDN/shared host) get treated as in-scope, and could wrongly exclude legitimate same-domain subdomains resolving to different IPs. Fixed to compare `registrableDomain(seed) == registrableDomain(candidate)` as originally specified below; the `ipResolver`/`siteChecker` machinery was removed.

## Context

Following the OSINT case against `codescoring.ru` (Domain → Subdomain → IpAddr, via the `subdomains`, `crtsh`, and `dns_resolver` plugins), the next step is to fetch the discovered hosts' web pages and extract contact-info traces from them — emails, phone numbers, and social-media links.

Two things make this non-trivial:
1. **No HTML parser exists in this repo.** Everything else (`entities.go`) is raw regex over already-isolated single values, never over a full document.
2. **The codebase has zero crawl-safety rails.** Confirmed by inspection: `entities.Trace` has no depth field, the BFS in `internal/app/deeper/engine/engine.go` has no depth/count cap, the `--depth` CLI flag (`cli/scan.go`) is a documented no-op, and the per-domain rate limiter (`internal/pkg/workerpool/domain_rate_limiter.go` + `domain.go`) is effectively broken for real plugin payloads — `ExtractDomain` string-sniffs `fmt.Sprintf("%v", task.Payload)` against anchored patterns, but production payloads are `*tasks.TraceProcessingTask` pointers that stringify as Go struct dumps matching nothing, so real traffic falls into one shared `"default"` bucket. Since this plugin will run against a real, permissioned but live external site, "follow links" needs its own bounded-crawl safety logic rather than relying on anything the engine already provides.

## Decisions

- Follow links, but as a **plugin-internal bounded crawl** (own depth/page caps) — do NOT wire `entities.Url` into the global BFS engine as a new consumable `InputTraceType`. Keeps the recursion fully contained in one plugin, independent of the engine's confirmed-absent safety nets. (`entities.Url` is currently a dead end — no plugin registers it as input; keep it that way.)
- Extract **email + phone + social handles** (Twitter/X, GitHub, LinkedIn, Instagram, Facebook, TikTok, Reddit, YouTube, Pinterest, Snapchat, Tumblr) — not email-only.
- Parse HTML with **`github.com/PuerkitoBio/goquery`** (`NewDocumentFromReader`, `.Find()`/`.Text()`/`.Attr("href")`/`.Remove()`), not hand-rolled tag-stripping regex.
- goquery v1.12.0 requires **Go 1.25+**. Bump this repo to **Go 1.26** (confirmed installed locally as 1.26.4) rather than pinning an older goquery release.

## Package layout

```
internal/pkg/plugins/contact_crawler/
  main.go          # plugin shell: struct, NewPlugin, Register, FollowTrace, String
  main_test.go     # FollowTrace-level tests, dns_resolver style
  crawl.go         # bounded crawl algorithm + fetch/parse orchestration
  crawl_test.go    # crawl-algorithm tests via a fake fetcher
  host.go          # registrable-domain / same-site scoping helpers
  host_test.go     # dedicated safety tests for host scoping
  extract.go       # unanchored email/phone regexes + mailto:/tel:/social-href parsing
  extract_test.go  # table-driven extraction tests
```

Follow `internal/pkg/plugins/dns_resolver/main.go` (the newest, TDD-authored plugin) as the template: pointer-receiver struct, `NewPlugin()` constructor wiring the real dependency, an **injectable interface for the network boundary** so tests never touch real HTTP:

```go
type pageFetcher interface {
    Get(ctx context.Context, url string) (*http.Response, error)
}
```

This signature matches `internal/pkg/http.Client.Get` exactly, so `deeperhttp.NewClient(cfg)` (the existing shared client — retry + rate limiting; already documented as the required pattern in `.cursor/rules/plugin-development.mdc`, though no existing plugin actually uses it) satisfies it with zero adapter code. Real dependency wired in `NewPlugin()`; tests inject a hand-rolled fake struct via struct literal, exactly like `dns_resolver/main_test.go`'s `fakeResolver`.

Registers for **two** input trace types (a first for this codebase — every existing plugin has exactly one):

```go
func (p *ContactCrawlerPlugin) Register() error {
    state.RegisterPlugin(entities.Domain, p)
    state.RegisterPlugin(entities.Subdomain, p)
    return nil
}
```

## Bounded crawl design (`crawl.go`)

- **Same-host scoping**: compare against the *registrable domain* (eTLD+1) via `golang.org/x/net/publicsuffix.EffectiveTLDPlusOne` — not naive suffix matching (`strings.HasSuffix(host, "codescoring.ru")` would wrongly accept `evilcodescoring.ru` / `codescoring.ru.evil.com`; this is exactly the bug class to avoid on a live target). `x/net` arrives as a transitive dependency of goquery.
- **Depth cap**: `defaultMaxDepth = 2` (seed page, its direct links, one further hop — covers `/contact`, `/about`, footer links without descending into unrelated content trees).
- **Page cap**: `defaultMaxPages = 20` per `FollowTrace` call.
- **Per-call visited-URL dedup**: local `map[string]bool`, fragment-stripped (not query-string-stripped — documented limitation, acceptable given the page cap bounds the worst case).
- **Cross-invocation safety** (a real gap beyond simple per-call bounding): the engine calls `FollowTrace` concurrently across multiple discovered subdomains of the *same* site (confirmed by the existing `TestProcessor_ProcessTrace_Concurrency` test in `internal/app/deeper/processor/processor_test.go`). Mitigate two ways, both leaning on the plugin being a process-wide singleton exactly like `dns_resolver`:
  1. The shared `deeperhttp.DefaultClient` constructed once in `NewPlugin()` has one ticker-based rate limiter (`internal/pkg/http/client.go:41,71`) — every concurrent crawl call funnels through the same gate.
  2. A mutex-protected `domainBudget map[string]int` on the plugin struct caps total pages ever fetched per registrable domain across the whole process run (`maxPagesPerRegistrableDomainPerProcess = 60`), so N discovered subdomains of one site can't each independently spend the full 20-page budget.
- **Fetch/parse**: `goquery.NewDocumentFromReader(resp.Body)`; strip `doc.Find("script, style, noscript").Remove()` before text extraction; resolve `<a href>` via `net/url` relative-to-absolute resolution.
  - `mailto:` hrefs → parsed directly into `Email` traces (strip prefix, drop `?query`) — precise, not enqueued as pages.
  - `tel:` hrefs → parsed into `Phone` traces the same way.
  - Everything else: must be http(s), same registrable domain, unvisited, within depth budget, within the process-wide page budget → enqueue.
- **Errors**: seed-page fetch failure → return the error (nothing collected yet, mirrors `dns_resolver`'s error-passthrough test). Mid-crawl page failure → log and skip, keep going — a dead link shouldn't discard already-collected contacts from other pages.
- Malformed HTML: goquery's underlying tokenizer follows WHATWG "forgiving" parsing and essentially never errors — tests should assert best-effort partial extraction, not an error return.

## Extraction (`extract.go`)

New unanchored regexes live here, **not** exported from `entities.go`. `entities.go`'s `isX` functions are unexported, `^...$`-anchored for whole-string classification of an already-isolated value, and not reusable unanchored without forking the pattern text anyway (anchors make `FindAllString` useless as-is) — sharing the *function* buys nothing and would wrongly couple `entities` (plugin-agnostic vocabulary) to one plugin's document-scanning needs.

- `extractEmails(text string) []string` — unanchored, word-boundary email regex.
- `extractPhones(text string) []string` — unanchored, conservative; document in-code as best-effort/noisy (freeform phone regex over prose risks false positives on version strings, zip+4 codes, dates).
- `mailtoTrace` / `telTrace(href string) (entities.Trace, bool)` — precise, href-based.
- `socialLinkTrace(href string) (entities.Trace, bool)` — matches known social domains in `<a href>` (twitter.com/x.com, linkedin.com/in/, github.com, facebook.com, instagram.com, t.me, etc.) → typed traces.
- **Scope reduction**: social handles are extracted **only** via href domain-matching, not free-text `@handle` regex scanning. `entities.go`'s handle patterns (`^@[a-zA-Z0-9_]{1,15}$` etc.) were built for validating an isolated value; unanchored over prose they'd false-positive on CSS `@media`, email local-parts, JS/template `@` syntax. Href matching is both more reliable and consistent with the mailto/tel precision argument.

## Dependency & CI changes

- `go.mod`: bump `go 1.23.0` → `go 1.26`; `go get github.com/PuerkitoBio/goquery@v1.12.0 && go mod tidy` (pulls in `cascadia` + `golang.org/x/net`).
- `.github/workflows/go.yml`: bump all **three** `go-version: '1.22'` occurrences (test/build/lint jobs) → `'1.26'`.
- `README.md:44`: `"Go 1.22 or higher"` → `"Go 1.26 or higher"`.
- Follow-up to check (not guess): `golangci-lint-action`'s pinned `version: v1.58` predates Go 1.26 — verify the lint job actually passes after the bump; bump the pinned version too if it doesn't.
- `cmd/deeper/main.go`: add `_ "github.com/smirnoffmg/deeper/internal/pkg/plugins/contact_crawler"` to the blank-import block.

## Test plan (TDD order: host → extract → crawl/main)

1. **`host_test.go`** first (pure functions, safety-critical): registrable-domain extraction for subdomains/case/IP-literal/empty; `sameSite` true for same-registrable-domain, false for unrelated domains, **false for the `codescoring.ru.evil.com` suffix-trap case** (dedicated regression test).
2. **`extract_test.go`**: table-driven email/phone matches and non-matches; `mailto:`/`tel:` href parsing; social-domain href matching per platform.
3. **`crawl_test.go` / `main_test.go`** (dns_resolver style, fake `pageFetcher` recording call order): wrong trace type → `nil,nil`; single page with text-email + mailto-email, deduped; links within budget followed; links beyond `defaultMaxPages` stop the crawl at the cap (assert exact fetch count); links beyond `defaultMaxDepth` never fetched; external-host links never fetched; mid-crawl fetch error skips that page without aborting the rest; seed fetch error returns the error; malformed HTML doesn't panic; `String()`; (if implementing the cross-invocation budget) two calls for two subdomains of one site together respect the shared per-domain cap.
4. **Explicitly not automated**: a real live call to `codescoring.ru`. That's a manual, deliberately-gated smoke check to run once by hand at the time of the actual permissioned engagement — not part of `go test ./...`/CI.

## Verification

1. `go build ./...` and `go vet ./...` clean after the Go 1.26 bump and goquery addition.
2. `go test -race ./...` green, including all new test files above.
3. `golangci-lint run ./...` — confirm it still runs correctly under Go 1.26.
4. Manual smoke run against `codescoring.ru` (the same permissioned target used earlier), comparing output against the existing Domain→Subdomain→IpAddr baseline — confirm new Email/Phone/social traces appear, confirm request volume stays within the documented caps, confirm no requests are made to any non-`codescoring.ru` host.
5. Re-run the full end-to-end scan (`./build/deeper scan codescoring.ru`) once more to confirm the new plugin composes cleanly with the existing plugin graph and doesn't regress the earlier ~5.8s baseline unboundedly (some increase is expected — real page fetches — but the depth/page caps exist precisely to keep it bounded).

## Critical files referenced

- `internal/pkg/plugins/dns_resolver/main.go`, `main_test.go` — the template to follow
- `internal/pkg/entities/entities.go` — trace vocabulary, existing (non-reusable) regex patterns
- `internal/pkg/http/client.go` — shared HTTP client to reuse
- `internal/app/deeper/engine/engine.go` — confirms no depth cap exists at the engine level (why this plugin self-bounds)
- `go.mod`, `.github/workflows/go.yml`, `README.md:44` — version bump sites
- `cmd/deeper/main.go` — plugin wiring

## Open item deferred from this spec

DB/graph-modeling work (persisting which trace led to which — an edges/provenance table, discussed separately) is a distinct task, not covered here.
