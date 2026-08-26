---
type: concept
title: Event Bus and Delivery
description: "End-to-end event flow in yolo: in-process pub/sub (internal/bus), SSE delivery by the server (internal/server/sse.go), the TUI's SSE client and event pump, re-hydration on drop, and the local-only rotating file logger (internal/log)."
tags: [event-bus, sse, pub-sub, logging, tui, delivery]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-a890e919ec17ab42b4e9a3be
    resource: repo://internal/bus/bus.go
  - id: openwiki-source-b3b67094b286e55952c7bbfa
    resource: repo://internal/log/log.go
  - id: openwiki-source-7be0e406371d03d1733b7d8f
    resource: repo://internal/server/sse.go
  - id: openwiki-source-dc1a4c07786693904124eeb5
    resource: repo://internal/tui/app.go
  - id: openwiki-source-8e451f54fa552553ca161ff0
    resource: repo://internal/tui/client/event.go
  - id: openwiki-source-fb3c0893e313eb6d52b0f35e
    resource: repo://internal/tui/hydrate.go
  - id: openwiki-source-73b92f8d965bfdb1db27b2bc
    resource: repo://internal/tui/store/store.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Event Bus and Delivery

yolo has a single in-process event path. Core packages (the session engine,
server handlers) publish `protocol.Event` values onto an in-process pub/sub bus;
the server fans them out over Server-Sent Events (SSE); the TUI subscribes to
that SSE stream and applies events to its display store. There is no replay, no
remote fan-out, and no telemetry.

## The in-process bus

`internal/bus.Bus` is a small pub/sub hub (internal/bus/bus.go:16-65). Each
subscriber owns a buffered channel of `protocol.Event` with a buffer of 1024
(`subscriberBuffer`). `Publish` delivers to every subscriber **non-blockingly**:
a subscriber whose buffer is full is dropped — removed from the set and its
channel closed — so `Publish` can never block or back-pressure the producer. A
dropped subscriber means the SSE connection ends and the client reconnects
(internal/bus/bus.go:52-65). `Subscribe` returns the receive channel plus a
cancel func that unregisters and closes the channel. `SubscriberCount` exists so
tests can wait for an SSE subscriber to register before publishing.

## SSE delivery by the server

`handleEvent` (internal/server/sse.go:9-53) streams bus events as SSE frames. It
sets `Content-Type: text/event-stream`, `Cache-Control: no-cache`, and
`X-Accel-Buffering: no`, flushes the headers, subscribes to the bus, then loops
on `select` between the request context done and the next event. Each event is
marshalled and written as `data: {json}\n\n` followed by a flush. The frame is
written as three separate writes (not `Fprintf`) to avoid `fmt` buffer growth for
large frames. A failed write ends the response — the client reconnects — instead
of holding the bus subscription for a gone client until context cancel. If the
bus closes the channel (subscriber overflow), the handler returns.

## The TUI SSE client

`client.Service.Events(ctx)` (internal/tui/client/event.go:25-52) returns two
channels: `chan protocol.Event` and a `resync` ping channel. A reader goroutine
loops: it reads one `/event` connection to exhaustion (`stream`), and on a dropped
connection (any non-nil error other than clean context done) it pings `resync`,
then backs off (`c.backoff(n)`) and reconnects. The `stream` method
(internal/tui/client/event.go:57-101) issues `GET /event` with the directory
scoping header, scans lines, keeps only `data: ` lines, and unmarshals each into
a `protocol.Event`. The scanner buffer is capped at 4 MiB — a single `data: `
line carries one whole event JSON, and escaped tool output can exceed the former
1 MiB cap; overflow still aborts the stream and fires the resync ping, bounded
by the re-hydrate.

**Drop contract.** The bus has no replay, so events published while the stream
was down are unrecoverable. Every dropped connection pings `resync`; the caller
must re-hydrate its state over REST on each ping. Reconnects must never be silent
again — a pre-v0.1.3 silent reconnect left the footer stuck on `busy` and the
transcript stale (internal/tui/client/event.go:19-24).

## Event pump and store application in the TUI

`App.eventPump()` (internal/tui/app.go:303-316) is a bubbletea cmd that blocks on
the SSE channel and delivers the next event as `EventMsg`, re-arming itself on
every event; on channel close it delivers `connLostMsg` and stops. In
`updateMsg` (internal/tui/app.go:149-251):

- `EventMsg` sets `store.Live = true`, calls `store.Apply(event)`, marks the
  session dirty (a re-render is coalesced once per applied event, not per
  frame), and re-arms the pump.
- `connLostMsg` sets `store.Live = false`.
- `resyncMsg` sets `resyncing = true`, and batches a REST re-hydrate
  (`hydrateCmd`) with a re-armed `resyncPump`; the footer shows the outage
  window until the re-hydrate completes.

`resyncPump` (internal/tui/hydrate.go:27-42) blocks on the resync channel and
delivers `resyncMsg` per ping; a closed channel ends the pump quietly
(`connLostMsg` arrives via the event channel at the same time).

`store.State.Apply` (internal/tui/store/store.go:46-67) is a closed switch over
the wire event types it renders: `message.updated`, `message.part.updated`,
`message.part.delta`, `message.removed`, `message.part.removed`,
`session.updated`, `session.deleted`, `session.status`, `permission.asked`, and
`permission.replied`. Each per-session applicator filters on the current session
(`isCurrent`) before upserting, so events for other sessions do not perturb the
active view.

## Tracing one event

1. The session engine calls `e.publish(type, props)`
   (internal/session/engine.go:360-372), which builds the `protocol.Event` and
   calls `bus.Publish`.
2. `Bus.Publish` fans it out to every subscriber channel
   (internal/bus/bus.go:54-64).
3. The SSE handler's `handleEvent` reads it, marshals, and writes
   `data: {json}\n\n` + flush (internal/server/sse.go:29-51).
4. The TUI client's `stream` scans the line and pushes the event onto the
   `Event` channel (internal/tui/client/event.go:77-93).
5. `App.eventPump` delivers `EventMsg`; `updateMsg` applies it to the store and
   marks the view dirty (internal/tui/app.go:162-168, 303-316).

## The log package

`internal/log` is yolo's leveled file logger: a small `slog` handler on a
rotating append-only file at `<dataDir>/log/yolo.log` (internal/log/log.go:1-11,
48-60). It is local-only — nothing is ever sent anywhere (zero telemetry).

- **Rotation.** The active file rotates to a single-generation backup
  (`yolo.log.1`) when a write would push it past 5 MiB
  (internal/log/log.go:28, 280-342).
- **Level and mirror.** `YOLO_LOG_LEVEL` (DEBUG/INFO/WARN/ERROR, case-insensitive,
  invalid → INFO) is read once; `YOLO_PRINT_LOGS=1` mirrors to stderr via an
  `io.MultiWriter` (internal/log/log.go:54-59, 73-84).
- **Best-effort.** Open/write failures never propagate: an unusable file no-ops
  and each write retries the open. A `nil *Logger` is a no-op; `Noop()` is an
  explicit discard logger (internal/log/log.go:5, 68-71, 291-314).
- **Pinned line format.** Each line is
  `time=<RFC3339 UTC ms> level=<LEVEL> run=<8hex> msg=<msg> k=v ...`, in that
  order (the stdlib `TextHandler` emits `msg` before handler attrs, so the pinned
  order is owned by `pinnedHandler`). `run` is an 8-hex process id (upstream
  `run=` parity). Values that could forge a line or break `key=value` parsing are
  quoted/escaped (CWE-117) (internal/log/log.go:96-243).

## Representative tests

- Bus semantics and overflow-drop are exercised through the server wire-contract
  and SSE ordering suites (see the wire-contract page).
- The SSE drop/resync contract is pinned by the TUI teatest suites and
  `internal/tui/client` tests (reconnects must re-hydrate, never go silent).
- Logger format and rotation are unit-tested in `internal/log`.
