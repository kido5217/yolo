# golang-naming — tui
Date: 2026-08-22 · chunk: tui · source files: 31 (24 root + 5 client/ + 2 store/, 6311 lines; scope list said 28/5892)
## Findings
- [naming-1] P3 internal/tui/client/client.go:29 — package/type stutter: `client.Client` repeats the package name in the type name (skill: `http.HTTPClient` / `dbpool.DBPool` anti-pattern; anti-stutter applies to ALL exported types) — fix: rename the exported type (or the package) so call sites stop reading "client client" — contract-risk: behavior
- [naming-2] P3 internal/tui/store/store.go:15 — package/type stutter: `store.Store` (same skill rule as naming-1; call sites `store.Store`, `a.store store.Store`) — fix: rename the exported type (or the package) — contract-risk: behavior
- [naming-3] P3 internal/tui/client/client.go:22-24 — sentinel error strings lack the required package prefix (`"not found"`, `"session busy"`, `"bad request"` instead of `"client: not found"` etc.); origin is lost when wrapped (`"not found: <server msg>"`) and the text is what the user sees in the status line — fix: prefix all three with `"client: "` — contract-risk: render
- [naming-4] P3 internal/tui/app.go:552 — boolean method `dialogStack.has()` is an incomplete question ("has" what?); Go idiom is the negated `empty()` or `hasItems()` — fix: rename to `empty()` and invert callers (or `hasItems()`) — contract-risk: none
- [naming-5] P3 internal/tui/app.go:526 — enum zero value not protected: `dlgQuit dialogKind = iota` makes the zero value a real, user-facing dialog, so a zero-initialized `dialog{}` silently becomes the quit dialog (skill: "not optional") — fix: `dlgUnknown dialogKind = iota` sentinel at 0 (or start at `iota + 1`) — contract-risk: none
- [naming-6] P3 internal/tui/app.go:26 — `EventMsg.Ev` uses a non-standard abbreviation; the same concept is `Event`/`ev`/`eventCh` everywhere else in the package — fix: field `Ev` → `Event` — contract-risk: none
- [naming-7] P3 internal/tui/app.go:209 — `hydratedMsg.id` is a session id but bare `id` is ambiguous next to sibling `sess *protocol.Session`; the codebase consistently names this concept `sessionID`/`SessionID` — fix: `id` → `sessID` — contract-risk: none
- [naming-8] P3 internal/tui/client/client.go:30 — field `Base string` holds the server URL but the name gives no type hint (skill: name describes the value, not the role alone) — fix: `Base` → `BaseURL` — contract-risk: none
- [naming-9] P3 internal/tui/store/store.go:25 — exported bool field `Conn` reads like a connection object, not a live-state flag (skill's exact bad example: `connected // could be confused with a connection object`) — fix: `Conn` → `Live` (or `SSELive`) — contract-risk: none
- [naming-10] P3 internal/tui/session.go:22-23 — bool fields `follow` and `dirty` lack the `is`/`has`-style question form the skill requires for boolean struct fields — fix: `follow` → `following` (or `isFollowing`); `dirty` → `isDirty` — contract-risk: none
- [naming-11] P3 internal/tui/model.go:32 — bool field `modelDlg.subChoice` is a bare noun, not a true/false question — fix: `subChoice` → `hasSubChoice` (or `subChoiceOpen`) — contract-risk: none
- [naming-12] P3 internal/tui/agent.go:17 — same bare-noun bool field `agentDlg.subChoice` as naming-11 (two dialog types share the pattern) — fix: rename like naming-11 — contract-risk: none
- [naming-13] P3 internal/tui/model_test.go:31 — `tuiProviderFixture` / `tuiAgentFixture` (line 52) repeat the package name `tui` at every in-package call site (anti-stutter); no collision with `providerFixture`/`agentFixture` in package tui — fix: drop the `tui` prefix — contract-risk: none
- [naming-14] P3 internal/tui/session_test.go:44 — `sessDivider()` re-derives a hardcoded 28-rune divider, duplicating production `dividerLine()` (internal/tui/style.go:22, const `dividerWidth`); same concept, two names, magic number that can drift from the locked layout — fix: delete the helper and call `dividerLine()` — contract-risk: none
- [naming-15] P3 internal/tui/app.go:536-552 — `dialogStack` methods use receiver `s`, which is not a 1-2 letter abbreviation of the type (skill receiver rule) and collides with `s *Store` in the same package — fix: receiver `s` → `d` on view/top/push/pop/has — contract-risk: none
- [naming-16] P3 internal/tui/app.go:45 — `App.cur string` is cryptic: it holds the current session id, the concept the codebase names `sessionID`/`SessionID` elsewhere; readers cannot tell what `cur` is from the field alone — fix: `cur` → `curSessionID` (all uses in package tui) — contract-risk: none
## Deferred / Noted (no fix in 0.1.2)
- [naming-1..3] auto-deferred by contract-risk tag (behavior/behavior/render)
- `route` (app.go:35) and `modelPane` (model.go:20) zero values are deliberate initial states (app starts home; dialog opens on providers pane) — different from dlgQuit; no change
- `upsertSession(se protocol.Session)` (store.go:160): `se` is a non-standard abbreviation for a session; `sess` is the codebase norm
- `stripANSITest` (app_test.go:31): test helper wearing a `Test` suffix reads like a test function
- `short6` (permission.go:42): named by value (6 runes) rather than role
- `sessionBusy(st *store.Store) bool` (session.go:51): bool helper in noun-phrase form; `isSessionBusy` would read as a question
- `catalogMsg.provs` (app.go:638): "provs" abbreviation of providers
- `permReplyMsg.id` (permission.go:77): permission request id named `id` while the client method calls the same parameter `requestID`
- `pane*`/`dlg*` enum stems vs type names `modelPane`/`dialogKind`: skill prefers type-name prefix, but literal `modelProviders`/`dialogUnknown` stems are worse; accepted
- `PathEscapeID` (client.go:123): exported but used only inside package client — over-export (design, not naming)
- `waitPending` (permission_test.go:232): vague helper name (waits for pending permission asks)
- `EventMsg`/`HydrateMsg` exported solely as test seams while sibling msgs are unexported — visibility choice, documented in comments; not a naming finding
- no snake_case identifiers, no ALL_CAPS constants, no mixed-case acronyms (Id/Url/Json/Sse) found in scope; subtest names are lowercase descriptive
## Stats
P0:0 P1:0 P2:0 P3:16
COVERAGE: full — skipped: none
