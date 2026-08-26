---
type: reference
title: Storage (SQLite)
description: "internal/storage: the pure-Go SQLite persistence layer (modernc.org/sqlite, no cgo) — how the DB is opened and PRAGMAs applied, the versioned migration scheme, the DAOs for sessions, messages, parts, permissions and todos, and the wire<->row encoding."
tags: [storage, sqlite, dao, persistence, migrations, schema]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-41612f8ed7b59c998588fda2
    resource: repo://cmd/yolo/deps.go
  - id: openwiki-source-48fd5460e02105c83d622c5c
    resource: repo://internal/storage/db.go
  - id: openwiki-source-5f0b78ce0ce6c354ef04ffd9
    resource: repo://internal/storage/migrate.go
  - id: openwiki-source-d19d4904487260e9b66d89bc
    resource: repo://internal/storage/part_convert.go
  - id: openwiki-source-14f8dd3c4b73c300fc01aa7c
    resource: repo://internal/storage/part_dao.go
  - id: openwiki-source-71a5645d30a5a42bdaafe50f
    resource: repo://internal/storage/permission_dao.go
  - id: openwiki-source-80ace50cce07e29702e5fcfa
    resource: repo://internal/storage/session_dao.go
  - id: openwiki-source-d6ec661780617521ad0e27ab
    resource: repo://internal/storage/todos_dao.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Storage (SQLite)

`internal/storage` is yolo's persistence layer. It wraps `database/sql` over
**`modernc.org/sqlite`** (pure Go, **no cgo**) — the single-writer SQLite store
behind the core server. Every session, message, part, permission request, and
todo list is persisted here; the TUI hydrates a session from it on start.

## The DB wrapper

`DB` wraps `*sql.DB` plus a **cache of prepared statements**: `Exec`/`Query`/
`QueryRow` and their `*Context` variants route through the cache, so repeated
calls reuse the driver's prepared statement instead of re-parsing the SQL
(`db.go`). The mutex is held only across the map update — a blocking `Exec`
never serializes the statement lookup.

`Open(path)` opens (creating if missing) the database at `path`, runs pending
migrations, and returns the `DB` (`db.go`). The DSN sets PRAGMAs **per
connection** — `busy_timeout` and `foreign_keys` are not persisted, so every
connection the pool opens carries them:

```
file:<path>?_foreign_keys=1&_busy_timeout=5000&_journal_mode=WAL
```

The pool is pinned to **one shared connection** (`SetMaxOpenConns(1)`,
`SetMaxIdleConns(1)`): a single-writer SQLite store needs at most one writer,
and that makes the per-connection PRAGMAs total by construction (no pooled
connection can bypass them).

The DB file lives at **`<data>/storage/yolo.db`**, opened through `openDB` in
`cmd/yolo/deps.go`.

## Migration scheme and schema version

Migrations live in `migrate.go` as a **`map[int]string` of schema version →
DDL**, applied in ascending order (`migrate.go`):

- A **`meta`** key/value table is created in `Open` before migrations run, so
  it is not in the map. `schema_version` is stored there.
- `currentSchemaVersion` reads `meta` (`0` when absent).
- Each unapplied version runs **in its own transaction**: execute the DDL, then
  `INSERT OR REPLACE` the new `schema_version`, then commit — rolling back on
  any failure.
- `DB.SchemaVersion()` returns the applied version.

The base schema (version 1) creates four tables, all with `ON DELETE CASCADE`
FKs down the tree `session → message → part`:

| table | key columns | notes |
|---|---|---|
| `session` | `id` PK, `project_dir`, `title`, `model`, `agent`, `cost`, `time_created`, `time_updated` | `cost` kept for parity, **ignored** by `SessionFromRow` |
| `message` | `id` PK, `session_id` FK, `role`, `agent`, `cost` REAL, `tokens` TEXT (JSON), `time_created`, `time_completed` | `cost`/`tokens` are the plan-noted additions |
| `part` | `id` PK, `message_id` FK, `session_id`, `type`, `tool`, `state_json`, `time_created` | `state_json` holds the encoded part body |
| `permission` | `request_id` PK, `session_id`, `action`, `resource`, `response`, `always_json`, `time_created` | one row per permission request |

Version 2 adds the **`todo`** table (`session_id` FK cascade, `content`,
`status`, `priority`, `position`) and an index on `session_id`.

## The DAOs

All DAOs use **parameterized queries only** (`?` placeholders) — no string
interpolation of values — following the golang-database policy.

**Sessions** (`session_dao.go`) — `SessionRow` and: `CreateSession` (an empty
agent takes the column default `"build"` via `agentOrDefault`), `GetSession`
(missing id → `ErrNotFound`), `ListSessions` (newest `time_updated` first,
optional `LIMIT`), `UpdateSession` (zero-valued patch fields left untouched; a
no-op patch returns nil; `RowsAffected == 0` → `ErrNotFound`), and
`DeleteSession` (messages/parts cascade via FK).

**Messages** (`message_dao.go`) — `MessageRow` (whose `Tokens` is the JSON in
`message.tokens`) and: `CreateMessage`, `UpdateMessage`, `DeleteMessage`
(parts cascade), `ListMessages` (earliest first).

**Parts** (`part_dao.go`, `part_convert.go`) — `PartRow` and: `UpsertPart`
(insert-or-update by id), `GetPart`, `ListParts` and `ListToolParts` (earliest
first). The **`part_convert`** functions encode the wire ↔ row body:

- `ProtocolToPart` — **tool** parts store the full `protocol.ToolState` JSON in
  `state_json`; **text/reasoning** parts store a fixed 3-key document
  `{"text":..., "end":n, "synthetic":true}` (`end`/`synthetic` omitted when
  unset). The hot (streamed-delta) path builds that document **by hand** so it
  stays byte-identical to a sorted-key compact marshal. `CallID` is transient
  and **not persisted**. A marshal failure is an **error** — persisting `""`
  would 500 every later read.
- `PartToProtocol` — the inverse: `tool` rows unmarshal `state_json` into a
  `ToolState`; other rows decode the 3-key document back into `Text`/`Time`/
  `IsSynthetic`.

**Session hydration** — `SessionFromRow` assembles the wire `protocol.Session`,
**recomputing `cost` and `tokens` as the sum over assistant messages** (the
`session.cost` column is ignored by design). `ProjectID` is derived
deterministically from the directory: `prj_` + the first 24 hex chars of the
directory's SHA-256 (`db.go`).

**Permissions** (`permission_dao.go`) — `PermissionRow` and: `SavePermission`
(insert-or-update by `request_id`), `ListPermissions` (`pendingOnly` filters to
rows with no `response` yet), `ReplyPermission` (sets `response`; unknown id →
`ErrNotFound`).

**Todos** (`todos_dao.go`) — `SaveTodos` **replaces a session's todo list
wholesale** (delete-then-insert in order, `position = index`; an empty list
clears it), `GetTodos` (stable `position` order), and `AlwaysRules` — which
derives allow-rules from `permission` rows with `response='always'`: one
`protocol.Rule{Permission: action, Pattern, Action: allow}` per pattern in
`always_json`.
