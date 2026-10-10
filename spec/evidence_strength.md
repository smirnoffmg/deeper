# Spec: evidence strength on discovery edges

Status: planned, not yet implemented. Builds on `spec/trace_graph.md` (the `trace_edges` graph this extends).

## Context

The README promises: "A guess is not a fact ... Deeper treats that as weak evidence, not a discovery." The code has no notion of evidence strength. `entities.Trace`, `entities.Discovery` and `trace_edges` carry no strength field, so a plugin can either emit a finding as full fact or drop it. Recent fixes (`0a46c96`, `5754679`, `e25a6ca`, `9d7ec7f`) all dropped garbage at emit time because no third option existed.

The cost of treating everything as fact shows up in the BFS. `engine.ProcessInput` (engine.go:86-92) enqueues every new child. One Sherlock false positive (`social_profiles` → `SocialGeneric`) is therefore expanded by the profile plugins (codeforces, keybase, ...), and their scraped `Company`/`Name` is expanded again. That gives the wrong-person chain username → namesake account → company → registry → domains. The same thing happens via `facebook`/`academic_papers` `Url` → `url_resolver` → `facebook.com` / `semanticscholar.org` `Domain` → whois, crt.sh, crawling.

The `SocialGeneric` chain has to survive, though. The profile plugins registered under `SocialGeneric` (telegram, keybase, codeforces, launchpad, linuxorgru, crowdin, bluesky) confirm that the account exists and scrape what it says about itself. So "never expand a weak trace" would break the main thing a username scan is for. `gravatar` and `contact_crawler` also *emit* `SocialGeneric` (t.me links); they keep the `strong` default here, so those accounts stay fully expanded. That is accepted and deferred with their labelling.

## Decisions

- **Strength is a three-level ordered scale on the edge, `weak < strong < proof`, plus a short free-text `basis`.**
  - `proof`: the source binds the child to the parent by itself (a signed proof, a DNS answer, a CT log entry, the seed).
  - `strong`: an authoritative-enough source asserts it (an account's own profile fields, a registry, a passive dataset).
  - `weak`: existence or co-occurrence only (an HTTP-status heuristic, a search hit, a fuzzy name match).
  - It is an attribute on the edge, not a separate provenance node. Hogan et al. describe the property-graph option as one "where the temporal context is defined as an attribute on the edge" (*Knowledge Graphs*, с. 21, PDF 21).
  - The level is stored rather than filtered at emit time, so the report and a later policy change can reinterpret old scans.
- **Path strength = the weakest edge on the path. A node's strength = its best path.** Annotated RDF combines annotations with "meet and join" over a value domain (*Knowledge Graphs*, с. 22, PDF 22). On an ordered scale those are min and max. That min-along-a-path / max-across-paths reading is **our inference**: no corpus passage says "weakest link" outright.
- **Weak nodes get exactly one further hop.** A child is enqueued only if its *parent* is not weak. This runs the profile plugins on a Sherlock hit, but whatever they emit stays weak and is recorded, not expanded. Hop count 1 is the smallest that keeps the `SocialGeneric` chain alive; it is a policy choice, not a derived constant.
  - Pivots given up from a Sherlock-found account: its Email → `gravatar` (telegram_profile/profile.go:79, bluesky_profile/profile.go:96) and its bio/link Url → `url_resolver` → personal Domain (telegram_profile:82, bluesky_profile:99, keybase_profile:126). Both are still recorded, just not expanded.
- **The "proof that binds back to a strong node" exception is dropped as a separate rule.** The max-across-paths rule already covers it: if the proof's target is already strong in the graph, it stays strong. A proof edge out of a *weak* node (keybase proofs from a Sherlock-found keybase account) shows that the linked account belongs to whoever owns the keybase account. It says nothing about the target, so its path stays weak. No special case is needed.
- **Default for plugins that don't opt in: `strong`.** A non-adopting plugin behaves as today when its input is strong; under a weak node (e.g. a profile plugin on a weak `SocialGeneric`) its outputs are recorded but not expanded, which is the point. A `weak` default would cut the DNS/CT chains after two hops. A `proof` default would overstate passive datasets. Existing rows migrate to `strong` for the same reason: that is what the code that wrote them meant.
- **Per-finding strength, not a per-plugin constant.** `social_profiles` already needs it: its basis differs per Sherlock entry (`errorType`, main.go:180-188). Mixed plugins such as companyregistry ("nearby enterprises" vs. INN) can only be told apart inside `FollowTrace`, so a per-plugin `Evidence()` would need a second interface later.
- **Seed edge = `proof`, basis `"seed"`.** The user typed it.

## Design

### entities (`internal/pkg/entities/entities.go`)

```go
type Strength string

const (
    StrengthWeak   Strength = "weak"
    StrengthStrong Strength = "strong"
    StrengthProof  Strength = "proof"
)

type Discovery struct {
    Parent     Trace
    PluginName string
    Child      Trace
    Strength   Strength
    Basis      string
}
```

`Trace` stays strength-free, because it is the BFS/dedup map key.

### Plugin contract (`internal/pkg/plugins/base.go`)

```go
type Finding struct {
    Trace    entities.Trace
    Strength entities.Strength
    Basis    string
}

// EvidenceFollower is optional, like ActivePlugin/TraceMatcher. When a plugin
// implements it, the processor calls it instead of FollowTrace.
type EvidenceFollower interface {
    FollowEvidence(ctx context.Context, trace entities.Trace) ([]Finding, error)
}
```

An adopting plugin keeps `FollowTrace` (required by `DeeperPlugin`) as a one-line strip of `FollowEvidence`.

### Processor (`processor.go`)

- `newTraceTaskHandler` (l.234-272) type-asserts `plugins.EvidenceFollower`, just as `TraceMatcher` is asserted at l.105. If the plugin implements it, the handler calls `FollowEvidence`. Otherwise it wraps the `FollowTrace` output as `Finding{Strength: StrengthStrong}`.
- The processor is the single normalisation point: any `Strength` other than `weak`/`strong`/`proof` (empty included) becomes `strong`; a non-empty unknown value is logged once per plugin.
- `pluginTraceResult.Traces` becomes `[]plugins.Finding`.
- The Discovery literal (l.152-159) copies `Strength`/`Basis`.

### Engine (`engine.go`)

- The seed `InsertEdge` (l.53-58) sets `Strength: entities.StrengthProof, Basis: "seed"`.
- `seen map[Trace]bool` currently answers both "is it recorded?" and "is it enqueued?". It is replaced by an internal `nodes map[Trace]*nodeState{strength Strength; queued bool}`. A `children map[Trace][]Discovery` holds every discovery seen so far. Node strength stays inside the engine; `ProcessInput` still returns `[]entities.Trace`.
- An unexported `rank(Strength) int` (weak 0, strong 1, proof 2) is the only ordering helper.

The expansion block (l.86-92) calls one pure function per discovery:

```go
// admit applies d to nodes and returns the traces that newly become enqueueable.
func admit(nodes map[Trace]*nodeState, children map[Trace][]Discovery, d Discovery) []Trace
```

Rule:
1. Append `d` to `children[d.Parent]`. `s :=` the lower of `nodes[d.Parent].strength` and `d.Strength`.
2. If the child is new, record it with `s` and append it to results. If it is known and `s` ranks higher than its strength, **promote** it.
3. Always: if the child is not yet `queued` and the parent is not weak, mark it `queued` and return it. This is independent of step 2, so a known child reached later from a non-weak parent is enqueued even when its strength does not change.
4. On promotion only, re-run `admit` over `children[child]`. Batches are FIFO and `processBatch` appends in goroutine completion order (l.123-133), so a weak path can arrive before a strong one. Re-running lets a promoted node's recorded children be promoted and, via step 3, enqueued.

This terminates because a node can be promoted at most twice. The seed starts as `{proof, queued}`.

Every discovery is still persisted (spec/trace_graph.md's "seen must not gate persistence").

### Persistence

New `internal/pkg/database/migrations/00003_edge_evidence.sql`. It is goose-embedded and applied by `NewDatabase` through `migrate.go`, like 00001/00002:

```sql
-- +goose Up
ALTER TABLE trace_edges ADD COLUMN strength TEXT NOT NULL DEFAULT 'strong';
ALTER TABLE trace_edges ADD COLUMN basis TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE trace_edges DROP COLUMN basis;
ALTER TABLE trace_edges DROP COLUMN strength;
```

- `DROP COLUMN` needs SQLite ≥ 3.35. go-sqlite3 v1.14.32 bundles 3.50.4 (checked: `SQLITE_VERSION` in the module's `sqlite3-binding.c`).
- No CHECK constraint, because SQLite can't add one through `ALTER`. The Go constants are the only writers.
- The column `DEFAULT` only backfills existing rows: `InsertEdge` and `PersistDiscoveries` list their columns explicitly (graph.go:57-58, 139-141), so they write whatever they are given. Callers must set `Strength`; the processor and the seed edge are the only production writers and both do.
- `models.go` `TraceEdge` gets `Strength`, `Basis`.
- `graph.go`: `InsertEdge` and `PersistDiscoveries` write both columns, and `GetScanGraph` reads them.
- `GetDiscoveryPath`/`GetReachableTraces` have no production callers and stay unchanged.
- Known and accepted: the UNIQUE key does not include strength. If the same parent/child/plugin/scan is emitted twice with different strengths, `INSERT OR IGNORE` keeps the first one.

### Adopting plugins in this change

Only the plugins that are wholly weak and feed a known contamination path adopt the interface. All three return constant `weak`:

| Plugin | Basis | Path it stops |
|---|---|---|
| `social_profiles` | `"sherlock <errorType> check"` (per entry, main.go:180-188) | namesake account → profile scrape → Company → registry/domains |
| `facebook` | `"search hit: <query> site:facebook.com"` | Url → `url_resolver` → `facebook.com` Domain → whois/crt.sh/crawl |
| `academic_papers` | `"author name within edit distance"` | same, via `semanticscholar.org` |

All the other plugins keep the `strong` default. Under the current rule `strong` vs `proof` changes nothing about expansion, so labelling more plugins would only add report detail.

### Report

Edge strength only, read from `trace_edges`, so a past scan renders the same as a fresh one.

- `graphreport.Edge` gets `Strength`, `Basis`; `buildGraphReport` (scan.go:145-156) copies them from `TraceEdge`. Its signature does not change.
- Template (vendored vis-network 9.1.13, see `graphreport/vendor/NOTICE.md`; options per https://visjs.github.io/vis-network/docs/network/edges.html, both present in the vendored build's option schema):
  - weak edges get `dashes: true`;
  - edge `title` (already used at graph.html.tmpl:170): `via <plugin> · <strength>: <basis>`, set via `textContent`.
- Colour and opacity are deliberately left alone. `resetHighlight` (l.237-243) and the `selectNode` dimming (l.252-258) overwrite both on every edge and node.
- No filter toggle and no proof styling: the only proof edge is the seed edge, which `buildGraphReport` skips (`ParentTraceID == nil`).

## Test plan (TDD order)

1. `engine/admit_test.go` (pure, no DB):
   - weak edge from the proof seed → child weak and enqueued;
   - any edge from a weak parent → recorded, not enqueued;
   - weak-then-strong: a child first seen via a weak parent, then reached via a strong edge from a strong parent → promoted and enqueued;
   - same strength, non-weak parent: C recorded via a weak parent, then reached over a weak edge from a strong parent → C stays weak and is enqueued; run in both arrival orders, same final state;
   - promotion re-propagates: a weak parent with a recorded grandchild (weak edge) is promoted → the grandchild is enqueued;
   - an A↔B cycle with mixed strengths terminates.
2. `processor_test.go`: an `EvidenceFollower` fake (copy `matcherPlugin`, l.92-110) → its `Discovery` carries strength and basis. A plain plugin → `strong`, `""`. Empty strength → `strong`. Unknown strength `"bogus"` → `strong`.
3. `engine_test.go` (real temp DB via `setupEngine`):
   - a fake Username plugin emits a weak `SocialGeneric`. A profile fake on `SocialGeneric` emits a `Company` and an `Email`; `recordingPlugin`s on `Company` and `Email` (the gravatar slot) are **never called**. Both are still in the results and in `trace_edges`. Mirror case: the same chain with a strong edge → the recorders are called;
   - the seed edge read back through `GetScanGraph` is `proof`, `"seed"`.
4. `database/graph_test.go`:
   - `PersistDiscoveries` → `GetScanGraph` round-trips strength and basis;
   - migration upgrade (in-package test, temp-file DB): `goose.SetBaseFS(migrationFS)`, `goose.SetDialect("sqlite3")`, `goose.UpTo(db, "migrations", 2)`, insert an edge, close, then `NewDatabase(path)` → the edge reads `strong`, `""`.
5. Plugin tests:
   - `social_profiles` with fake `checkFn` → findings weak, basis names the errorType;
   - `facebook`/`academic_papers` with fake fetchers → weak.
6. `cli/scan_test.go` `TestBuildGraphReport_*`: edge strength/basis are mapped. `graphreport_test.go`: JSON round-trip with the new fields, plus a malicious `basis` in the escaping test.

## Verification

1. `go build ./...`, `go vet ./...`, `go test -race ./...` all green.
2. `./build/deeper scan <a username with known Sherlock false positives>`:
   - the report shows dashed SocialGeneric edges;
   - profile-scraped Names/Companies/Emails/Urls are present but have no outgoing edges;
   - `sqlite3 ~/.deeper/deeper.db "select strength,count(*) from trace_edges group by 1"` shows weak rows.
3. `./build/deeper scan codescoring.ru` (the baseline case from trace_graph.md), before and after the change, each with `DEEPER_DEDUP_PERSISTENT_CACHE=false` (the persistent dedup cache, 24h TTL, otherwise empties the second run). Diff the trace sets: the trace count is ≤ baseline, every missing trace descends from a `facebook`/`academic_papers` Url (reached via `live_host` page-title Names), and the `facebook.com`/`semanticscholar.org` Domains are recorded but unexpanded. Nothing else changes.

## Critical files

- `internal/pkg/entities/entities.go`: `Strength`, `Discovery` fields
- `internal/pkg/plugins/base.go`: `Finding`, `EvidenceFollower`
- `internal/app/deeper/processor/processor.go`: handler type-assert, normalisation, Discovery fields
- `internal/app/deeper/engine/engine.go`: `admit`, `rank`, node state, seed edge
- `internal/pkg/database/migrations/00003_edge_evidence.sql`, `models.go`, `graph.go`
- `internal/pkg/plugins/{social_profiles,facebook,academic_papers}/main.go`
- `internal/app/deeper/cli/scan.go`, `internal/app/deeper/graphreport/{graphreport.go,graph.html.tmpl}`

## Explicitly out of scope

- **The implicit namesake hop.** `github_profile`, `github_keys`, `habr_profile` and `coderepos` are keyed on a bare `Username`, so every edge they emit hides an unrecorded "account U on platform P is the target" claim. With the `strong` default they still feed the github Company → companyregistry path. Fixing that properly means re-keying them on an account node (`Github`/`SocialGeneric`) the way the profile plugins already are. That is a separate change, flagged as the next one.
- Labelling the proof/mixed plugins (keybase proofs, companyregistry INN vs. name, contact_crawler, gravatar, github_identity co-authors, ip_intel, live_host title, tech_detect).
- Node (path) strength outside the engine: in the report, CLI table/JSON/CSV, scanservice/MCP, or persisted.
- A "hide weak" toggle in the report.
- Strength ≠ relevance. Proof edges such as the SOA RNAME, provider NS/MX, CDN IPs and the AS-name Company remain noise that this change does not address.
