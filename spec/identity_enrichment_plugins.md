# Spec: identity-enrichment plugins — turn emails and GitHub links into people

Status: planned, not yet implemented.

## Context

Following the `codescoring.ru` case study, we confirmed that `deeper`
currently extracts zero information about actual people from a target: no
plugin ever produces `entities.Name`, and the only trace types consumed as
plugin *input* anywhere in the codebase are `Domain`, `Subdomain`, and
`Username` (confirmed by grepping every `state.RegisterPlugin` call site).
Everything `contact_crawler` finds — `Email`, `Phone`, `Github`, `Twitter`,
`Linkedin`, `SocialGeneric`, etc. — is a dead end in the discovery graph.

Two techniques were evaluated and researched against current API docs
(PGP-keyserver lookup and HaveIBeenPwned were also evaluated and dropped —
the former can structurally never return a name since `keys.openpgp.org`
strips all non-email User ID content on upload regardless of verification,
and the latter needs a paid API key and only returns breach names, never
personal data; neither is worth building for an identity-resolution goal).

## Plugin 1: `gravatar` — HIGH priority, real names, no auth required

Hash the email with **SHA256** (Gravatar's current documented algorithm;
`https://gravatar.com/{hash}.json` — no API key needed) and fetch the public
profile. If a profile exists, it can include `displayName`, and (per the
richer, Bearer-token-authenticated **v3** endpoint,
`https://api.gravatar.com/v3/profiles/{hash}`) `first_name`, `last_name`,
`job_title`, `company`, and `verified_accounts` (a list of the person's other
verified social profiles — a second source of new graph edges, not just a
name).

**Unresolved by docs, to verify empirically during implementation, not
guessed here:** whether the v3 endpoint returns the full field set
unauthenticated or requires a Bearer token for anything beyond `display_name`,
and the exact HTTP status for "no profile" (404 assumed, standard REST
convention, but not documented). First implementation step is a manual
`curl` against a known public Gravatar (e.g. a maintainer's published email)
to settle this before writing the parser.

```go
const InputTraceType = entities.Email

type profileFetcher interface {
    Get(ctx context.Context, url string) (*http.Response, error)
}
```

- `emailHash(email string) string` — lowercase + trim, then SHA256 hex (per
  current Gravatar docs; NOT MD5 — that's the legacy/deprecated algorithm).
- `fetchProfile(ctx, fetcher, hash string) (*gravatarProfile, bool, error)` —
  GETs the profile endpoint; `bool` is "profile exists" (false on 404-class
  responses, treated as "no signal," not an error — same convention as
  `contact_crawler`'s seed-error handling).
- Emit `entities.Trace{Type: entities.Name, Value: <first+last or displayName>}`
  only when a real name is present and isn't just an echo of the email
  local-part (avoid trivial noise, e.g. a profile whose displayName is
  literally the email itself).
- Emit `entities.Trace{Type: entities.Company, Value: company}` when present.
- If `verified_accounts` is available (may require the API key — verify
  empirically first): map known account types to existing social trace types
  (`Twitter`, `Github`, `Linkedin`, etc.), same domain-matching approach
  `contact_crawler/extract.go`'s `socialLinkTrace` already uses, so this
  plugin's output can chain into further discovery just like a scraped page
  would.

## Plugin 2: `github_identity` — HIGH priority, the actual flagship

This is the one that closes the loop. `entities.Github` traces are already
produced today (`contact_crawler` found
`https://github.com/CodeScoring/awesome-open-source-licensing` in the
`codescoring.ru` case) and are currently a dead end — nothing consumes
`entities.Github` as input anywhere. `GET /repos/{owner}/{repo}/commits`
(verified against current GitHub REST API docs) returns, per commit, both
`commit.author.{name,email}` (the raw git identity, whatever the committer
configured locally — often a real name and a real, personal-format email)
and, at the root level, `author.login` — the actual GitHub username, when
GitHub could match the commit's email to a verified account.

That `author.login` is the highest-value output of either plugin: it's an
`entities.Username` trace, and `Username` is one of only three trace types
the rest of the codebase already knows how to expand
(`AcademicPapersPlugin`, `CodeRepositoriesPlugin`, `FacebookPlugin`,
`SocialProfilesPlugin` all consume it). So this plugin doesn't just find a
name — it reconnects a dead-end branch back into the entire rest of the
plugin graph.

Rate limits (verified against GitHub's current docs): 60 requests/hour
unauthenticated per IP, 5,000/hour with a personal access token. Given a repo
can have many commits, an optional token is close to required for real use,
not just a nice-to-have.

```go
const InputTraceType = entities.Github

type commitFetcher interface {
    Get(ctx context.Context, url string) (*http.Response, error)
}
```

- `parseOwnerRepo(githubURL string) (owner, repo string, ok bool)` — parse
  `entities.Github` trace values (repo URLs, e.g.
  `https://github.com/CodeScoring/awesome-open-source-licensing`, per what
  `coderepos` already produces) into `owner`/`repo`. Reject non-repo GitHub
  URLs (org root, user profile) — `ok=false`, `nil, nil` from `FollowTrace`.
- `fetchCommitAuthors(ctx, fetcher, owner, repo string) ([]commitAuthor, error)`
  — `GET /repos/{owner}/{repo}/commits?per_page=100`, one page only for v1
  (bound the cost; document that older history requires pagination as a
  documented limitation, not a silent gap). Attach
  `Authorization: Bearer <token>` header only if `GitHubToken` is non-empty.
- Per commit, extract:
  - `entities.Trace{Type: entities.Name, Value: commit.author.name}` —
    dedup by exact value within one call.
  - `entities.Trace{Type: entities.Email, Value: commit.author.email}` — a
    **new**, potentially personal-format email, worth feeding back into the
    `gravatar` plugin above on a later BFS iteration (the existing engine
    already handles this naturally: any new `Email` trace re-enters the
    graph and gets processed by every registered `Email`-consuming plugin).
  - `entities.Trace{Type: entities.Username, Value: author.login}` **only**
    when the root-level `author` object is present and non-empty (GitHub
    matched the commit to a real account) — this is the trace that
    reconnects into `AcademicPapersPlugin`/`CodeRepositoriesPlugin`/
    `FacebookPlugin`/`SocialProfilesPlugin`.
- Rate-limit handling: a 403/429 with `X-RateLimit-Remaining: 0` should be
  treated as a plugin error (surfaced, not silently swallowed) so it's
  visible in scan output rather than looking like "this repo has no commits."

## Config additions

Two new optional credentials, following the existing `os.Getenv` pattern in
`internal/pkg/config/config.go` (env vars read in `loadWorkerPoolConfig`-style
functions, all optional with safe zero-value defaults):

```go
// Config, new fields
GravatarAPIKey string // DEEPER_GRAVATAR_API_KEY — optional, raises v3 profile field coverage
GitHubToken    string // DEEPER_GITHUB_TOKEN     — optional, 60/hr -> 5,000/hr
```

Both plugins must work with these unset — empty token means "unauthenticated
request," not an error, exactly like `contact_crawler` working without any
config today.

## Package layout

```
internal/pkg/plugins/gravatar/
  main.go          # plugin shell, NewPlugin, Register (Email), FollowTrace, String
  main_test.go
  profile.go       # hash + fetch + parse
  profile_test.go

internal/pkg/plugins/github_identity/
  main.go          # plugin shell, NewPlugin, Register (Github), FollowTrace, String
  main_test.go
  commits.go       # owner/repo parsing + commit-list fetch + author extraction
  commits_test.go
```

Both follow the `dns_resolver`/`contact_crawler` template: pointer-receiver
struct, `NewPlugin()` wiring the real HTTP dependency via
`deeperhttp.NewClient(config.LoadConfig())` (the existing shared client —
retry + rate limiting, already the documented pattern in
`.cursor/rules/plugin-development.mdc`), an injectable interface at the
network boundary so tests never touch real HTTP, real dependency in
`NewPlugin()`, fake struct literal in tests.

## Test plan (TDD order, per plugin)

1. Pure-function tests first: `emailHash` (known email → known SHA256 hex,
   verify against a hand-computed value, not a guess), `parseOwnerRepo`
   (valid repo URL, org-root URL rejected, malformed URL rejected).
2. Fetch/parse tests via a fake `profileFetcher`/`commitFetcher`: profile
   found with full fields; profile not found (no traces, no error); malformed
   JSON (no panic); name-equals-email-local-part suppressed (gravatar only);
   commit with matched GitHub login vs. commit with only raw git identity
   (github_identity only) — assert `Username` trace is only emitted in the
   matched case.
3. `main_test.go`: wrong input trace type → `nil, nil`; `String()`; optional
   token/key present vs. absent (assert the `Authorization` header is set
   only when configured).
4. **Not automated**: a real call against a known public Gravatar/GitHub repo
   as a manual smoke check, same convention as `contact_crawler`'s deferred
   live-smoke-test step.

## Verification

1. `go build ./...`, `go vet ./...`, `golangci-lint run ./...` clean.
2. `go test -race ./...` green, including all new plugin test files.
3. `./build/deeper plugins list` shows the new plugins under `email` and
   `github` (new rows in that table — today only `domain`/`subdomain`/
   `username` have any registered consumer).
4. Manual smoke run against `codescoring.ru`'s already-discovered
   `https://github.com/CodeScoring/awesome-open-source-licensing` repo —
   confirm real commit-author names/emails/usernames come back, and that any
   discovered `Username` trace correctly re-enters the graph and gets picked
   up by the existing username-consuming plugins on the next BFS iteration.

## Critical files referenced

- `internal/pkg/entities/entities.go` — `Name`, `Company`, `Email`,
  `Username`, `Github` trace types already exist; no vocabulary additions
  needed.
- `internal/pkg/plugins/dns_resolver/main.go`, `contact_crawler/main.go` —
  plugin template to follow.
- `internal/pkg/plugins/coderepos/main.go` — confirms the exact format of
  `entities.Github` trace values this spec's `github_identity` plugin parses.
- `internal/pkg/http/client.go` — shared HTTP client to reuse.
- `internal/pkg/config/config.go` — where the two new optional credential
  fields and their `DEEPER_*` env vars get added.
- `internal/pkg/state/state.go` — confirms today only `Domain`/`Subdomain`/
  `Username` have any registered consumer; this spec adds `Email` and
  `Github` as newly-consumed types.
