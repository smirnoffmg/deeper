# Troubles specification

Findings from a full-repo analysis on 2026-07-04 (commit `0e6da95`, branch
`main`). Each entry is self-contained: problem, evidence, required fix, and
how to verify the fix. Work top to bottom — #1 blocks CI and should land
before anything else.

**Status as of this pass: all 10 items resolved.** #1 (`.gitignore`), #3
(worker pool concurrency), #4 (orphaned `PluginRegistry`), and #5 (three
unregistered plugins) were fixed externally earlier in this effort. #2
(GoReleaser/CGO database outage) was fixed by removing `.goreleaser.yml`/
`.github/workflows/release.yml`. #6 (lint) is now clean (`golangci-lint run
./...` → 0 issues). #7 (README staleness) was already corrected. #8
(untracked files) — `TESTING.md`/`cache_test.go` are committed, no untracked
files remain. #9 (`SocialProfilesPlugin` registration failure) and #10
(worker pool result cross-attribution race) were found by independently
verifying `codescoring.ru` scan results against ground truth, and fixed in
this pass.

---

## 1. `.gitignore` swallows real source files under `internal/app/deeper/` [CRITICAL]

**Problem:** `.gitignore:144` contains a bare `deeper` entry, added to ignore
the local build binary (`./deeper`). Git matches a pattern with no `/` in it
at *any* directory depth, so it also matches every path containing a
directory component named `deeper` — including all of
`internal/app/deeper/**`. Three real, actively-imported source files exist
on disk but have never been committed:

- `internal/app/deeper/cli/rate_limit.go`
- `internal/app/deeper/cli/benchmark.go`
- `internal/app/deeper/processor/tasks/tasks.go`

**Evidence:**
```
$ git check-ignore -v internal/app/deeper/cli/rate_limit.go
.gitignore:144:deeper	internal/app/deeper/cli/rate_limit.go

$ git log --all --oneline -- internal/app/deeper/processor/tasks/tasks.go
(empty — never tracked)

$ git clone <repo> /tmp/x && cd /tmp/x && go build ./...
internal/app/deeper/processor/processor.go:9:2: no required module provides
package github.com/smirnoffmg/deeper/internal/app/deeper/processor/tasks
```
`gh run list` confirms every CI run on `main` since the 2025-08-22 refactor
is red, and this is the reason: a clean checkout (exactly what CI does)
cannot build.

**Fix:**
1. In `.gitignore`, change the line `deeper` to `/deeper` (anchors the
   ignore to the repo root, so it only matches the top-level binary, not
   `internal/app/deeper/`). Also consider `/build/deeper` if the binary is
   only ever produced under `build/`.
2. `git add internal/app/deeper/cli/rate_limit.go
   internal/app/deeper/cli/benchmark.go
   internal/app/deeper/processor/tasks/tasks.go` and commit.
3. Re-run `git status --ignored` and confirm no file under
   `internal/app/deeper/` appears in the ignored list.

**Verify:** clone the repo fresh (or `git stash -u && git clean -xdn` to
simulate) and run `go build ./...` — must succeed. Push and confirm the
GitHub Actions `Go CI` workflow goes green on `main`.

---

## 2. Officially released binaries cannot open the database at all [CRITICAL]

**Problem:** `.goreleaser.yml:7` builds every released binary (linux/windows/
darwin, amd64/arm64) with `CGO_ENABLED=0`. The database driver,
`github.com/mattn/go-sqlite3` (`go.mod`), is CGO-based with no pure-Go
fallback — when compiled without cgo it silently falls back to a
non-functional stub rather than failing the build, so the binary compiles
fine but cannot open the database at runtime.

**Evidence (reproduced directly):**
```
$ CGO_ENABLED=0 go build -o /tmp/deepertest ./cmd/deeper
# succeeds
$ /tmp/deepertest database info
Error: failed to connect to database: failed to run migrations: migration 1 failed:
Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work. This is a stub
```
This means every officially released `deeper` binary today has zero working
database functionality — the plugin-result cache (`cache_entries`, the only
currently-live DB usage — see `spec/trace_graph.md` for why `traces`/
`scan_sessions` are already dead code separately) silently never works
either, since it's the same broken connection.

**Fix (decided):** remove `.goreleaser.yml` and the `.github/workflows/release.yml`
automation entirely, rather than keep CGO disabled or reconfigure
cross-compilation toolchains. `CGO_ENABLED=0` only exists to make the
6-target (linux/windows/darwin × amd64/arm64) cross-compiled release matrix
buildable from a single CI runner without per-target C cross-compilers —
simply flipping it to `CGO_ENABLED=1` would break that same cross-compile
matrix (CGO cross-compilation needs a C toolchain per target, which isn't
configured anywhere in this repo). Since the project is dropping automated
multi-platform release binaries rather than taking on that toolchain
complexity, `mattn/go-sqlite3` (CGO-based) is fine to keep as-is — building
from source with plain `go build` on any single machine works today since
CGO is enabled by default there.

**Verify:** `.goreleaser.yml` and `.github/workflows/release.yml` no longer
exist; `go build -o build/deeper ./cmd/deeper` (default, cgo enabled) still
produces a working binary with functioning database access.

## 3. Worker pool never runs real plugin logic — no actual concurrency [HIGH]

**Problem:** `internal/pkg/workerpool/workerpool.go:313`, `Worker.processTask`,
is a stub:
```go
var err error
if task.ID == "failing-task" {
    err = fmt.Errorf("simulated failure for testing")
}
result := &TaskResult{TaskID: task.ID, Result: task.Payload, Error: err, ...}
```
It never touches `task.Payload` (a `*tasks.TraceProcessingTask` carrying the
plugin and trace). The real work — `plugin.FollowTrace(...)` — is instead
called back in `internal/app/deeper/processor/processor.go:142`,
synchronously, inside the loop that drains `p.workerPool.GetResult(ctx)`
(processor.go:122). So each plugin call still runs one at a time on the
calling goroutine; the pool only shuttles an inert payload through channels
and adds synchronization overhead. Separately,
`internal/app/deeper/engine/engine.go:83` (`processBatch`) loops over a
batch of traces with a plain `for _, trace := range traces` — sequential,
despite the comment above it claiming "Process batch concurrently".

**Evidence:** read `workerpool.go:301-345` (`Worker.run` / `processTask`)
alongside `processor.go:73-179` (`ProcessTrace`) — the payload is round-
tripped through the queue unexecuted, then `FollowTrace` is invoked
explicitly after the round trip.

**Fix:**
1. Move the real work into `Worker.processTask`: type-assert
   `task.Payload.(*tasks.TraceProcessingTask)`, call
   `payload.Plugin.(interface{ FollowTrace(...) }).FollowTrace(payload.Trace)`
   there, and put the *trace results* (not the untouched payload) into
   `TaskResult.Result`.
2. Simplify `Processor.ProcessTrace`'s result-collection loop
   (processor.go:120-161) to just read `result.Result.([]entities.Trace)`
   and `result.Error` — delete the second `FollowTrace` call entirely.
3. Change `engine.processBatch` (engine.go:78-100) to actually fan out
   `e.processor.ProcessTrace` calls concurrently (e.g. goroutines + a
   `sync.WaitGroup` or an `errgroup`, bounded by `e.config.MaxConcurrency`)
   instead of the sequential `for` loop — or, if the worker pool from step 1
   already bounds concurrency, drop the outdated comment so it matches
   the code.

**Verify:** add a test/benchmark that submits N traces whose plugins sleep
for a fixed duration, and assert wall-clock time is closer to
`N/MaxConcurrency * sleep` than `N * sleep`. Existing
`internal/pkg/workerpool/*_test.go` and `integration_test.go` must still
pass.

---

## 4. Orphaned `PluginRegistry` — populated by nobody, queried by nobody [HIGH]

**Problem:** `internal/pkg/plugins/registry.go`'s `PluginRegistry` (health
checks, per-plugin status, metadata — ~320 lines) is constructed via
`fx.Provide(providePluginRegistry)` and started via
`fx.Invoke(startupPluginRegistry)` in `internal/app/deeper/app.go`, but no
code anywhere calls `registry.RegisterPlugin`. All real plugin registration
goes through the separate global `internal/pkg/state.ActivePlugins` map
(populated by each plugin's `init()` → `Register()` →
`state.RegisterPlugin()`). The CLI commands that report plugin state
(`internal/app/deeper/cli/health.go`, `internal/app/deeper/cli/plugins.go`)
already read `state.ActivePlugins` directly and never touch the registry.
Net effect: the fx-provided registry's background health-check goroutine
(`StartHealthChecks`) runs forever, reporting on zero plugins, doing
nothing observable.

**Evidence:**
```
$ grep -rn "RegisterPlugin" --include=*.go .
internal/pkg/plugins/registry.go: func (r *PluginRegistry) RegisterPlugin(...)
internal/pkg/state/state.go:      func RegisterPlugin(...) { ... }
# no call site of PluginRegistry.RegisterPlugin anywhere
$ grep -n "PluginRegistry\|Registry" internal/app/deeper/cli/health.go internal/app/deeper/cli/plugins.go
# no matches — both use state.ActivePlugins directly
```

**Fix — pick one:**
- **A (preferred, less churn):** delete `PluginRegistry` and its fx wiring
  (`providePluginRegistry`, `startupPluginRegistry` in `app.go`) entirely,
  since `state.ActivePlugins` + the existing `health.go`/`plugins.go` logic
  already cover what it was meant to provide.
- **B:** if the health-check/status-tracking behavior is actually wanted,
  wire it up for real: call `registry.RegisterPlugin(traceType, plugin)`
  from each plugin's `Register()` (replacing or alongside
  `state.RegisterPlugin`), and have `health.go`/`plugins.go` read from the
  registry instead of `state.ActivePlugins` directly.

**Verify:** `go build ./...` and `go test ./...` still pass; if option A,
confirm `app.go` no longer references `plugins.PluginRegistry` and
`grep -rn PluginRegistry .` only matches test/plugin-internal code (or
nothing, if removed).

---

## 5. Three plugins are implemented but never registered — dead code [HIGH]

**Problem:** `internal/pkg/plugins/academic_papers`, `.../crtsh`, and
`.../facebook` each implement `DeeperPlugin` correctly with a valid
`init()`/`Register()`, but nothing blank-imports them anywhere in the
module. `cmd/deeper/main.go` only imports `coderepos`, `social_profiles`,
`subdomains`. These three packages compile and pass `go vet` but never run
in the shipped binary.

**Evidence:**
```
$ grep -rn 'plugins/academic_papers\|plugins/crtsh\|plugins/facebook' \
    $(find . -name '*.go')
# zero hits outside each package's own files
```

**Fix — pick one per plugin:**
- If the plugin is finished and intended to ship: add
  `_ "github.com/smirnoffmg/deeper/internal/pkg/plugins/academic_papers"`
  (and the other two) to the import block in `cmd/deeper/main.go`.
- If unfinished/abandoned: delete the package directory rather than leaving
  it to rot.

**Verify:** `deeper plugins list` (or `checkPluginRegistration` in
`health.go`) shows 6 registered plugins instead of 3, if all are wired in.

---

## 6. Lint is red independent of the build issue [MEDIUM]

**Problem:** `golangci-lint run ./...` reports ~20 findings that are real
even after #1 is fixed:
- `errcheck`: unchecked `db.Close()` (`cli/database.go:60,88,109`),
  `rows.Close()` (`database/repository.go:108,306`), `resp.Body.Close()`
  (plugins: `academic_papers/main.go:61`, `facebook_plugin.go:45`,
  `social_profiles/main.go:99`), `rateLimitCmd.MarkFlagRequired`
  (`cli/rate_limit.go:39`), `workerPool.Shutdown`
  (`benchmark.go:63`, `integration_test.go:38,200,265`), `wp.Submit`
  (`integration_test.go:129,148,154`), `os.Setenv`/`os.Unsetenv`
  (`config_test.go:43-54`), `limiter.Wait` (`domain_rate_limiter_test.go:282`).
- `ineffassign`: `workerpool_test.go:237` (`err = wp.Submit(...)` never read).
- `staticcheck`: `cli/health.go:240` (tagged switch suggestion),
  `benchmark.go:229` (`make([]byte, 100, 100)` → `make([]byte, 100)`).
- `unused`: `app.go:27` `logger *zap.Logger` field on `App` is never read.

**Fix:** for each `errcheck` hit, either handle the error (log it) or
explicitly discard with `_ = expr` where genuinely safe (e.g.
`defer func() { _ = db.Close() }()`); fix the ineffectual assignment by
using `err =` result or `_ =`; apply the two staticcheck suggestions;
remove the unused `logger` field from `App` (or start using it).

**Verify:** `golangci-lint run ./...` exits 0.

---

## 7. README is stale relative to the current architecture [MEDIUM]

**Problem:** README's Roadmap lists "CLI framework with subcommands" as a
future item, but it already shipped (`cobra` command tree: `scan`,
`plugins`, `health`, `metrics`, `database`, `version`, `rate-limit`,
`benchmark`). The "Project Structure" section still shows the pre-refactor
flat `internal/{config,display,engine,...}` layout, not the current
`internal/app/deeper/*` + `internal/pkg/*` split.

**Fix:** update the Roadmap checklist and the Project Structure tree in
`README.md` to reflect the actual current layout and shipped features.

**Verify:** manual read-through; no automated check.

---

## 9. `SocialProfilesPlugin` fails to register — Sherlock data source moved and its schema changed [HIGH] — RESOLVED

**Problem:** `internal/pkg/plugins/social_profiles/main.go:119` fetched
`https://raw.githubusercontent.com/sherlock-project/sherlock/master/sherlock/resources/data.json`,
which 404s — upstream renamed the Python package directory from `sherlock/`
to `sherlock_project/` (likely a PyPI naming-conflict fix), moving the file
to `sherlock_project/resources/data.json`. Confirmed live during the
`codescoring.ru` smoke test: `Failed to register plugin SocialProfilesPlugin`
with `invalid character ':' after top-level value` (the literal text
"404: Not Found" isn't valid JSON). Fixing the URL alone surfaced a second,
independent problem: the corrected `data.json` now has a top-level
`"$schema": "data.schema.json"` metadata key, which the naive
`map[string]SherlockEntry` unmarshal chokes on (`json: cannot unmarshal
string into Go value of type struct {...}`) since `$schema`'s value is a
string, not a site-entry object.

**Fix (applied):**
1. Updated the URL to `sherlock_project/resources/data.json`.
2. Extracted a pure `parseSherlockData(data []byte) (map[string]SherlockEntry, error)`
   function that unmarshals into `map[string]json.RawMessage` first, then
   decodes each value into `SherlockEntry` individually, silently skipping
   any key whose value doesn't decode as a site entry (currently just
   `$schema`, robust against future non-entry metadata keys too).
3. Added `main_test.go` (TDD: written failing first) covering the
   `$schema`-skip case and the existing string/list `errorMsg` polymorphism.

**Verify:** `go test ./internal/pkg/plugins/social_profiles/...` passes;
`./build/deeper plugins list` shows `SocialProfilesPlugin` under `username`
with no registration error in the logs (confirmed: 481 entries loaded).

---

## 10. Worker pool result queue has no per-caller correlation — concurrent scans silently cross-attribute results [CRITICAL] — RESOLVED

**Problem:** Found by independently re-deriving DNS data for `codescoring.ru`
(direct `socket.getaddrinfo` lookups + crt.sh) and diffing against what
`deeper` persisted in `trace_edges`. Several subdomains had *someone else's*
IP address recorded against them — e.g. `docs.codescoring.ru`'s real IP is
`185.55.56.154`, but `deeper` recorded `135.181.176.35` (actually
`registry-zero.codescoring.ru`'s IP) and `81.163.23.76` (actually
`files.codescoring.ru`'s IP) against it instead.

Root cause: `Processor.ProcessTrace` (`processor.go`) submits N tasks to the
single shared `Processor.workerPool` and then calls `wp.GetResult(ctx)`
exactly N times, assuming those N results are its own. But
`WorkerPool.GetResult` (`workerpool.go:195`) just pulls the next item off one
pool-wide `resultQueue` channel with **no correlation to which caller
submitted which task** — `TaskResult.TaskID` carries enough information to
correlate, but nothing reads it. This was harmless back when `ProcessTrace`
calls were sequential, but `Engine.processBatch`'s goroutine fan-out (the
"real concurrency" fix earlier in this effort) now calls `ProcessTrace`
concurrently for many different traces against the same `Processor`
instance — so one trace's plugin results get silently stolen by whichever
concurrent `ProcessTrace` call happens to call `GetResult()` next. Confirmed
with a deterministic reproduction: 20 goroutines each calling `ProcessTrace`
for a distinct trace, with a plugin that echoes back its own input — results
came back rotated/shuffled across goroutines every run.

This is a correctness bug in the exact feature this session was building —
the discovery-graph provenance — corrupted specifically under the
concurrency mode that is now the default execution path.

**Fix (applied):**
1. Added `Task.ReplyTo chan *TaskResult` (`workerpool.go`) — when set,
   `Worker.processTask` sends the result there instead of the shared
   `resultQueue`, so each caller gets only its own results. Callers that
   don't set it keep the old shared-queue/`GetResult()` behavior unchanged.
2. `Processor.ProcessTrace` now creates a per-call, buffered `replyTo`
   channel, attaches it to every task it submits, and collects results by
   reading that channel directly instead of calling `wp.GetResult(ctx)`.
3. Added `TestProcessor_ConcurrentProcessTrace_NoCrossAttribution` —
   regression test reproducing the 20-goroutine echo scenario above.

**Verify:** `go test -race ./...` green, including the new regression test
(run repeatedly — the original bug reproduced on effectively every run, not
intermittently); re-ran `./build/deeper scan codescoring.ru` and confirmed
every `DNSResolverPlugin` edge in `trace_edges` now matches independently
verified DNS data exactly.

---

## 8. Untracked files pending commit [LOW — housekeeping, not a defect]

**Problem:** `TESTING.md` and `internal/pkg/database/cache_test.go` are
untracked (`git status`). They look like legitimate in-progress work (a
testing guide and a real test file), unrelated to the `.gitignore` bug in
#1 — they aren't under `internal/app/deeper/`.

**Fix:** `git add TESTING.md internal/pkg/database/cache_test.go` and
commit, once reviewed, so the work isn't lost.

**Verify:** `git status --short` shows no untracked files.
