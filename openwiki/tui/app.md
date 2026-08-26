---
type: reference
title: TUI Application
description: "internal/tui: the bubbletea v2 App model with home/session routes, the Update loop and SSE event/resync pumps, the in-memory store (REST hydration + SSE Apply with a per-part delta fast path), the HTTP/SSE client over the wire contract, the key ladder, dialogs, toasts, and the import-purity contract enforced by TestImportsDirection."
tags: [tui, bubbletea, app, store, client, sse, hydration, keymap, teatest]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-dc1a4c07786693904124eeb5
    resource: repo://internal/tui/app.go
  - id: openwiki-source-9c30f75085f59b7711d105f2
    resource: repo://internal/tui/client/client.go
  - id: openwiki-source-8e451f54fa552553ca161ff0
    resource: repo://internal/tui/client/event.go
  - id: openwiki-source-fb3c0893e313eb6d52b0f35e
    resource: repo://internal/tui/hydrate.go
  - id: openwiki-source-99b37a6823820f4cb0c51a48
    resource: repo://internal/tui/imports_test.go
  - id: openwiki-source-9810dd9771e7a2d6b9a8c61d
    resource: repo://internal/tui/keys.go
  - id: openwiki-source-73b92f8d965bfdb1db27b2bc
    resource: repo://internal/tui/store/store.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# TUI Application

`internal/tui` is the **bubbletea v2** frontend. It is a **pure client**: it
talks to the core server **only through the wire contract
(`internal/protocol`)** via `internal/tui/client` — non-test files import only
`internal/protocol`, `internal/tui/*`, the standard library, and the charm
deps (enforced by `TestImportsDirection`).

## The App model

`App` is the root bubbletea model (route, store, dialog stack, SSE event pump)
(`app.go`). It embeds **`*client.Service`** and holds:

- `store.State` — the in-memory server view;
- `route` — `routeHome` / `routeSession` (plus `curSessionID`);
- the per-route models `homeModel`, `sessionModel`, and the always-focused
  `promptModel` (static, non-blinking cursor);
- the `dialogStack`, `toasts`, and `lastErr`;
- the **theme engine** (`engine *theme.Engine`, `theme theme.Theme`) — a nil
  engine runs unthemed (the zero Theme paints nothing);
- the tea plumbing: window `size`, the SSE `eventCh`, the drop `resyncCh`,
  `resyncing`, a `stop` cancel func, and an `emitSink` test seam.

`NewApp(c, s, startSessionID, engine)` builds it: a non-empty `startSessionID`
starts on that session (resume), empty starts at home; it calls
`c.Events(ctx)` to arm the SSE + resync pumps and re-themes. `Init` hydrates
the starting route and arms the SSE + resync pumps; `Update` dispatches the
message and then drains the toast ticks armed during the update (each toast
owns its 4s auto-clear tick).

## The Update loop

`updateMsg` is the message switch. Notable cases:

- **`WindowSizeMsg`** — sizes the prompt line (`width - 3`, since textinput's
  View is prompt(2) + width + cursor(1)) and marks the session dirty so the
  word-wrapped transcript re-wraps at the new width.
- **`EventMsg`** — applies the event to the store, marks the transcript dirty,
  and re-arms the event pump; `afterApply` arms the footer spinner when the
  session is left non-idle.
- **`connLostMsg`** — the SSE channel closed; `Live=false`.
- **`resyncMsg`** — a `/event` connection dropped; the app **re-hydrates the
  current route over REST** (gap events are unrecoverable) and re-arms the
  resync pump; the footer shows the outage window until the re-hydrate
  completes.
- **`hydratedMsg`** — stores the fetched home/session payload.
- **`KeyPressMsg` / `InterruptMsg`** — routed through the key ladder; a SIGINT
  is treated exactly like a ctrl+c keystroke so a pending permission ask or an
  open dialog still owns the keys.
- **`ThemeRefreshMsg`** — see the two-leg retheme below.

The **SSE pumps** are self-re-arming `tea.Cmd`s: `eventPump` blocks on the SSE
channel and delivers the next event (re-arming on each; on channel close it
delivers `connLostMsg`), and `resyncPump` delivers a `resyncMsg` per drop ping.

### Theme refresh (two-leg retheme)

`ThemeRefreshMsg` (sent by `cmd/yolo` on every theme signal — SIGUSR2 via
`theme.WatchThemeSignals`) arms **two `tea.Tick` legs** (mirroring upstream
`THEME_REFRESH_DELAYS`): the **250 ms leg** (`themeReapplyMsg`) regenerates the
system theme; the **1000 ms leg** (`themeCustomsMsg`, last) regenerates the
system theme **and re-discovers customs** — the upstream order is system theme
first. `retheme` reads the engine's active theme and applies the `text` color to
the prompt cursor. The legs are idempotent, so a re-signal that re-arms a second
pair (bubbletea v2 has no tick cancellation) leaves the outcome unchanged. Deep
theming lives in the theme page.

## The client (wire-contract only)

`client.Service` talks to one core server: `BaseURL` plus a scope `Dir`, sent
as the **`x-yolo-directory`** header (omitted when `""`, falling back to the
server work dir). Every REST route routes through `do()` (dir header, JSON
body, JSON decode, error mapping): **404 → `ErrNotFound`, 409 → `ErrBusy`,
400 → `ErrBadRequest`**, each carrying the server's error-envelope message.
Routes include Health, session list/create/get/patch/delete, `ListMessages`,
`SendMessage` (202; `ErrBusy` on 409), `Abort`, `Command`, `Status` (plain
`idle|busy|retry` strings per session id), `ListProviders`, get/patch config,
global config, `Auth`, `ListAgents`, `ListCommands`, `ListPermissions`, and
`ReplyPermission`.

`Events(ctx)` streams server events from **`GET /event` (SSE)** until ctx is
done. On a dropped connection it **backs off and reconnects**, and **pings the
resync channel on every drop** — events published while the stream was down are
lost (the bus has no replay), so the caller must re-hydrate over REST on each
ping. The SSE scanner allows a **4 MiB max token** (a single `data:` line
carries one whole event JSON; escaped tool output can exceed 1 MiB).

## Store: hydration + SSE Apply

`store.State` is the single shared TUI state (sessions, current session,
messages, providers, agents, commands, config, status, pending permission asks,
`Live`, `LastHydrate`). The app loop **hydrates it over REST** (`hydrateCmd`:
home = `ListSessions`+`ListCommands`+`GetConfig`; session = `GetSession`+
`ListCommands`+`ListMessages`, each with a 5s timeout) and calls **`Apply` per
SSE event**, which folds the message/part/session/permission event families
into state — **only the current session's messages are tracked**.

A resume that hits a missing session sets `lastErr`, exits to home, and the cmd
layer maps that Quit to exit code 2. `createSessionCmd` creates a session with
server-side defaults, `putSessionFirst` upserts it at the head of the home list,
and `openSession` routes into it and re-hydrates.

The **delta fast path** is the store's performance core: `State.parts` holds,
per streamed part, its location in `Messages` plus a `strings.Builder` shadow,
so a per-token delta is one `Write` instead of re-copying the whole accumulated
text. A full part update is authoritative and drops the shadow (the next delta
re-seeds); a message/part removal re-derives the index; `ForgetParts` drops
every part state when `Messages` is replaced wholesale (a session-route
hydrate).

## Key ladder, dialogs, toasts

`handleKey` is the key dispatcher, a strict ladder:
**permission > dialog > model/agent openers > slash menu > route
(session/home) > prompt**. A pending permission ask owns every key
(`1`/`2`/`3`/`esc`); an open dialog owns its keys; the slash menu (live-filtered
over `store.Commands`) owns arrows/enter/esc; routes handle their navigation
keys; everything else falls through to the always-focused prompt input.

`promptEnter` implements the **locked send semantics**: a trailing backslash
soft-enters a draft line; empty input is ignored; a busy store toasts;
otherwise the draft+line is sent and the input clears **only on success** (a
server-side busy error leaves the line for retry).

`view.go` renders the active route, the dialog overlay, and the last error line
into a `tea.View` (bubbletea v2's Model interface returns `tea.View`, not
`string`); the plain-string composition lives in `a.view()` for unit testing,
and AltScreen keeps the TUI in the alternate screen buffer.
