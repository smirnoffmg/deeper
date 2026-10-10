# Spec: the implicit namesake hop

Status: **decided: Option 3 (platform-qualified seed) on top of the shared base.** Builds on `spec/evidence_strength.md` (uncommitted work in the tree: `Strength`/`Basis`, `plugins.EvidenceFollower`, `engine.admit`).

## Problem

`github_profile`, `github_keys`, `habr_profile` and `coderepos` register on `entities.Username` (github_profile/main.go:21, github_keys/main.go:22, habr_profile/main.go:21, coderepos/main.go:15,30). Each of their edges `Username U -> Name/Email/Repository` silently contains the claim "the account U on platform P is the target". Nothing records it, and since these plugins do not adopt `EvidenceFollower`, the edge defaults to `strong`. This is the same guess `social_profiles` now labels `weak` (social_profiles/main.go `FollowEvidence`, basis `"sherlock <errorType> check"`). Deciding whether two records "represent the same person" is entity resolution, and the general case is "a hard problem" (*Database Systems: The Complete Book*, с. 1079, PDF 1116). A matching handle is the weakest form of that evidence.

## Facts from the code

- `entities.Github` exists (entities.go:50). It is emitted only as a URL: by `gravatar` from verified accounts (gravatar/profile.go:250,266) and by `contact_crawler` for in-scope owners (extract.go:25, crawl.go:228). The seed classifier never produces it, because `https://github.com/U` hits `isUrl` first (entities.go:278).
- `github_identity` is the only consumer of `Github` (main.go:25). `parseOwnerRepo` requires exactly `owner/repo` (commits.go:37-40), so a **Github account URL (one path segment) is a dead end today**. Only repo links get mined.
- `github_identity` emits `author.Login` as a bare `Username` (commits.go:276). A login is a known GitHub account. Re-entering it as a bare `Username` also runs `habr_profile`, gitlab/bitbucket `coderepos`, `social_profiles` and `facebook` on it, which are real namesake hops.
- `coderepos` emits GitHub, Bitbucket and GitLab repos from one bare handle (main.go:57-70). Only the GitHub ones have a consumer (`github_identity`). The others are recorded and go nowhere.
- `social_profiles` emits weak `SocialGeneric` for every Sherlock hit, which may include `github.com/U` and Habr if the remote data.json lists them (not checked offline). No plugin consumes a GitHub `SocialGeneric`.
- `admit` (engine.go:205): a child is enqueued only if its **parent** is not weak. A node reached by a weak edge is therefore expanded once, and its children are recorded but not expanded. Strength along a path is the minimum (engine.go:188-191).

## Chains today (post evidence_strength)

| id | chain | today |
|---|---|---|
| U1 | U -> github_profile -> Email/Company/Url/Twitter -> gravatar / companyregistry / url_resolver -> Domain -> whois, dns, crt.sh, crawl | full |
| U2 | U -> github_keys -> SSHKey/PGPKey/verified Email -> gravatar ... | full |
| U3 | U -> coderepos -> Repository(github) -> github_identity -> Name/Email/Username L -> everything on L | full, unbounded |
| U4 | U -> coderepos -> Repository(gitlab/bitbucket) | recorded, no consumer |
| U5 | U -> habr_profile -> Name/Company/Address/DOB -> companyregistry | full |
| E1 | E -> gravatar -> Github `github.com/U` (verified account) | dead end |
| D1 | D -> contact_crawler -> Github `github.com/o/r` -> github_identity -> L -> U1..U5 on L | full |
| D2 | D -> contact_crawler -> Github `github.com/o` | dead end |

The constraint from the brief, made concrete: if the four plugins are re-keyed on a **weak** account node A, a bare-username scan expands A once. The profile, keys and repos are recorded as weak but not enqueued. `github_identity` never runs, so U3 loses not just depth but **every commit author** (they are not even recorded). U1, U2 and U5 lose everything after the first hop.

## Shared base (all options build on it)

1. **New plugin `namesake`** (`internal/pkg/plugins/namesake/`, registered on `Username`, implements `EvidenceFollower`). It checks the account exists with one GET per platform and emits:
   - `Github https://github.com/U`, weak, basis `"namesake: github.com/U exists"`;
   - `SocialGeneric https://habr.com/ru/users/U`, weak (no trailing slash: the same shape as Sherlock's data.json `habr` entry, so the two dedupe), basis `"namesake: habr.com/users/U exists"`.
2. `github_profile` and `github_keys` re-register on `entities.Github` and accept one-segment URLs. A new helper `githubLogin(value) (string, bool)` sits next to `parseOwnerRepo` (commits.go:26). Do not share a package for it yet: three similar lines are fine.
3. `coderepos` registers on both types. On `Github` it lists GitHub repos (strong, basis `"repos of github.com/U"`). On `Username` it lists only gitlab/bitbucket repos as `weak`, with a `"namesake"` basis. That is an `EvidenceFollower`, about 20 lines.
4. `habr_profile` moves to `SocialGeneric` + a `Matches` host filter, the same pattern as codeforces_profile/main.go:20-27.
5. `github_identity` emits `Github https://github.com/<login>` (strong, basis `"commit author login"`) **in addition to** `Username L` (commits.go:276). GitHub data about L stays strong, and habr/gitlab on L become weak namesake hops.
6. Side effects:
   - Fix the comment at config.go:84 ("6 plugins register on Username").
   - Update catalog.go:61-73 to add `namesake.NewPlugin`.
   - health.go:217-220 still finds plugins on `Username`.
   - E1 and D2 open up as a by-product, because GitHub plugins now consume account URLs.

Size of the base: about 150 lines of production code and about 200 lines of tests.

## Option 1: uniform weak (base only)

Every namesake edge is weak, including the one from the seed. This is the same policy the evidence spec applied to Sherlock hits from the seed.

- (a) seed=username: U1, U2 and U5 keep the first hop (recorded, weak) and lose everything after it. **U3 is lost entirely.** U4 is unchanged but now labelled weak.
- (b) seed=email: **gains.** E1 becomes Github(U) strong -> profile, keys, repos -> commits -> Github(L) strong, the full chain. habr/gitlab on L get one weak hop.
- (c) seed=domain: D1's GitHub part is preserved through step 5. habr/gitlab/bitbucket/social on L drop from full to one weak hop, which is the intended fix. D2 opens.
- Cost: the default `deeper scan <handle>` stops mining commits. That is the most valuable chain, and also the one that leaks strangers' data when the handle is someone else's.

## Option 2: seed-handle trust (base + engine exception)

The user typed U, so namesake edges **out of the seed** are promoted to `strong`, with basis suffix `" (seed handle)"`. Usernames found further down keep weak namesake edges.

- Change: a pure `trustSeedNamesake(seed Trace, ds []Discovery) []Discovery` rewrites discoveries whose `Parent == seed && PluginName == namesake`. Call it in engine.go between `processBatch` (l.83) and `PersistDiscoveries` (l.85), so the stored edge carries the promoted strength. About 15 lines of code and 40 of tests.
- (a) U1, U2, U3 and U5 are preserved for U itself. For logins L found in commits, GitHub is strong (step 5) and habr/gitlab are weak. (b) and (c) are the same as Option 1.
- Cost: the engine learns a plugin name. Calling it strong because "the user typed a bare handle" is still a guess labelled `strong`, and it contradicts the evidence spec, where Sherlock's `keybase.io/U` from the same seed stays weak. Existence of `github.com/U` is no better evidence of ownership than a Sherlock 200.

## Option 3: platform-qualified seed (base + classifier)

Weak stays weak. To assert the platform, the user seeds the account itself: `deeper scan https://github.com/U`.

- Change: add `isGithubAccountURL(value)` (host `github.com`/`www.github.com`, one or two path segments) and a `case` before `isUrl` at entities.go:278 that returns `Github`. The seed edge is then `proof`, and the GitHub plugins run strong. Document it in README and the MCP tool description. About 15 lines of code and 30 of tests.
- (a) A bare handle behaves as Option 1. With a `github.com/U` seed, U1, U2 and U3 are preserved in full. U5 (habr) does not run, because there is no Username; the user seeds `habr.com/...` separately or adds the handle. (b) and (c) are the same as Option 1.
- Cost: an extra step for the common case. Mitigation: when a scan ends with a weak `namesake` Github node that has recorded repos, the report shows a "re-scan as github.com/U to expand" hint. This is optional and not in the estimate.

## Test plan

- Plugins: unit tests with a fake fetcher. `namesake`: one finding per existing account, weak, with basis. Nothing on 404. A failed probe is logged when the other platform confirmed an account, and returned as the plugin error only when nothing was found (so a rate-limit 403 is visible rather than a silent empty result).
- github_profile/keys: accept `Github` account URLs, reject repo URLs. coderepos: Github -> strong github repos, Username -> weak gitlab/bitbucket. habr: `Matches` host filter. github_identity: emits `Github` + `Username` per login.
- Entities (Option 3): a table test for `NewTrace` covering `github.com/U` and `https://www.github.com/U/` -> Github, `github.com/U/r` -> Github, `github.com` -> Domain, and other URLs unchanged.
- Engine: an `admit`-level test with a fake processor. Bare U -> namesake(weak) -> Github(U) -> Repository is not enqueued (documents the U3 loss for Options 1/3). Email seed -> Github(U) strong -> Repository enqueued -> login emits Github(L) enqueued.
  - Option 2 adds: the seed's namesake edge is persisted `strong`, and a non-seed namesake edge stays `weak`.
- Regression: the D1 chain keeps reaching `github_profile` on L through the typed Github node.

## Recommendation

**Option 3.** It keeps the evidence spec's single rule (weak is expanded one hop, no exceptions, no plugin names in the engine) and treats GitHub the way Sherlock hits already are. It still lets the analyst state "this account is the target" explicitly, as a `proof` seed. Email and domain scans strictly gain (E1, D2 open, and D1 is kept through typed logins). The only loss is the depth of a bare-handle scan, which is the namesake case the README warns about. If bare-handle depth turns out to matter for MCP clients, Option 2 is additive on top of Option 3 and can be added later without rework.

Estimate for Option 1 + 3: about 165 lines of code and about 230 lines of tests across 8 packages.
