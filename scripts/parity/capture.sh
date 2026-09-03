#!/usr/bin/env bash
# capture.sh — the S8.2 upstream pty-capture (spec §7.3). ON-DEMAND,
# user-run, NEVER CI (the root e2e-live.sh pattern).
#
# Usage: just parity-capture   (or: bash scripts/parity/capture.sh)
#
# Prereqs: node + npm (the opencode-ai@1.18.18 capture runtime), python3
# (stdlib only), go (the S8.1 mock build). Installs the npm package into a
# scratch dir (first run), builds the mock, creates the catalog pin on
# first run (curl + browser UA — the CDN 403s python-urllib), then
# re-captures all 17 surfaces. The re-baselined fixtures are printed —
# commit them in the same commit (root principle 3). Exits 0 with PASS,
# 1 with FAIL.

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRATCH="/tmp/opencode-parity"
CATALOG_URL="https://models.opencode.ai/api.json"
UA="Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
FAIL=0

step() { printf '\n== %s\n' "$1"; }
ok()   { printf '   ok: %s\n' "$1"; }
bad()  { printf '   FAIL: %s\n' "$1"; FAIL=1; }

command -v python3 >/dev/null || { bad "python3 not found"; exit 1; }
command -v node >/dev/null || { bad "node not found"; exit 1; }
command -v npm >/dev/null || { bad "npm not found"; exit 1; }
command -v go >/dev/null || { bad "go not found"; exit 1; }

step "scratch + npm runtime (opencode-ai@1.18.18)"
rm -rf "$SCRATCH/node"
mkdir -p "$SCRATCH/node"
(cd "$SCRATCH/node" && npm init -y >/dev/null && npm install opencode-ai@1.18.18 --loglevel=error) \
  && ok "opencode-ai@1.18.18 installed" || bad "npm install failed"
[ "$FAIL" -eq 0 ] || exit 1

step "mock binary"
(cd "$ROOT" && go build -o "$SCRATCH/mock" ./scripts/parity/mock) \
  && ok "mock built" || { bad "go build failed"; exit 1; }

step "catalog pin"
CATPIN="$ROOT/internal/tui/testdata/parity/catalog-pin.json"
mkdir -p "$(dirname "$CATPIN")"
if [ ! -f "$CATPIN" ]; then
  curl -fsSL -A "$UA" "$CATALOG_URL" -o "$SCRATCH/api.json" \
    && node -e "const fs=require('fs');const c=JSON.parse(fs.readFileSync(process.argv[1],'utf8'));fs.writeFileSync(process.argv[2],JSON.stringify({openai:c.openai}))" "$SCRATCH/api.json" "$CATPIN" \
    && ok "catalog pin created (reduced {openai} snapshot)" || { bad "catalog fetch/trim failed"; exit 1; }
else
  ok "catalog pin present (re-fetch manually to re-baseline it)"
fi

step "capture (17 surfaces, double-run determinism)"
(cd "$ROOT" && python3 scripts/parity/capture.py)
rc=$?
[ $rc -eq 0 ] && ok "fixtures + MANIFEST.json written" || bad "capture.py exit $rc"

exit $FAIL
