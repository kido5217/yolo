# golang-code-style — R15b
Date: 2026-08-22 · chunk: R15b · source files: 28
## Findings
- [style-1] P3 prompt.go:41 — `menuItems` is the only function with >3 levels of nested if/for (4: if > for > if > for, alias lookup buried in the negative-match branch) — fix: extract `matchesAlias(c protocol.Command, prefix string) bool` helper (prefix match against `commandAliases[c.Name]`) so the loop body drops to 3 levels — contract-risk: none
- [style-2] P3 client/client.go:61 — `do` takes 5 params (ctx, method, path, in, out), over the skill's ≤4; it is the one choke point every wrapper routes through — fix: group (method, path, in, out) or accept as a wire-helper exception with a comment — contract-risk: none
- [style-3] P3 client/client.go:179 — 131-char line: `c.do(...)` call in `SendMessage` with 4 args + path concat on one line — fix: break the call one-arg-per-line at the string-concat boundary — contract-risk: none
- [style-4] P3 client/client.go:288 — 126-char line: `return c.do(...)` in `ReplyPermission`, same shape as style-3 — fix: break one-arg-per-line — contract-risk: none
- [style-5] P3 session_bench_test.go:22-27 — five lines 125–166 chars: `protocol.Part` composite literals with all fields on one line each (tool entries split awkwardly mid-literal at lines 25-31) — fix: one field per line — contract-risk: none
- [style-6] P3 footer_test.go:34 — 151-char line: `&protocol.Session{ID, Agent, Model, Cost, Tokens}` all six fields on one line in `TestFooterRender` — fix: break one field per line (values are locked golden inputs; break only) — contract-risk: none
- [style-7] P3 home_test.go:75-77 — three lines 131–147 chars: `protocol.Session` literals passed one-per-line to `testApp(...)` in `TestHomeRenderLockedLayout` — fix: break each literal to one field per line — contract-risk: none
- [style-8] P3 model_test.go:43-44 — two lines 131–141 chars: `protocol.Model` map values with ID/ProviderID/Name/Limit on one line each in `tuiProviderFixture` — fix: one field per line — contract-risk: none
- [style-9] P3 model_test.go:228 — 124-char line: `t.Fatalf` with long format string + 2 args in `TestModelDialogKeySequence` — fix: break args onto separate lines — contract-risk: none
- [style-10] P3 agent_test.go:133 — 124-char line: same `t.Fatalf` shape in agent-dialog esc test — fix: break args onto separate lines — contract-risk: none
- [style-11] P3 tui_suite_test.go:42 — 123-char line: `llm.Part{Kind, Name, CallID, Args, Finish}` literal on one line — fix: one field per line — contract-risk: none
- [style-12] P3 app_test.go:90 — 123-char line: same `llm.Part` literal shape in `TestSessionStreamingViewport` — fix: one field per line — contract-risk: none
- [style-13] P3 client/event_test.go:24,28 — two lines 137–138 chars: `fmt.Fprint(w, ` + multi-hundred-char JSON SSE payload + `+"\n\n")` — fix: hoist each payload into a local `const`/`var` above the handler — contract-risk: none
## Deferred / Noted (no fix in 0.1.2)
- All 30 naked-return hits are inside void functions (guard clauses in `Store.Apply`, `move`, `syncSel`, `removeToast`, SSE pump) — skill's naming concern applies only to return values; not findings.
- `var cmds []tea.Cmd` nil slice in app.go:408 `inputUpdate` — bubbletea "nil = no cmd" idiom, never JSON-serialized; judged deliberate.
- Nesting ==3 (at threshold, not flagged): app.go:752 `emit`, model.go:60 `syncModelSel`, model.go:220 `view`, session.go:222 `toolTitleFallback`, session.go:260 `lastToolPartID`, store.go:180 `upsertPart`, store.go:205 `applyDelta` — all read, clear as written.
- what-comment + commented-out-code scans: every hit is an explanatory doc comment (LOCKED/T2x/deviation rationale) or a doc-comment line wrap; no restating comments, no commented-out code.
- app.go/model.go render-path layout math (viewport reservation, alt-screen frame fit) judged deliberate per scope note (deviation 60); not flagged.
## Stats
P0:0 P1:0 P2:0 P3:13
COVERAGE: full — skipped: none
