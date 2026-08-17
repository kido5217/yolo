# Yolo — Progress & Status (session checkpoint)

**Updated:** 2026-08-17 (session restart checkpoint)

## Where we are

Superpowers **brainstorming** is COMPLETE. The design spec is written, source-verified, self-reviewed, and committed. We are at the **"user reviews written spec"** gate. After user approval, the ONLY next step per the superpowers flow is the **`writing-plans` skill** (spec → implementation plan). No implementation code exists yet.

## Resume instructions (next session)

1. Repo: `/home/kido/network/projects/yolo` (branch `brainstorm`). State is fully in git — see `git log`.
2. Ask the user to review the spec (`docs/superpowers/specs/2026-08-17-yolo-go-port-design.md`), or if already reviewed/approved go straight to step 3.
3. On approval: invoke the `writing-plans` skill and produce the implementation plan from the spec (milestones M0–M8 in spec §8 define the skeleton). Do NOT start writing code before the plan exists.
4. Upstream reference clone (`/tmp/opencode-upstream`, tag `v1.18.18`) lives in /tmp and is likely gone after restart. Recreate if needed:
   `git clone --depth 1 --branch v1.18.18 https://github.com/anomalyco/opencode /tmp/opencode-upstream`
   (`/tmp/opencode` is pre-existing user data — never touch it.)

## Completed work this session

| Item | Where |
|---|---|
| Requirements gathered (6 decision questions): TUI-only, bubbletea **v2** (`charm.land/*`), opencode provider = paid models only, 7 Google-adapter zen models excluded (57/64 kept), `kido` provider default (`ai.kido.ws/v1`, model `Qwen3.8-27B`, key optional), opencode config schema under `yolo.json`/`yolo.jsonc` names, auth = env → `auth.json` → config + `yolo auth` CLI, agents `build`/`plan`/`yolo` (yolo = permit-all), header rename `x-yolo-directory` (only deliberate wire deviation) | spec §1 |
| All 7 design sections presented and approved (LGTM) by user | spec §2–§8 |
| Spec written + committed | `f54991c` |
| Self-review vs v1.18.18 source; fixed: permission semantics (last-match-wins, ask fallback, doom-loop × 3 identical inputs, reject cascade, wildcard-deny hides tool from model — upstream docs' `.env` "deny" is stale vs code's "ask"), zen catalog TTL 60min→**5min**, teatest path `charm.land/x/exp/teatest`, cross-ref bug, `always_json` permission column | `cec9a8f` |

## Key verified facts (so they don't get re-litigated)

- Permission engine = port of `packages/opencode/src/permission/index.ts` + agent rules in `agent/agent.ts` (v1 permission system is what v1.18.18 runs). `evaluate()` = `findLast` match, fallback `ask`; effective order: `[…agent base, …user config permission, …always-approvals]` (last wins).
- Build agent base: `*` allow, `doom_loop` ask, `external_directory` ask (+whitelist), `question` allow, `plan_enter` allow, `plan_exit` deny, read `*.env`/`*.env.*` ask.
- Pinned deps: `charm.land/bubbletea/v2` v2.0.8, `charm.land/lipgloss/v2` v2.0.6, `charm.land/bubbles/v2` v2.1.1, `modernc.org/sqlite` v1.56.0, `tidwall/jsonc` v0.3.3. Dev-only: `charm.land/x/exp/teatest`. Everything else stdlib (`net/http` ServeMux, `flag`).
- Module `github.com/kido5217/yolo`, Go ≥ 1.25 (bubbletea v2 requirement; installed 1.26.5).
- Single deliberate wire deviation: `x-yolo-directory` header.

## Open items

- [ ] User review of the spec
- [ ] `writing-plans` skill: turn spec into an implementation plan (follow milestones M0-M8)
- [ ] Then execute the plan (fresh session or this one, per plan)
