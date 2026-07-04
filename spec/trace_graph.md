# Spec: persist the trace discovery graph in SQLite

Status: planned, not yet implemented. Companion to `spec/troubles.md` (bug list) and `spec/contact_crawler.md` (new plugin spec) — this is a new-feature spec for the persistence layer.

## Context

Today the tool's `traces`/`scan_sessions` DB tables are entirely dead code in production — `StoreTrace`/`CreateScanSession`/`UpdateScanSession` are called only from their own unit tests; nothing in `engine.go`/`processor.go`/`cli/scan.go` ever persists a scan. The only live DB usage is the plugin-result memoization cache (`cache_entries`), unrelated to graph structure. Even if `StoreTrace` were wired up, its `UNIQUE(value, type)` + `INSERT OR IGNORE` design would structurally prevent recording a trace being reached via more than one discovery path — the core thing this feature needs. And the parent→plugin→child link itself is discarded in memory today: `Processor.ProcessTrace` takes one trace, dispatches to N plugins, and returns a flat `[]entities.Trace` with the per-plugin/per-parent association already gone; `Engine.processBatch`/`ProcessInput` flatten further. So this is "build discovery-graph persistence from scratch," including a real data-flow change through the processing pipeline, not "add a table to something already wired up."

## Decisions

- **Stay in SQLite**, add a `trace_edges` table, query via `WITH RECURSIVE` CTEs — rejected an embedded graph DB (Kuzu) because its Go API is CGO-based, and this tool's actual graph scale (dozens–hundreds of nodes per scan) doesn't need that machinery.
- **A related finding, tracked separately in `spec/troubles.md` #2**: the current SQLite driver (`mattn/go-sqlite3`) is CGO-based, and `.goreleaser.yml` built all released binaries with `CGO_ENABLED=0` — verified by direct reproduction that this made every officially released `deeper` binary unable to open the database at all. **Resolved by removing `.goreleaser.yml`/`.github/workflows/release.yml` entirely** (decided separately from this spec) rather than swapping the driver — the project is dropping automated cross-platform release binaries rather than taking on CGO cross-compilation toolchain complexity, so `mattn/go-sqlite3` stays as-is; building from source with plain `go build` works fine since CGO is enabled by default. No driver change needed here as a result — this spec's schema/queries are unaffected either way (same SQLite dialect).

## Schema design

In `internal/pkg/database/database.go`'s `migrate()`:

```sql
CREATE TABLE IF NOT EXISTS traces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    value TEXT NOT NULL,
    type TEXT NOT NULL,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    metadata TEXT,
    UNIQUE(value, type)
);

CREATE TABLE IF NOT EXISTS trace_edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_trace_id INTEGER,               -- NULL only for the scan's root/seed trace
    child_trace_id  INTEGER NOT NULL,
    plugin_name     TEXT NOT NULL,          -- '__seed__' sentinel for the root edge
    scan_id         INTEGER NOT NULL,
    discovered_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_trace_id) REFERENCES traces(id),
    FOREIGN KEY (child_trace_id)  REFERENCES traces(id),
    FOREIGN KEY (scan_id)         REFERENCES scan_sessions(id),
    UNIQUE(parent_trace_id, child_trace_id, plugin_name, scan_id)
);

CREATE INDEX IF NOT EXISTS idx_trace_edges_child  ON trace_edges(child_trace_id, scan_id);
CREATE INDEX IF NOT EXISTS idx_trace_edges_parent ON trace_edges(parent_trace_id, scan_id);
CREATE INDEX IF NOT EXISTS idx_trace_edges_scan   ON trace_edges(scan_id);
```

Add `PRAGMA foreign_keys = ON` right after `sql.Open` in `NewDatabase` — SQLite ignores declared `FOREIGN KEY` clauses unless this is set per-connection, and today it isn't set at all (the existing `scan_id` FK-shaped column on `traces` was already unenforced).

**Drop `source_plugin`, `scan_id`, `depth` from `traces`.** They're structurally singular (one column, one value) but a trace node can now have N parents/plugins/scans — keeping them would store misleading "first writer wins" data that contradicts `trace_edges` as the source of truth. `depth` is inherently path-relative (different depth via different parents/scans), not a node property — it becomes a computed property of a path via the recursive CTE, not stored data. No real data depends on the current columns (confirmed dead code path). `models.Trace` loses these 3 fields; `FromEntity` drops the corresponding params; `StoreTrace`/`GetTraces`/`GetTraceByValue`/`TraceQuery` shrink accordingly.

**Uniqueness constraint reasoning**: `UNIQUE(parent_trace_id, child_trace_id, plugin_name, scan_id)` — including `plugin_name` because two different plugins independently deriving the same child from the same parent is two distinct provenance events worth keeping (the whole point of this feature); including `scan_id` because re-confirming an edge in a later scan is a distinct historical event. Same plugin/parent/child/scan (e.g. a worker-pool retry) collapses via `INSERT OR IGNORE`, matching the existing convention.

## New file: `internal/pkg/database/graph.go` (+ `graph_test.go`)

Kept separate from `repository.go` (already ~400 lines) the same way `cache.go` already is.

- `entities.Discovery{Parent, PluginName, Child Trace}` — new type in `internal/pkg/entities/entities.go` (the only shared leaf both `processor` and `database` can use without an import cycle; `database` cannot import `processor`, which itself imports `database`). Placement note: this is a processor/engine-domain concept sitting in the trace-vocabulary package out of necessity (avoiding an import cycle), not because it's a natural fit — functionally identical to introducing a new `internal/pkg/discovery` package, just less churn.
- `Repository.GetOrCreateTrace(trace entities.Trace) (int64, error)` — resolves/creates a node by (value,type). **Correctness note**: after an `INSERT OR IGNORE` no-op, `LastInsertId()` keeps returning the *previous successful* insert's rowid (a false positive) — must gate on `RowsAffected() > 0`, not on the returned ID being non-zero, then fall back to a `SELECT id WHERE value=? AND type=?` when the insert was ignored. (The existing `Repository.StoreTrace` has this same latent flaw today — never manifested since `trace.ID` is never read anywhere — worth being aware of, not necessarily fixing in this pass.)
- `Repository.InsertEdge(edge *TraceEdge) error` — idempotent single-edge insert (`INSERT OR IGNORE`).
- `Repository.PersistDiscoveries(scanID int64, discoveries []entities.Discovery) error` — resolves every trace in a batch to an ID (creating as needed, caching within the call) and inserts all edges, in one transaction.
- `Repository.GetDiscoveryPath(scanID, targetTraceID int64) ([]TraceEdge, error)` — `WITH RECURSIVE` walking `parent_trace_id` backward to the root, returns root-first.
- `Repository.GetReachableTraces(scanID, startTraceID int64, maxHops int) ([]ReachableTrace, error)` — `WITH RECURSIVE` walking `child_trace_id` forward, hop-bounded, each trace paired with its minimum hop-distance.

**Critical technical correction**: the "use `UNION` not `UNION ALL` to prevent infinite loops on cycles" idiom only works for the *naive* recursive CTE with no per-row accumulator. Both queries above need a `hop` counter (to bound/order results) and a `visited` path-accumulator column with a `NOT LIKE '%,'||id||',%'` guard — once a hop counter is added, every recursive row is distinct by construction, so plain `UNION`'s row-dedup does nothing to stop a real cycle (e.g. Domain → Subdomain → DNS PTR back to the same Domain); it would recurse forever. Use the explicit visited-guard + `UNION ALL` idiom instead. This is exactly why `TestRepository_GetReachableTraces_HandlesCycle` (test plan below) exists — it would hang/OOM without this.

Example shape (root-to-target path query):
```sql
WITH RECURSIVE path(child_trace_id, parent_trace_id, plugin_name, hop, visited) AS (
    SELECT child_trace_id, parent_trace_id, plugin_name, 0,
           ',' || child_trace_id || ','
    FROM trace_edges
    WHERE scan_id = ? AND child_trace_id = ?

    UNION ALL

    SELECT e.child_trace_id, e.parent_trace_id, e.plugin_name, p.hop + 1,
           p.visited || e.parent_trace_id || ','
    FROM trace_edges e
    JOIN path p ON e.child_trace_id = p.parent_trace_id
    WHERE e.scan_id = ?
      AND p.visited NOT LIKE '%,' || e.parent_trace_id || ',%'
)
SELECT child_trace_id, parent_trace_id, plugin_name, hop
FROM path
ORDER BY hop DESC;
```

## Data-flow change: `ProcessTrace` → `processBatch` → `ProcessInput`

Only the *element type* flowing through changes — the concurrency shape (worker-pool `Submit`/`GetResult` inside `ProcessTrace`; goroutines+semaphore+mutex in `processBatch`) stays exactly as it is, so the recently-fixed real concurrency isn't touched:

- `newTraceTaskHandler`'s handler returns a small `pluginTraceResult{PluginName string; Traces []entities.Trace}` instead of a bare slice, since `ProcessTrace`'s result-collection loop reads results in *completion* order (not submission order) and can't otherwise recover which plugin produced which children.
- `Processor.ProcessTrace(ctx, trace) ([]entities.Discovery, error)` — builds `entities.Discovery{Parent: trace, PluginName, Child}` per child in the collection loop.
- `Engine.processBatch(ctx, traces) ([]entities.Discovery, error)` — same goroutines/semaphore, `allResults []entities.Discovery` instead of `[]entities.Trace`.
- `Engine` gets a `repo *database.Repository` field (constructor already receives `repo`, currently drops it after forwarding to `processor.NewProcessor` — just also assign it).
- `Engine.ProcessInput(ctx, input string, scanID int64) ([]entities.Trace, error)` — persists the root trace + root edge (`PluginName: database.SeedPluginName`, nil parent) before the BFS loop; inside the loop, calls `repo.PersistDiscoveries(scanID, discoveries)` on every batch's discoveries.

**The one place this whole feature would silently break if implemented by pattern-matching the existing code**: `ProcessInput`'s existing `seen map[entities.Trace]bool` gate controls whether a node gets re-enqueued for BFS traversal — it must **not** also gate whether an edge gets persisted, or `PersistDiscoveries` only ever sees a node's *first* parent, which defeats the entire multi-parent point of this feature while every basic "is there an edges table" test still passes. Persist every discovery unconditionally (subject only to `InsertEdge`'s own idempotent `INSERT OR IGNORE`); use `seen` only to decide whether to push the child onto `stack`/`allTraces`.

## Wiring into `cli/scan.go` / `cli/root.go`

`createEngine()` currently builds its own private DB/Repository/Cache and returns only `*engine.Engine` — `scan.go` has no repo handle to create a scan session with, and today silently returns/consumes a bare `nil` engine on setup failure (a latent nil-pointer-panic risk, naturally fixed by this signature change). New shape:

```go
// cli/root.go
func createEngine() (*engine.Engine, *database.Repository, error)
```

```go
// cli/scan.go RunE
eng, repo, err := createEngine()
// ... error handling ...
session, err := repo.CreateScanSession(input)
// ...
traces, procErr := eng.ProcessInput(ctx, input, session.ID)
// ... update session.Status/CompletedAt/UniqueTraces, repo.UpdateScanSession(session) ...
```

Note: `ProcessInput` currently computes `processedCount`/`errorCount` internally but discards them, so `scan_sessions.total_traces`/`.errors` can only be populated as a proxy (`len(traces)`) unless `ProcessInput`'s return type is also expanded to a small result struct — an optional nice-to-have, not required for the core feature.

`app.go`'s Fx-provided engine/repo and `saveResults`'s stub implementation are pre-existing, unrelated dead code/gaps — out of scope here.

## Test plan (TDD order)

1. `database_test.go`: update `TestRepository_StoreAndGetTrace` for the narrowed `traces` schema (this test must go red first, from the schema change, before any new code).
2. `graph_test.go` (new): `GetOrCreateTrace` dedup; `InsertEdge` idempotency; **`PersistDiscoveries_MultiParent`** — two discoveries with different parents but the same child+plugin+scan must produce 2 edge rows, not 1 overwritten row (this is the test that would catch the `seen`-gates-persistence mistake); `GetDiscoveryPath` root-to-target and multi-parent cases; `GetReachableTraces` hop-bounding and **cycle handling** (A→B→A edges, assert termination + correct minimum hops).
3. `processor_test.go`: update `TestProcessor_ProcessTrace_Concurrency` for `[]entities.Discovery`; add a test asserting each `Discovery.PluginName` pairs with the correct child when multiple plugins return distinct children.
4. `engine_test.go` (new — none exist today): fake-plugin chain (mirroring `processor_test.go`'s `slowPlugin`) proving an end-to-end multi-hop scan produces the correct edge chain in a real temp-file DB, verified via `GetDiscoveryPath`/`GetReachableTraces`, plus a synthetic multi-parent case proving the `seen`-gate design point holds through the *full* pipeline, not just at the repository layer.
5. `cli/scan_test.go` (new, lighter smoke test): `createEngine()`'s new signature used correctly; scan session transitions `running` → `completed`/`failed`.

## Verification

1. `go build ./...`, `go vet ./...` clean.
2. `go test -race ./...` green, including all new/updated tests above.
3. Manual run: `./build/deeper scan codescoring.ru`, then inspect `~/.deeper/deeper.db` directly (`sqlite3` CLI or a quick throwaway query) to confirm `trace_edges` contains the expected Domain→Subdomain→IpAddr chain from the established baseline case, and that `GetDiscoveryPath`/`GetReachableTraces` return sane results against it.

## Critical files

- `internal/pkg/database/database.go` — schema/migrate/PRAGMA
- `internal/pkg/database/graph.go` (new) — all graph persistence + recursive queries
- `internal/pkg/database/models.go` — narrowed `Trace` model
- `internal/pkg/entities/entities.go` — new `Discovery` type
- `internal/app/deeper/processor/processor.go` — `ProcessTrace` return type change
- `internal/app/deeper/engine/engine.go` — `processBatch`/`ProcessInput` changes, the seen-vs-persist crux
- `internal/app/deeper/cli/scan.go`, `internal/app/deeper/cli/root.go` — scan-session wiring

## Explicitly out of scope (tracked separately)

- Removing `.goreleaser.yml`/`.github/workflows/release.yml` (fixes the confirmed total DB outage in `CGO_ENABLED=0` released binaries — see `spec/troubles.md` #2) — a separate, already-decided change, not folded into this one.
- `app.go`'s dead Fx-provided engine/repo path, `saveResults`'s unimplemented stub.
