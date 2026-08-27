#!/usr/bin/env bash
set -euo pipefail

# wiki-stale: tell whether the generated openwiki/ index is current with HEAD.
# Exit 0 = current, 1 = stale (source moved since the wiki last documented it).
# Used as the pre-merge gate in the root AGENTS.md superpowers workflow.

root="$(git rev-parse --show-toplevel)"
meta="$root/openwiki/.last-update.json"

if [[ ! -f "$meta" ]]; then
  echo "no wiki metadata: $meta" >&2
  echo "run an OpenWiki init/update first" >&2
  exit 1
fi

git_head="$(jq -r '.gitHead' "$meta" 2>/dev/null || true)"
if [[ -z "$git_head" || "$git_head" == "null" ]]; then
  echo "no gitHead in $meta — cannot determine staleness" >&2
  exit 1
fi

head="$(git rev-parse HEAD)"

if [[ "$git_head" == "$head" ]]; then
  echo "wiki current (gitHead == HEAD)"
  exit 0
fi

# The wiki commit itself (openwiki/ output + the root CLAUDE.md pointer) never
# makes the wiki stale: only source outside openwiki/ since gitHead counts.
# gitHead is the wiki commit's parent after a refresh, so a pure-wiki commit
# must not trip the gate.
if git diff --quiet "$git_head"..HEAD -- . ':(exclude)openwiki/**' ':(exclude)CLAUDE.md'; then
  echo "wiki current (source unchanged since gitHead)"
  exit 0
fi

echo "wiki STALE: gitHead $git_head, HEAD $head" >&2
echo "refresh via the openwiki skill (host-driven update) before merging" >&2
exit 1
