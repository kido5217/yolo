#!/usr/bin/env bash
# e2e-live.sh — ON-DEMAND live end-to-end check against a real kido endpoint.
# NEVER run in CI. User-run only.
#
# Usage:
#   KIDO_API_KEY=... scripts/e2e-live.sh
#
# Env:
#   KIDO_API_KEY   (required) API key for the kido provider
#   KIDO_BASE_URL  (optional) endpoint override, default https://ai.kido.ws/v1
#                  (the code never reads this env var; the script writes it
#                  into the scratch project yolo.jsonc and boots the server
#                  from that dir, whose config builds the provider registry)
#   E2E_TIMEOUT    (optional) per-turn poll timeout seconds, default 180
#
# Boots `yolo serve` in a scratch project dir, then drives the wire contract:
#   1. health
#   2. create session (agent=yolo)
#   3. session list + rename (GET /session, PATCH title, re-list)
#   4. config theme round-trip (GET /config, PATCH {"theme":"aura"}, re-GET)
#   5. send "list files in /tmp" -> expect a completed read/glob tool call
#      plus a non-empty assistant text reply
#   6. abort test (deterministic: abort while idle -> aborted:false, then a
#      best-effort abort of a busy turn)
#   7. SIGTERM the server -> expect graceful shutdown, exit 0
# The TTY-only S3 smoke legs (help, retry-action dialog, theme-list UI, mode
# switch/lock) remain user-run; this script covers the wire side only.
# Exits 0 with PASS, 1 with FAIL.

set -uo pipefail

BASE_URL="${KIDO_BASE_URL:-https://ai.kido.ws/v1}"
TURN_TIMEOUT="${E2E_TIMEOUT:-180}"
FAIL=0

step() { printf '\n== %s\n' "$1"; }
ok()   { printf '   ok: %s\n' "$1"; }
bad()  { printf '   FAIL: %s\n' "$1"; FAIL=1; }

command -v go >/dev/null || { echo "FAIL: go not found"; exit 1; }
command -v curl >/dev/null || { echo "FAIL: curl not found"; exit 1; }
command -v jq >/dev/null || { echo "FAIL: jq not found"; exit 1; }
[ -n "${KIDO_API_KEY:-}" ] || { echo "FAIL: KIDO_API_KEY is required"; exit 1; }

ROOT="$(mktemp -d /tmp/yolo-e2e.XXXXXX)"
BIN="$ROOT/yolo"
PROJ="$ROOT/project"
SERVER_LOG="$ROOT/server.log"
SERVER_PID=""
mkdir -p "$PROJ"
# point the kido provider at the endpoint via project config (wins over the
# builtin default https://ai.kido.ws/v1)
jq -n --arg base "$BASE_URL" '{provider: {kido: {baseURL: $base}}}' >"$PROJ/yolo.jsonc"

cleanup() {
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null
    wait "$SERVER_PID" 2>/dev/null
  fi
}
trap cleanup INT TERM EXIT

uri() { jq -rn --arg p "$1" '$p | @uri'; }
# req METHOD PATH [BODY] -> sets globals HTTP_STATUS and BODY (call directly,
# never inside a command substitution)
req() {
  local method="$1" path="$2" body="${3:-}"
  local args=(-sS -o "$ROOT/last_body" -w '%{http_code}' -X "$method" \
    -H "x-yolo-directory: $(uri "$PROJ")" "$BASE_API$path")
  [ -n "$body" ] && args+=(-H "Content-Type: application/json" -d "$body")
  HTTP_STATUS="$(curl "${args[@]}" 2>/dev/null)"
  BODY="$(cat "$ROOT/last_body" 2>/dev/null)"
}

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
step "build"
( cd "$REPO" && go build -o "$BIN" ./cmd/yolo ) || { echo "FAIL: go build"; exit 1; }
ok "built $BIN"

step "server"
# boot from PROJ: the provider registry (and thus the kido baseURL) is built
# from the server's startup-dir config at process start, so the scratch
# project's yolo.jsonc is what points the run at $BASE_URL.
( cd "$PROJ" && exec env KIDO_API_KEY="$KIDO_API_KEY" "$BIN" serve --addr 127.0.0.1:0 ) >"$SERVER_LOG" 2>&1 &
SERVER_PID=$!
BASE_API=""
for _ in $(seq 1 50); do
  if grep -q "yolo serving on" "$SERVER_LOG" 2>/dev/null; then
    BASE_API="$(sed -n 's/^yolo serving on \(http:\/\/[^ )]*\).*/\1/p' "$SERVER_LOG")"
    break
  fi
  kill -0 "$SERVER_PID" 2>/dev/null || { bad "server died at startup"; cat "$SERVER_LOG"; exit 1; }
  sleep 0.2
done
[ -n "$BASE_API" ] || { bad "server did not report address"; cat "$SERVER_LOG"; exit 1; }
ok "$BASE_API (pid $SERVER_PID)"

step "1. health"
req GET /global/health
b=$BODY
if [ "$HTTP_STATUS" = "200" ] && jq -e '.status == "ok" or .ok == true' <<<"$b" >/dev/null; then
  ok "health ok"
else
  bad "health: $HTTP_STATUS $b"
fi

step "2. create session (agent=yolo)"
req POST /session '{"title":"e2e","agent":"yolo"}'
b=$BODY
SESS_ID="$(jq -r '.id // empty' <<<"$b")"
if [ "$HTTP_STATUS" = "201" ] && [ -n "$SESS_ID" ]; then
  ok "session $SESS_ID"
else
  bad "create session: $HTTP_STATUS $b"
fi

wait_text() { # $1 timeout_s ; polls /message for a completed assistant text part
  local deadline=$((SECONDS + $1))
  while [ $SECONDS -lt $deadline ]; do
    req GET "/session/$SESS_ID/message"
    b="${BODY:-}"
    if [ "$(jq -r '[.[] | select(.info.error.type == "aborted")] | length' <<<"$b" 2>/dev/null)" != "0" ]; then
      return 2
    fi
    if jq -e '[.[] | select(.info.role == "assistant") | .parts[]? | select(.type == "text" and .text != "")] | length > 0' <<<"$b" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

if [ -n "$SESS_ID" ]; then
  step "3. session list + rename"
  req GET /session
  b=$BODY
  if [ "$HTTP_STATUS" = "200" ] && jq -e --arg id "$SESS_ID" \
    '[.[] | select(.id == $id and .agent == "yolo")] | length > 0' <<<"$b" >/dev/null 2>&1; then
    ok "session $SESS_ID (agent=yolo) in list"
  else
    bad "list sessions: $HTTP_STATUS $b"
  fi
  req PATCH "/session/$SESS_ID" '{"title":"e2e-renamed"}'
  b=$BODY
  if [ "$HTTP_STATUS" = "200" ] && [ "$(jq -r '.title // empty' <<<"$b")" = "e2e-renamed" ]; then
    ok "rename -> title e2e-renamed"
  else
    bad "rename session: $HTTP_STATUS $b"
  fi
  req GET /session
  b=$BODY
  if [ "$HTTP_STATUS" = "200" ] && jq -e --arg id "$SESS_ID" --arg t "e2e-renamed" \
    '[.[] | select(.id == $id)] | length == 1 and .[0].title == $t' <<<"$b" >/dev/null 2>&1; then
    ok "re-list: rename persisted"
  else
    bad "rename not in list: $HTTP_STATUS $b"
  fi

  step "4. config theme round-trip"
  req GET /config
  b=$BODY
  if [ "$HTTP_STATUS" = "200" ] && [ "$(jq -r '.provider.kido.baseURL // empty' <<<"$b")" = "$BASE_URL" ]; then
    ok "config loaded from scratch project"
  else
    bad "get config: $HTTP_STATUS $b"
  fi
  req PATCH /config '{"theme":"aura"}'
  b=$BODY
  if [ "$HTTP_STATUS" = "200" ] && [ "$(jq -r '.theme // empty' <<<"$b")" = "aura" ]; then
    ok "theme set to aura"
  else
    bad "patch config theme: $HTTP_STATUS $b"
  fi
  req GET /config
  b=$BODY
  if [ "$HTTP_STATUS" = "200" ] && [ "$(jq -r '.theme // empty' <<<"$b")" = "aura" ]; then
    ok "re-get: theme aura persisted (string)"
  else
    bad "theme not persisted: $HTTP_STATUS $b"
  fi

  step "5. turn: 'list files in /tmp'"
  req POST "/session/$SESS_ID/message" '{"text":"list files in /tmp"}'
  b=$BODY
  MID="$(jq -r '.message_id // empty' <<<"$b")"
  if [ "$HTTP_STATUS" = "202" ] && [ -n "$MID" ]; then
    ok "accepted (message $MID)"
  else
    bad "send message: $HTTP_STATUS $b"
  fi

  rc=0
  wait_text "$TURN_TIMEOUT"; rc=$?
  req GET "/session/$SESS_ID/message"
  b=$BODY
  case $rc in
    0)
      ok "assistant text reply arrived"
      ;;
    2)
      bad "turn aborted before reply; server log tail:"
      tail -5 "$SERVER_LOG"
      ;;
    *)
      bad "no assistant text reply within ${TURN_TIMEOUT}s; server log tail:"
      tail -5 "$SERVER_LOG"
      ;;
  esac
  if [ -n "$SESS_ID" ]; then
    tool_states="$(jq -r '[.[] | select(.info.role == "assistant") | .parts[]? | select(.type == "tool")] as $ts
      | [ $ts[] | {tool, status: .state.status} ] | @json' <<<"$b" 2>/dev/null)"
    ok "tool parts: ${tool_states:-none}"
    if jq -e '
        [ [ .[] | select(.info.role == "assistant") | .parts[]? | select(.type == "tool") ][]
          | select(.state.status == "completed"
                   and (.tool == "read" or .tool == "glob" or .tool == "grep" or .tool == "bash"))
        ] | length > 0' <<<"$b" >/dev/null 2>&1; then
      ok "completed file-listing tool call present"
    else
      bad "no completed tool call in turn"
    fi

    wait_idle() { # $1 deadline_s
      local end=$((SECONDS + $1))
      while [ $SECONDS -lt $end ]; do
        req GET /session/status
        [ "$(jq -r --arg id "$SESS_ID" '.sessions[$id] // "missing"' <<<"$BODY")" = "idle" ] && return 0
        sleep 0.5
      done
      return 1
    }

    step "6. abort test"
    # the turn may still be settling after its text reply; require idle first
    if wait_idle 20; then
      ok "session idle before abort test"
    else
      bad "session not idle 20s after turn"
    fi
    req POST "/session/$SESS_ID/abort" '{}'
    b=$BODY
    if [ "$HTTP_STATUS" = "200" ] && jq -e '.aborted == false' <<<"$b" >/dev/null; then
      ok "abort while idle -> aborted:false"
    else
      bad "abort idle: $HTTP_STATUS $b"
    fi
    # best-effort: start a turn, abort while busy (may legitimately complete first)
    req POST "/session/$SESS_ID/message" '{"text":"write a 300 word essay about the history of file systems"}' >/dev/null
    BUSY=0
    for _ in $(seq 1 20); do
      req GET /session/status
      st=$BODY
      if [ "$(jq -r --arg id "$SESS_ID" '.sessions[$id] // empty' <<<"$st" 2>/dev/null)" = "busy" ]; then
        BUSY=1
        break
      fi
      sleep 0.5
    done
    req POST "/session/$SESS_ID/abort" '{}'
    b=$BODY
    aborted="$(jq -r '.aborted' <<<"$b" 2>/dev/null)"
    if [ "$BUSY" = "1" ] && [ "$aborted" = "true" ]; then
      ok "abort while busy -> aborted:true"
    else
      ok "abort while busy skipped (turn finished first; aborted=${aborted:-n/a})"
    fi
    # session must return to idle
    deadline=$((SECONDS + 15))
    IDLE=0
    while [ $SECONDS -lt $deadline ]; do
      req GET /session/status
      st=$BODY
      [ "$(jq -r --arg id "$SESS_ID" '.sessions[$id] // empty' <<<"$st" 2>/dev/null)" = "idle" ] && { IDLE=1; break; }
      sleep 0.5
    done
    [ "$IDLE" = "1" ] && ok "session back to idle" || bad "session not idle 15s after abort"
  fi
fi

step "7. graceful shutdown (SIGTERM)"
kill -TERM "$SERVER_PID"
deadline=$((SECONDS + 6))
while kill -0 "$SERVER_PID" 2>/dev/null && [ $SECONDS -lt $deadline ]; do
  sleep 0.2
done
if kill -0 "$SERVER_PID" 2>/dev/null; then
  bad "server still alive 6s after SIGTERM"
  kill -9 "$SERVER_PID" 2>/dev/null
else
  wait "$SERVER_PID" 2>/dev/null
  rc=$?
  if [ $rc -eq 0 ]; then
    ok "server exited 0"
  else
    bad "server exit code $rc (want 0)"
  fi
fi
SERVER_PID=""

step "result"
if [ "$FAIL" = "0" ]; then
  echo "PASS"
  exit 0
fi
echo "FAIL"
echo "--- server log tail ---"
tail -20 "$SERVER_LOG"
exit 1
