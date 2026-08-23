# Yolo — Verified Facts (session memory)

Task status lives in beads (the release epic; `bd ready`) and in `git log
--oneline`. This file holds proven facts a resumed session must not
re-litigate. The append-only deviation audit log lives in `DEVIATIONS.md`
(items 1–66 frozen in `deviations-archive-v0.1.0.md`).

**Status (2026-08-23):** v0.2.0 released — merged to `main` + tagged `v0.2.0` +
release cut. 16-task implementation, gate-green; `just e2e-live` PASS against
the real `https://ai.kido.ws/v1` on 2026-08-23 (pre-tag). Beads: epic
`yolo-8vl` and every subtask closed. Spec:
`docs/superpowers/specs/2026-08-22-v0.2.0-design.md`; plan:
`docs/superpowers/plans/2026-08-22-v0.2.0.md`. 0.3.0 is in progress on branch
`v0.3.0` (no upstream): spec `docs/superpowers/specs/2026-08-23-v0.3.0-design.md`,
Plan 1 `docs/superpowers/plans/2026-08-23-v0.3.0-plan-1-defects.md` (defect
slice, all 39 tasks authored + committed), beads epic `yolo-5hy` / sub-epic
`yolo-5hy.1` / task beads `yolo-5hy.1.1`–`.39`; execution starts at Task 0 via
`bd ready`. The 0.3.0 deferred backlog lives in `docs/superpowers/DEFERRED.md`.

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

v0.2.0 released (2026-08-23, `v0.2.0` → `main` + tag `v0.2.0` + release cut):
all 16 plan tasks done — version wiring + `justfile`, the slog logging rework
(run id, `YOLO_LOG_LEVEL`, `YOLO_PRINT_LOGS` mirror, new log points), and the
12 v0.1.2 backlog fixes (①–⑫ + version stream + logging stream). Deviations
78–88 logged. `just e2e-live` PASS against the real `ai.kido.ws` 2026-08-23.
All beads closed (epic `yolo-8vl` + subtasks + `yolo-k98`); the 0.3.0 deferred
backlog persists in `DEFERRED.md`.
(Prior: v0.2.0 spec written + committed 2026-08-22; v0.1.3 released — PR #7 →
`main` `1d3eca6` + tag + release; the five root causes are deviations 73–77.)

## Open items

- [x] Version wiring — spec'd in v0.2.0 design §2 (ldflags + VCS stamping +
  justfile); tracked as bead `yolo-2bf`.
- [x] All 15 waves full-coverage (wave-1 skipped chunks backfilled — deviation 67; no
  residual `COVERAGE: skipped` notes)

## Key verified facts (so they don't get re-litigated)

- Permission engine = port of `packages/opencode/src/permission/index.ts` + matrices in
`agent/agent.ts` (build/plan/yolo verbatim, Task 10).
- Doom loop = sliding 3-identical window; wildcard-deny hides tool iff last matching rule
is `*` deny; `write`+`edit` both map to permission `edit`.
- Pinned deps: `charm.land/bubbletea/v2` v2.0.8, `charm.land/lipgloss/v2` v2.0.6,
`charm.land/bubbles/v2` v2.1.1, `modernc.org/sqlite` v1.56.0 (pure Go, no cgo),
`tidwall/jsonc` v0.3.3; dev-only `teatest/v2` v2.0.0-20260816001655-68d539dca504.
- Module `github.com/kido5217/yolo`, Go ≥ 1.25 (installed 1.26.5).
- Single deliberate wire deviation: `x-yolo-directory` header.
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
- e2e/endpoint facts (Task 30): `scripts/e2e-live.sh` (entry point `just e2e-live`,
script path unchanged) validated PASS (exit 0) 2026-08-18
BOTH against a mock OpenAI SSE endpoint and the REAL `https://ai.kido.ws/v1`,
and re-validated PASS on 2026-08-23 before the v0.2.0 tag — the spec v1
dogfood success criterion is met (completed `bash ls /tmp` tool call + text reply; abort
idle → `aborted:false`, busy → `aborted:true`; SIGTERM → exit 0). `ai.kido.ws` accepts ANY
bearer token (private endpoint; no auth.json on this host — key order env → auth.json →
config). `GET /global/health` → `{"status":"ok"}`; `/session/{id}/message` rows =
`{"info":{role,error:{type},...},"parts":[...]}` (jq: `.info.role`). Script mechanics:
`req()` must set globals (never run inside `$(…)` — subshell loses `HTTP_STATUS`); boot
from the scratch project dir (deviation 65).
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

