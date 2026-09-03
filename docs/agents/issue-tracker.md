# Issue tracker: beads (bd)

Issues and specs for this repo live in **beads** — a local, Dolt-backed issue store
(`.beads/`). Operate through the `bd` CLI; where the harness exposes a beads MCP
server, its tools are equivalent — use whichever is available, fall back to the CLI.

## Conventions

- **Find ready work**: `bd ready --json` — run before asking "what should I work on?"
- **Create an issue**: `bd create "Title" --description="why + what" -t bug|feature|task|epic|chore -p 0-4 --json`
- **Hierarchical child**: `--parent <id>` (inherits labels)
- **Link discovered work**: `--deps discovered-from:<parent-id>` at create, or `bd dep add <issue> <depends-on>` (issue depends on depends-on)
- **Claim**: `bd update <id> --claim --json`
- **Label**: `bd tag <id> <label>` / `bd label` (manage) / `--labels` at create
- **Close**: `bd close <id> --reason "..." --json` (multiple ids per call)
- **Show**: `bd show <id> --json`
- Programmatic use: always `--json`; distinguish parsing errors from command failures.

## When a skill says "publish to the issue tracker"

Create a beads issue (`bd create ...`) — not a GitHub issue.

## When a skill says "fetch the relevant ticket"

Run `bd show <id>` (add `--json` for programmatic reads).

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: an epic issue (`bd create "..." -t epic`) holding the Notes / Decisions-so-far / Fog body.
- **Child ticket**: a beads child (`--parent <map-id>`), typed per work kind.
- **Blocking**: native beads dependencies — `bd dep add <child> <blocker>` (child depends on blocker). A ticket is unblocked when every blocker is closed.
- **Frontier query**: the map's open children (`bd children <map-id>`), filtered to ready (`bd ready`) and unassigned; first in map order wins.
- **Claim**: `bd update <id> --claim` — the session's first write.
- **Resolve**: `bd close <id> --reason "<answer>"`, then append a context pointer to the map's Decisions-so-far.

## Sync

Issue history lives in the local Dolt DB. Remote sync: `bd dolt push` / `bd dolt pull`
(uses `refs/dolt/data` on the git remote). `.beads/issues.jsonl` is a passive export —
never the sync protocol.
