---
type: concept
title: Config and Profiles
description: "internal/config: XDG dir resolution, JSONC config discovery and deterministic deep merge, {env:NAME} substitution, and the profile lifecycle (selection precedence, first-run default, list/add/use/remove/copy) with corruption tolerance."
tags: [config, profiles, xdg, jsonc, env-substitution, resolution]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-a8910515ddd14810ad43f5c1
    resource: repo://internal/config/config.go
  - id: openwiki-source-e9096526cb7c8e8729cb3c9b
    resource: repo://internal/config/profile.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Config and Profiles

`internal/config` implements `yolo.jsonc` / `yolo.json` discovery, JSONC
parsing, deterministic deep merge, and `{env:NAME}` substitution, plus the
profile lifecycle. A profile is a yolo extension (no upstream opencode
counterpart): a directory `<root>/<id>/` holding the usual global config files,
with the active profile id recorded in the one-line `<root>/active` marker
(internal/config/profile.go:17-25).

## XDG dir resolution

The three XDG roots resolve env-first, home-fallback (internal/config/config.go:
17-75):

- `Home()` — `XDG_CONFIG_HOME` or `~/.config`
- `Data()` — `XDG_DATA_HOME` or `~/.local/share`
- `Cache()` — `XDG_CACHE_HOME` or `~/.cache`

A **broken home** (no XDG override, `$HOME` unset and no passwd entry) is an
**error**, so config never silently degrades to CWD-relative paths. The yolo
dirs are the roots with `yolo` appended: `GlobalYoloDir() = <home>/yolo`,
`DataYoloDir() = <data>/yolo`, `CacheYoloDir() = <cache>/yolo`. The `Dirs` struct
carries the three roots plus the process-selected profile id (pinned at startup;
empty only in the testutil harness, which resolves the active marker per
request).

## Config discovery and merge

Global file precedence is `config.json` → `yolo.json` → `yolo.jsonc`, later
wins; missing files are skipped (internal/config/config.go:132-157). `LoadAt`
deterministically merges, in this order (internal/config/config.go:211-266):

1. the global layer (`config.json` → `yolo.json` → `yolo.jsonc` in `globalDir`);
2. the **project chain** — `startDir` walked up to the filesystem root,
   processed **outermost first, innermost last** (`yolo.json` then
   `yolo.jsonc` per directory);
3. `<startDir>/.yolo` (`yolo.json` then `yolo.jsonc`) as the innermost override
   of the project directory;
4. `{env:NAME}` substitution over the merged tree.

`Merge` is a deterministic deep merge (internal/config/config.go:297-352): maps
recurse, the special `instructions` key **concatenates** (deduplicated, first
occurrence wins), and every other array/leaf is **src-wins**. `Substitute`
(internal/config/config.go:354-387) replaces `{env:NAME}` occurrences and also
treats a whole-string env name as a substitution (an unset var leaves the
string). JSONC is parsed via `tidwall/jsonc` (`UnmarshalJSONC`); a parse error
is wrapped with the offending file path since `LoadAt` merges many candidate
files.

`SaveGlobal` merges a config over the existing global config and rewrites
`<globalDir>/yolo.jsonc` (the highest-precedence global file). **JSONC comments
are not preserved** (parse → merge → `MarshalIndent` 2-space) — a flagged
deviation; the TUI never relies on comments
(internal/config/config.go:159-185).

## Profile selection precedence

`ProcessProfile(root, flag, env)` resolves the profile id for a process
(internal/config/profile.go:45-64). The precedence is **locked**:

1. the **flag reference** (an id or a name) — `yolo --profile X`;
2. the **`YOLO_PROFILE` env** value (below the flag, above the marker);
3. the **active marker** — `EnsureActive` creates the default profile on a first
   run.

`Resolve` maps a reference to an id: an **exact id (directory name) match wins**;
otherwise an **exact effective-name match** — a name shared by several profiles
is `ErrAmbiguous`, none is `ErrNotFound`. A profile whose config fails to load is
skipped in *name* matching, but its id still matches exactly, so a corrupt
sibling never blocks a healthy one (internal/config/profile.go:169-211).

### First-run and recovery

`EnsureActive` (internal/config/profile.go:105-125) guarantees the root holds a
usable active profile: a **fresh root gets a `default` profile**, and a missing
or stale marker **recovers to `default`** (created if absent, never clobbered).
`ActiveID` reads the marker; a missing marker (or missing root) is the empty
string, not an error. `SetActive` records the id only after verifying the
profile directory exists.

### Ids

`GenerateID` returns 8 lowercase hex chars from `crypto/rand`
(Kubernetes-style short random suffix); `uniqueID` retries up to 10 times before
failing (internal/config/profile.go:66-74, 424-437).

## Listing and corruption tolerance

`List` returns every profile directory under the root sorted by **effective name
(then id)**, with metadata (name, description) from each profile's merged global
config. A missing root is an empty list, and the `active` marker is not a
profile. **A profile whose config fails to load still lists — with its id as the
name and a blank description — so one corrupt profile can never break the
listing** (or the name checks built on it) (internal/config/profile.go:127-167;
deviation 121).

## Lifecycle operations

- **`Add(root, name, description)`** — creates a profile with an auto-generated
  id; `name`/`description` are stored in the new profile's `yolo.jsonc`
  `profile` element; a duplicate effective name is `ErrNameTaken`
  (internal/config/profile.go:213-251).
- **`Remove(root, id)`** — deletes a profile; when it was the active one, the
  marker falls back to the first remaining profile by name, or to a recreated
  `default` when none remain (internal/config/profile.go:253-281).
- **`Copy(root, srcID, name, description)`** — copies the source to a new
  profile with an auto-generated id; `name` is required and must not duplicate
  an existing effective name (including the source's); `description` overrides
  or carries over; the destination config is the source's merged config rewritten
  as a single `yolo.jsonc` (same comment loss as `SaveGlobal`)
  (internal/config/profile.go:283-327).
- **`Edit(root, id, name, description, hasName, hasDesc)`** — changes the display
  name and/or description; `hasName`/`hasDesc` report which flag was given
  (absent ≠ empty: an empty value clears the field); renaming to one's own
  current effective name is a successful no-op, a name matching another profile
  is `ErrNameTaken`; when both fields end up empty the `profile` element is
  dropped; the id (directory name) and the active marker are never touched
  (internal/config/profile.go:329-379).

## Representative tests

Config and profile behavior are covered by the `internal/config` test files
(merge determinism, `instructions` concatenation, `{env:}` substitution,
selection precedence, and the corruption-tolerance cases in `List`/`Resolve`).
