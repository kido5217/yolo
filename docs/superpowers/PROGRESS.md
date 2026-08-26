# Yolo — Verified Facts (session memory)

Task status lives in beads (the release epic; `bd ready`) and in `git log
--oneline`. This file holds proven facts a resumed session must not
re-litigate. The append-only deviation audit log lives in `DEVIATIONS.md`
(items 1–66 frozen in `deviations-archive-v0.1.0.md`).

**Status (2026-08-26):** TUI parity S0 landed on `new_tui` (epic
`yolo-oae`): theme engine (33 embedded upstream themes, the
resolveTheme/generateSystem ports, OSC 11/10/4 detection, custom
discovery + SIGUSR2, the config>KV>default selection chain over the TUI
KV) + the app-shell restyle (logo, borders, home list, footer, session
chrome — teatest SGR goldens under the pinned ANSI256 env); deviations
122–147 logged. The S0.5-review follow-up (bead `yolo-oae.1.12`) fixed
the lingering /dev/tty reader in DetectStd (poll-loop pump joined before
return; deviation 145). Next: the S1 detail pass (Slice Detail Protocol,
plan 2026-08-24-opencode-tui-parity) then S1 execution — first bead is
the glamour dep proposal (approval gate).
Prior release: v0.4.3 (2026-08-24) — allowlisted dependency bump
(PR #20, branch `chore/deps-update`) merged to `main` + tagged `v0.4.3`
+ GitHub release cut: bubbletea v2.0.9, bubbles v2.2.1,
modernc.org/sqlite v1.57.0, teatest v2.0.0-20260823001701 (dev);
no code/wire changes, gate green. Prior release: v0.4.2 (PR #19, branch
`many_words`, beads `yolo-0ca` + `yolo-ukc`): transcript + every
below-viewport surface word-wraps at the terminal width (`wrapLine`;
`SetWidth(w-3)` prompt fix).
Prior release: v0.4.1 (PR #18, branch `code_review`, bead `yolo-lkh`):
corrupt profile configs no longer break `List`/name-based ops, `buildDeps`
pins the loader to the RESOLVED profile id, `FakeFromEnv` follows
`env nil = real env`. v0.4.0 (direction-change docs + config profiles,
deviation 121, PRs #14–#17, tag `v0.4.0`); v0.3.0 (PR #11/#12, tag
`v0.3.0`, epic `yolo-5hy` closed; 0.3.0 backlog frozen in
`docs/superpowers/deferred-archive.md`, `DEFERRED.md` reset).
TUI parity design approved (2026-08-24, spec
`2026-08-24-opencode-tui-parity-design.md`, epic `yolo-oae`): full copy of
opencode's TUI (style, design, colors) — scope TUI-only contract-backed,
strict-copy bar; 33 themes + theme engine, glamour v2.0.1 transcript
rendering, huh v2.0.3 field dialogs + ported select, sahilm/fuzzy v0.1.3,
command palette, which-key + configurable keymap, prompt
history/frecency/autocomplete, home/session completion, parity audit vs
upstream pty captures; 9 slices `yolo-oae.1`–`.9`. Plan done (2026-08-25,
branch `new_tui`): directory `plans/2026-08-24-opencode-tui-parity/` —
`plan.md` (binding 65-bead inventory + Slice Detail Protocol) +
`s0-theme-engine.md` (S0.1–S0.10 full 5-step TDD + slice gate — the active
slice) + 8 slice briefs (S1–S8, each gated on its own detail pass before
execution). Deviations 122–125 pre-drafted in the plan (logged in-commit at
execution time). Execution starts on user go-ahead.
The harness-testing scope remains a future spec.

## Root causes (archive, v0.1.3)

(1) `16d0483` (v0.1.2 datastruct-2) re-wrote the shared
end-marker regex without re-teaching `decodeMarker` — from the 2nd bash command on the
reported exit code was the marker counter and the cwd was never decoded (a latent extra
bug: `pwd`'s trailing newline in the base64 made the respawn `os.Stat` fail → always
respawned in the root dir); (2) the TUI never noticed its SSE stream dropping (silent
reconnect, no re-hydrate) — a lost `session.status` left the footer stuck on `busy`
forever ("hang") with a stale transcript ("nothing printed"); (3) bash output was
row-only until alt+e (upstream shows it inline); (4) truncated bash output reached the
model silently — `tail()` was ported without upstream's full-output save + `Full output
saved to:` marker (plan Task 11 pinned only `tail()`), so a 1036-of-1209-line CI-gate run
arrived mid-stream and the model re-ran the gate ~14×; (5) plan Task 16's LOCKED mapping
RE-APPENDED the newest user message at the tail of every tool-call round, so the model
re-saw its own instruction each round and re-ran tools in a loop even with (4) fixed —
upstream replays history 1:1 (round ends with the tool result). Decisive diff: the same
prompt+model does NOT loop in upstream opencode. Detail: deviations 73–77.

## Last completed

Slice S0 tasks 1–5 (2026-08-25, branch `new_tui`, epic `yolo-oae`, beads
`yolo-oae.1.1`–`.5` closed; every task reviewed clean): theme-engine
foundation — S0.1 embed 33 upstream theme JSONs + `ThemeJson` model
(`c9364ed`); S0.2 `resolveTheme` + 33×2 golden matrix + node oracle
(`6622f7d`); S0.3 `Theme` struct + 51 lipgloss style accessors
(`48bf062`); S0.4 `generateSystem` port — grays/muted/tint/terminalMode
(`9bdaab7`); S0.5 OSC 11/10/4 palette detection + `DetectStd` + x/term
promotion (`7de800a`, fix round `8619252`). Deviations 122–129.
v0.4.0 direction-change docs merged to `main` (2026-08-24, branch
`v0.4.0_spec`, epic `yolo-5u1`): spec `2026-08-24-v0.4.0-design.md`
(beads `yolo-5u1.1`), plan `2026-08-24-v0.4.0-direction-change.md`
(beads `yolo-5u1.3`), restated root `AGENTS.md` (purpose + principles 2–3),
`README.md` (intro + purpose note), internal DOX chain
(`internal/AGENTS.md` + `internal/protocol/AGENTS.md`), this record
(beads `yolo-5u1.2`); deviations 119–120; `TestDescPinned` now pins all
seven tool desc files. Docs-only scope: purpose change to harness
testing, opencode demoted to reference, pins as change gates.
Profile edit (2026-08-24, branch `feat/profile-edit`, beads `yolo-bjp`,
PR #16): `yolo profile edit <id_or_name> [-n name] [-d description]` —
change a profile's display name and/or description after creation.
`config.Edit` (Copy-style single-`yolo.jsonc` rewrite); absent flag keeps,
empty value clears (`-n ""` → name falls back to id; both empty drops the
`profile` element); id and active marker untouched; rename to own name =
no-op, collision with another profile = `ErrNameTaken`.
Profile support (2026-08-24, branch `feat/profiles`, beads `yolo-3pe`):
config profiles under `~/.config/yolo/<profile_id>/` with an active
marker + `yolo profile` CLI + per-run selection (`--profile` flag >
`YOLO_PROFILE` env > marker > `default` recovery). Deviation 121 (hard
deviation/high, no upstream counterpart). Implementation:
`internal/config/profile.go`, `protocol.Config.Profile`, profile-aware
`Loader.Load` / `Server.globalDir()` / `buildDeps`. Gate green
(`go vet` + `go test ./...` + `gofmt`).
0.3.0 Plan 2 (refactor slice) complete (2026-08-24, branch
`v0.3.0-plan-2`): all 16 wave-8 refactors closed as beads
`yolo-5hy.2.1`–`.16` (engine test-harness + engine 4-way split + runRound/
executeTool extracts, pure mapHistory, shared llm sseLoop + anRequest
builders, server contract-suite + handler splits, storage per-entity DAOs,
cmd/yolo deps.go, store per-event Apply, app.go 5-way split, read tool
extracts, Shell execTimeout + markerCmd, dialog payload ownership),
DEFERRED.md dispositions landed, close-out gate green incl. `-race` +
`golangci-lint`. Deviations 116–118 logged (runRound line target vs named
extracts; R16 test-reference rewrite vs "UNMODIFIED" pin; TUI prompt-suite
`-race` flake — brittle contiguous-substring WaitFors hardened to
strip-SGR + independent tokens).
0.3.0 Plan 1 (defect slice) complete (2026-08-24, branch `v0.3.0`): all 39
plan tasks closed as beads `yolo-5hy.1.1`–`.39` (engine lifecycle, storage,
tools, server, TUI, CLI/e2e, naming V1–V8, two hermetic benchmarks),
DEFERRED.md dispositions landed, close-out gate green incl. `-race` +
`golangci-lint`. Deviations 112–115 logged (plan test-code fixes in W/X/AC +
the race-tolerant draft amortization bound). Next: Plan 2 (refactors, 16
tasks, spec §4).
(Prior: v0.2.0 released 2026-08-23 — 16 tasks, deviations 78–88, `just
e2e-live` PASS pre-tag, epic `yolo-8vl` closed; v0.1.3 released — PR #7,
deviations 73–77. Detail in `git log --oneline`.)

## Key verified facts (so they don't get re-litigated)

- Permission engine = port of `packages/opencode/src/permission/index.ts` + matrices in
`agent/agent.ts` (build/plan/yolo verbatim, Task 10).
- Doom loop = sliding 3-identical window; wildcard-deny hides tool iff last matching rule
is `*` deny; `write`+`edit` both map to permission `edit`.
 - Pinned deps (2026-08-24 update, all allowlisted, gate green):
 `charm.land/bubbletea/v2` v2.0.9, `charm.land/lipgloss/v2` v2.0.6,
 `charm.land/bubbles/v2` v2.2.1, `modernc.org/sqlite` v1.57.0 (pure Go, no
 cgo), `tidwall/jsonc` v0.3.3; dev-only `teatest/v2`
 v2.0.0-20260823001701-96af6d2cb5f6.
- Module `github.com/kido5217/yolo`, Go ≥ 1.25 (installed 1.26.7).
- Single deliberate wire deviation: `x-yolo-directory` header.
- Config profiles (2026-08-24, deviation 121, beads `yolo-3pe`): global
config lives at `~/.config/yolo/<profile_id>/` (precedence `config.json`
< `yolo.json` < `yolo.jsonc`); id auto-generated 8-hex (first-run literal
`default`); `~/.config/yolo/active` = active marker; selection =
`--profile` flag > `YOLO_PROFILE` env > marker > `default` recovery;
`yolo profile list|add [name] [-d DESC]|use|edit REF [-n NAME]
[-d DESC]|remove|copy SRC NAME [-d DESC]`;
name unique + id-then-name resolution (dup name = ambiguous error); legacy
flat files ignored; data dir shared.
- Test gating: unit tests never hit network; `YOLO_LLM=fake` (+ `YOLO_FAKE_SCRIPT`) selects
the scripted fake driver; zen fixture gate = 57 models (42 openai + 15 anthropic,
7 google excluded).
- TUI import rule: non-test files under `internal/tui/` import only `internal/protocol` +
`internal/tui/*`; `_test.go` may use `internal/server/testutil` (escape hatch).
- TUI transcript word-wrap (2026-08-24, bead `yolo-0ca`): the bubbles
viewport hard-CLIPS over-width lines and the TUI binds no horizontal
scroll, so pre-wrap the transcript lost everything past the right edge
(unreadable; upstream ink word-wraps). `wrapLine` (`internal/tui/wrap.go`)
word-wraps at the viewport width (word boundaries, over-long tokens
hard-split, CJK/emoji = 2 columns, tab = separator, plain text only);
styled lines wrap before styling (`toolRowLine` returns style + plain);
`WindowSizeMsg` re-wraps via `sess.isDirty`. Tests: `TestWrapLine`,
`TestRenderMessagesWrapsLongLines`, `TestTUILongReplyWraps` (the last word
of a 1000-word single-line fake reply reaches the screen).
- TUI below-viewport surface wrap (2026-08-24, bead `yolo-ukc`): toasts,
the permission overlay, the slash menu, the model/agent dialogs (rows AND
hint lines, via `dimWrapped`), the home session rows and the `!` error line
all wrap at the terminal width (`App.termWidth()`, fallback 80) with the
same `wrapLine`; the session route's viewport height counts the wrapped
help line's real line count. The model dialog's cell hangs at the left-pane
column (`modelRow`); the left pane alone ≥ width degenerates to full-width
cell lines. Footer, divider and the locked quit/help dialogs stay
single-line. Prompt width arithmetic: bubbles v2 textinput `View` =
prompt(2) + `SetWidth` + cursor(1), so `WindowSizeMsg` sets `SetWidth(w-3)`
(pre-fix `w-2` left the prompt line 1 column past the edge). Tests:
`internal/tui/overflow_test.go` (7 wrap tests incl. the composed-frame fit).
- v1 behavior pins: keymap is pgup/pgdn scroll + `\`+enter newline (noted in /help; spec's
↑/↓ viewport scroll replaced); JSONC comments are NOT preserved when a config PATCH
rewrites `yolo.jsonc`.
- lipgloss v2 `Render()` appends a trailing SGR reset AFTER the styled input: trim padded
plain strings (`TrimRight`) BEFORE styling (a styled string's last bytes are `\x1b[m`, so a
post-style trim silently misses), and count display widths in runes
(`utf8.RuneCountInString`) — `·` is 2 bytes, `○` 3 (both 1 column). Both bit T27's two-pane
column math.
- e2e/endpoint facts: `scripts/e2e-live.sh` (entry point `just e2e-live`),
  validated PASS against the REAL `https://ai.kido.ws/v1` on 2026-08-24
  (post Plan 1 merge; the pre-tag re-validation of spec §5) — success shape:
  completed bash tool call + text reply; abort idle → `aborted:false`, busy
  → `aborted:true`; SIGTERM → exit 0. `ai.kido.ws` accepts ANY bearer token
  (private endpoint — key order env → auth.json → config).
  `GET /global/health` → `{"status":"ok"}`; `/session/{id}/message` rows =
  `{"info":{role,error:{type},...},"parts":[...]}` (jq: `.info.role`). Script
  mechanics: `req()` must set globals (never run inside `$(…)` — subshell
  loses `HTTP_STATUS`); boot from the scratch project dir (deviation 65).
- teatest v2 output mechanics (bit T28's suites): (a) each `WaitFor` drains the SHARED
output buffer — consecutive `WaitFor`s observe DISJOINT slices, so a multi-token terminal
state must be ONE merged condition, never two sequential waits (an idle app emits no new
frames for the second); probe `Read`s consume bytes later assertions need; (b) the fake
terminal is not a TTY → lipgloss strips EVERY style; pin `teatest.WithProgramOptions(tea.WithEnvironment([]string{"TTY_FORCE=1","TERM=xterm-256color"}))` for deterministic
ANSI256 SGR (derived from TERM alone, no terminfo; `charmbracelet/colorprofile` stays
indirect — never import directly, "no other deps" rule); (c) v2 `tea.Tick(d, f)` callback
is `func(time.Time) tea.Msg` (v1: `func() tea.Msg`); v2 programs handle `tea.QuitMsg`
internally.
- Shell end-marker wire form (v0.1.3, verified against live markers):
`__YOLO_END_{n}_{exit}_{b64(pwd incl. trailing \n)}` matched by the shared regex
`^__YOLO_END_(\d+)_([^\s]*)$` with `m[1]==n`; `decodeMarker` splits `m[2]` at the first
`_` (std base64 has no `_`) and trims `pwd`'s newline. The emitted marker has never
carried a colon (one stale comment claimed `_{n}_:`, a doc typo since the protocol's
first commit `ae0ff27`); the pre-v0.1.3 `decodeMarker` mis-parsed the GROUP positions,
not a separator. The first command (n=0) masked the group bug because counter==0.
- SSE drop contract (v0.1.3): `client.Events` returns `(events, resync)`; a ping per drop
(buffered, non-blocking); app re-hydrates the current route on `resyncMsg` (the bus has
no replay — gap events are unrecoverable, recovery is REST hydration from storage).
- go-udiff v0.4.1 pinned as the direct line-diff dependency (2026-08-23, 0.3.0 task N,
deviation 104) — the sole new dependency of 0.3.0 (root AGENTS.md allowlist, proposal #1).
- v0.4.0 direction change (2026-08-24, user-approved; spec
  `docs/superpowers/specs/2026-08-24-v0.4.0-design.md`): the original
  Qwen3.8-27B testing goal is complete (local Qwen 3.8 tested, stable,
  optimized). From v0.4.0 the project tests various LLM harnesses and
  frameworks — yolo drives/evaluates other harnesses (the subsystem itself is
  a future architectural spec). opencode v1.18.18 is a reference, not a
  contract: yolo may deviate (wire shapes, behavior, pinned text) on explicit
  user instruction, each deviation logged in `DEVIATIONS.md` with severity.
  The 21 sha256-pinned files (14 `session/prompt/*.txt` + 7 `tool/desc/*.txt`)
  record current intended content, not an upstream lock — an intentional
  change re-baselines the pin in the same commit.

- v0.4.0 post-release code review (2026-08-24, range a2379c1..9c37870, all
  findings fixed on branch code_review): a corrupt sibling profile config
  no longer breaks `List`/name-based `Resolve`/`Add`/`Remove` (id fallback,
  blank metadata); `buildDeps` pins the loader to the RESOLVED profile id
  and `FakeFromEnv` follows the `env nil = real env` convention (a bare
  nil map is not the real env); README documents the ignored pre-v0.4.0
  flat files and the `--profile` flag. User accepted beads-only tracking
   (no dated spec/plan) as sufficient for bounded features such as the
   profiles work (deviation 121); the spec-first workflow rule still
   applies to architectural work.
- Theme engine S0.1–S0.5 (2026-08-25, slice S0, branch `new_tui`):
  `internal/tui/theme` is a strict-copy port of the upstream opencode
  theme engine — the FLOAT operation order is binding (`Tint` blends in
  0–1; grays/muted do 0–255 floor/min/max) or the goldens drift. Golden
  harness: `scripts/tui-theme-golden.mjs` (node oracle) +
  `testdata/{theme,system,terminal-mode}-golden.json`; S0.4 fixed three
  oracle bugs and regenerated (0–1 terminalMode scale — #7f7f7f is
  "dark"; hexToRgb fixture plumbing; NaN collapse upstream does at
  `RGBA.fromInts` construction — `uint8(NaN)=0` on x86_64, e.g.
  `system.black.light` diff line-number bgs `#001200ff`/`#120000ff`,
  NOT `#000000ff`). OSC detection: single-buffer demux stores the
  probe's OSC 4;0 answer as `Palette[0]` first-wins (test-pinned,
  deviation 129 note — indistinguishable on real terminals; only
  `palette[0]` PRESENCE gates system-theme eligibility in S0.7);
  `DetectStd` probes via an owned `/dev/tty` in raw mode ONLY (deviation
  129) — no controlling terminal → `(zero, false)`, no system theme
  (spec §3); timeouts spec-pinned 100/100/100 ms vs upstream 300/300/5 s
  (deviation 128; `PaletteOptions` overridable). Follow-up bead
  `yolo-oae.1.12` (P1, blocked by S0.7): on Linux `close()` does not
  wake a kernel-blocked tty read — the probe pump can linger until the
  next input and discard it; fix (poll-with-timeout pump / inline
  reads) lands with S0.7's wiring.
- Dep promotion (2026-08-25, bead `yolo-oae.1.11`, user-approved
  proposal): `github.com/charmbracelet/x/term` v0.2.2 promoted indirect →
  direct (raw-mode tty for OSC palette detection); ZERO new modules —
  already in the module graph via bubbletea v2; now on the root
  AGENTS.md allowlist.
- Deviation renumbering (slice S0, supersedes the plan's 122–125 map):
  the log now runs 122–129 (122/123 S0.2, 124 S0.3, 125/126 S0.4,
  127/128/129 S0.5); remaining plan entries keep their TEXT with shifted
  numbers: S0.7 config.theme wire → 130, S0.7 single-probe scoping → 131,
  slice-gate SGR quantization → 132 (cross-refs: S0.7 step 5,
  `config_test.go` comment, S0.10 DOX bullet, slice gate steps 4/5/7).
