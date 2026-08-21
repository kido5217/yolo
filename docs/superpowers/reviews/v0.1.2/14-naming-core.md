# golang-naming — core
Date: 2026-08-22 · chunk: core · source files: 30

## Findings
- [naming-1] P3 internal/storage/dao.go:60 — `nullStrPtr(p *int64)` names a type it does not take: "Str" in the name but it dereferences `*int64` (its own doc: "nullable integer columns"); the adjacent `nullStr` reinforces a wrong mental model. — fix: rename to `nullPtr` (or `nullIntPtr`) — contract-risk: none
- [naming-2] P2 internal/permission/service.go:263 (also :240 `cascade`) — adjacent string params `stored, wire` are cryptic; at call sites like `s.resolve(req.RequestID, Deny, "aborted", "reject", false)` a reader cannot tell which literal is the DB-persisted response and which is the `permission.replied` wire reply — fix: rename to `dbResponse, wireReply` (params double as documentation per the skill) — contract-risk: none
- [naming-3] P3 internal/permission/permission.go:16 — inconsistent enum trio `Allow`, `Deny`, `AskAction`: only the third member carries an `Action` suffix, and it is avoidable (`Ask` collides with nothing in package scope; `(*Service).Ask` is a method, not a package identifier). — fix: rename `AskAction` → `Ask` — contract-risk: none
- [naming-4] P3 internal/permission/service.go:36 — `Request.DecisionPre Decision` is an awkward compound (adjective belongs before the noun); doc comment also has a stray space ("Allow|Deny | AskAction"). — fix: rename to `PreDecision` — contract-risk: none
- [naming-5] P3 internal/config/config.go:106 — `yoloFilesC = "yolo.jsonc"` misreads as a C-language file (vague trailing "C"); symmetric name is `yoloFilesJSONC`. Related: all three constants (`globalFiles`, `yoloFilesJSON`, `yoloFilesC`) are plurals holding single filenames. — fix: `globalFile`, `yoloFileJSON`, `yoloFileJSONC` — contract-risk: none
- [naming-6] P3 internal/storage/storage_test.go:349,354,359,364 — subtest names `t.Run("GetPart")`, `t.Run("UpdateSession")`, `t.Run("UpdateMessage")`, `t.Run("ReplyPermission")` are capitalized identifier-style; the skill requires fully lowercase descriptive phrases, and the four sibling subtests in the same function (`"message time_completed"`, `"part tool"`, `"permission response"`) already follow it. — fix: e.g. `"missing part"`, `"missing session"`, `"missing message"`, `"missing permission request"` — contract-risk: none
- [naming-7] P3 internal/protocol/protocol_test.go:9 — import alias `p` on the package under test has no collision to justify it (`protocol` is free); `protocol.NewID("ses")` is the readable form. — fix: drop the alias — contract-risk: none
- [naming-8] P3 internal/storage/dao_bench_test.go:17 — `benchText(want int)`: `want` is Go-testing's expected-value term, repurposed here as a byte-size argument; readers will expect the result to be compared against it. — fix: rename to `n` or `size` — contract-risk: none
- [naming-9] P3 internal/protocol/provider.go:5-7,20-22 (also agent.go:10, part.go:30-31, event.go:103) — wire-DTO bool fields lack the is/has predicate form (`ProviderAuth.KeyRequired`, `Model.ToolCall`, `Model.Reasoning`, `Model.Attachment`, `Agent.Hidden`, `Part.Synthetic`, `Part.Ignored`, `PermissionRepliedProps.Auto`); any rename (e.g. `IsKeyRequired`) changes JSON tags. — fix: only in a deliberate wire-shape revision — contract-risk: wire
- [naming-10] P3 internal/protocol/session.go:28-31 — `StatusIdle`/`StatusBusy`/`StatusRetry` are not prefixed with their owning type name `SessionStatus`; `protocol.StatusBusy` reads ambiguously beside `Todo.Status`, `ToolState.Status`, `SessionStatus.Type`. — fix: `SessionStatusIdle` et al. — contract-risk: behavior

## Deferred / Noted (no fix in 0.1.2)
- `var base = []protocol.Rule` (permission/builtins.go:17): package-level `base` could be `baseRules`; doc comment makes it clear — cosmetic, not flagged.
- `LoadBuiltins` (strict, errors) vs `BuiltinsFor` (lenient, build-fallback) in permission/builtins.go: similar names, genuinely different semantics, documented — intentional.
- `type Decision = Action` alias (permission/permission.go:20): two names for one concept, deliberate and documented.
- `segmentRe(seg string)` (glob/glob.go:72): the param holds a pattern segment (callers pass `pat`); `seg` is ambiguous — sub-finding level.
- `CommandResponse.Handled string` (protocol/command.go:6): adverb-named string holding a route value (`"client"`); faithful upstream wire mirror (`handled`) — untouchable, not flagged.
- `config.Home()/Data()/Cache()` (config/config.go:20-51): `config.Home()` reads as "config home" via the anti-stutter rule — not a finding.
- [naming-10] `StatusIdle|Busy|Retry` touch 12 files (>5) → auto-deferred per chunk rule; Go constant names carry no wire reflection, only churn risk.

## Stats
P0:0 P1:0 P2:1 P3:9
COVERAGE: full — skipped: none
