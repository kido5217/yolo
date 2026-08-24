# Yolo — Verified Facts (session memory)

Task status lives in beads (the release epic; `bd ready`) and in `git log
--oneline`. This file holds proven facts a resumed session must not
re-litigate. The append-only deviation audit log lives in `DEVIATIONS.md`
(items 1–66 frozen in `deviations-archive-v0.1.0.md`).

**Status (2026-08-24):** v0.4.0 released — direction-change docs (epic
`yolo-5u1`, branch `v0.4.0_spec`) merged to `main` + config profiles
(deviation 121, PR #15) + `yolo profile edit` (PR #16) + tagged `v0.4.0`
+ GitHub release cut. Purpose change (docs-only; spec
`docs/superpowers/specs/2026-08-24-v0.4.0-design.md`, approved
2026-08-24): the Qwen3.8-27B testing goal is complete (local Qwen 3.8
tested, stable, optimized); from v0.4.0 the project tests various LLM
harnesses and frameworks; opencode v1.18.18 is a reference, not a
contract (deviations on explicit user instruction, logged in
`DEVIATIONS.md`); the 21 sha256-pinned text files are change gates, not
upstream locks. Prior release: v0.3.0 (PRs #11/#12, tag `v0.3.0`,
`just e2e-live` PASS 2026-08-24, epic `yolo-5hy` closed; 0.3.0 backlog
frozen in `docs/superpowers/deferred-archive.md`, `DEFERRED.md` reset).
Next: the harness-testing scope is a future spec.

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
- Pinned deps: `charm.land/bubbletea/v2` v2.0.8, `charm.land/lipgloss/v2` v2.0.6,
`charm.land/bubbles/v2` v2.1.1, `modernc.org/sqlite` v1.56.0 (pure Go, no cgo),
`tidwall/jsonc` v0.3.3; dev-only `teatest/v2` v2.0.0-20260816001655-68d539dca504.
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

