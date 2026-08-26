---
type: reference
title: Permission Engine
description: "internal/permission: the ported decision engine (findLast rule evaluation), the build/plan/yolo builtin matrices, doom-loop detection, and the blocking Service that gates session tool execution, persists requests, and parks asks until the TUI replies once/always/reject."
tags: [permissions, engine, decision, ask, doom-loop, builtins, gate]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-b0736174bc9c12e56860bdee
    resource: repo://internal/permission/builtins.go
  - id: openwiki-source-d7514ce9a39b019aeb51f02c
    resource: repo://internal/permission/permission.go
  - id: openwiki-source-9e3eb17a79b2db2b9df2751f
    resource: repo://internal/permission/service.go
  - id: openwiki-source-c8be83aef956b9049cf244dd
    resource: repo://internal/session/tool_exec.go
  - id: openwiki-source-5384f7fe2519c80b0b35ab0d
    resource: repo://internal/tui/permission.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Permission Engine

`internal/permission` ports opencode's permission engine: **rule evaluation
(findLast semantics)**, the **build/plan/yolo matrices**, **doom-loop
detection**, and the **blocking ask/reply service**. The service is the gate
between the session's tool execution and the tool invocation: a tool call may
only run after the engine has decided allow.

## Decision and rule vocabulary

`Decision = Action` has three values: `allow`, `denied`, `ask`. The **rule**
vocabulary stored on `protocol.Rule.Action` (config/wire form) is `allow`,
`deny`, `ask` — note `"deny"` on a rule vs the `Decision` constant `Deny =
"denied"` (`permission.go`).

`Evaluate(rules, action, resources)` applies **findLast semantics**: for each
resource, the **last** rule whose permission matches (`"*"` or exact) and whose
pattern glob-matches the resource decides it. If any resource decides deny →
`Deny`; else any deciding ask (or no matching rule at all) → `Ask`; else
`Allow` (`permission.go`).

## Builtin matrices

`builtins.go` holds the shared **`base`** rule matrix — order is significant
(findLast), so **broad rules come first, narrow rules later**:

| permission | pattern | action |
|---|---|---|
| `*` | `*` | allow |
| `doom_loop` | `*` | ask |
| `external_directory` | `*` | ask |
| `question` | `*` | deny |
| `plan_enter` | `*` | deny |
| `plan_exit` | `*` | deny |
| `read` | `*` | allow |
| `read` | `*.env` | ask |
| `read` | `*.env.*` | ask |
| `read` | `*.env.example` | allow |

Each agent's matrix is `base` plus additions:

- **build** — adds `question allow`, `plan_enter allow`.
- **plan** — adds `question allow`, `plan_exit allow`,
  `external_directory allow` (`<dataDir>/plans/*`), `edit deny`, and
  `edit allow` for `<dataDir>/plans/*.md`.
- **yolo** — a single catch-all `{*,*,allow}` (allows everything).

`LoadBuiltins` errors on unknown agents; **`BuiltinsFor`** is the shared
fallback used by both the service's decision path and the engine's ruleset
path — unknown (config-defined) custom agents evaluate against the **build**
matrix so both see the same verdicts. (The worktree-relative plan rule the
upstream "edit" plan carries is session-dependent; the engine adds it at
session start.)

## Tool hiding and doom-loop detection

- **`Hidden(rules, tools)`** — a tool's permission is `"edit"` for write
  tools, otherwise the tool name itself; a tool is hidden iff the **last** rule
  for that permission (no resource matching) has `Pattern "*"` and `Action
  deny`. The session uses this to fail a hidden tool call with "tool not
  available".
- **`CallKey{Tool, Hash}`** identifies a call for doom-loop detection, `Hash`
  being the sha256 of the canonical JSON args. **`DoomLoopDue`** reports
  whether the next call would be the **third consecutive identical call**
  (last two of the turn's history == next).

## The Service (the gate)

`Service` **enforces + blocks**: the engine evaluates and passes its verdict
via `Request.PreDecision`; the service **persists, parks, and resolves**. It is
constructed with `db`, `bus`, `log`, and the process-constant `dataDir`
(`service.go`).

- **`EvaluateRules`** — builtins (`BuiltinsFor`) + the turn's config rules.
  Session always-rules are **not** included here (they need a session).
- **`DecisionFor`** — re-evaluates builtins + config rules **+ the session's
  persisted always-rules** (`db.AlwaysRules`; a load failure degrades to no
  always-rules — re-asks at worst). The catch-all `"*"` allow in the matrices
  stands in for the no-rule default **only for known core actions**
  (`corePermissions`); unknown actions fall through to `ask`.
- **`Ask`** — a decisive verdict persists the row and returns immediately; an
  `Ask` verdict **parks** the request (a `pendingEntry` with a channel),
  persists it with `response ""` (NULL = pending), publishes
  `permission.asked`, and blocks until `Reply` — or ctx cancel, which stores
  `response='aborted'` and **denies**.
- **`Reply`** — `"once"` → allow (stored `"once"`); `"reject"` → deny (stored
  `"rejected"`) **and cascades the verdict to every other parked request in
  the session**; `"always"` → allow (stored `"always"`) **and `autoAllow`s**
  same-session pendings whose permission matches and whose resources are all
  covered by the just-persisted always patterns.
- **`resolve`** — records the response (`db.ReplyPermission`, best-effort — the
  decision is already in memory), emits `permission.replied`, and delivers the
  decision to the waiting asker. A **double-settle** guard means only the
  remover settles, so a concurrent Reply/cancel that lost the claim cannot flip
  the row or emit a second event.

## How it gates session tool execution

In `internal/session/tool_exec.go`, after a tool call is parsed and the
ruleset loaded, the engine runs the gates **before** the tool runs:

1. **Hidden check** — `permission.Hidden`; a hidden tool fails immediately.
2. **`checkDoom`** — a sliding 3-identical window on the turn's call history;
   the **third identical call asks before it runs** (the ask fires before the
   part goes "running"); a non-Allow decision finalizes the part and stops the
   call.
3. **`gateExternal`** — one ask per external-directory pattern outside the
   session dir; the part is "running" first so the TUI shows the pending
   state.
4. **`coreAsk`** — the core permission ask, with `Resources`/`Always` from
   `tool.Patterns`.

Any `Ask` error, deny, or ctx-cancel while parked finalizes the part
(`gateFail`/`failToolPart`) and stops the call.

## Surfacing to the TUI

`internal/tui/permission.go` renders the pending-ask **overlay** from
`store.Pending` (shown while non-empty, above the prompt) — the **first**
parked ask only: the permission, its patterns (word-wrapped), the suggested
`Always` patterns, the tool-call ref, and the hint line
**`[1] once  [2] always  [3] reject`**. `handlePermKey` owns every key while an
ask is pending — `1`/`2`/`3` reply once/always/reject, `esc` rejects
(locked), everything else is ignored. `replyPermCmd` posts the reply for the
first parked ask; `applyPermReply` drops the answered ask on success
(idempotent with the `permission.replied` event) or toasts and keeps the dialog
on failure.
