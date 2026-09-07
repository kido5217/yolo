# yolo run — Design

Date: 2026-09-07 · Status: approved (user, 2026-09-07) · Bead: `yolo-26j`
Upstream referent: opencode v1.18.18 `opencode run`
(`/tmp/opencode-upstream/packages/opencode/src/cli/cmd/run.ts`,
`packages/schema/src/v1/session.ts`) — reference, not contract (root
principle 2). Scope: a single binary addition (`yolo run`) + the
engine/storage/wire changes it needs. No new dependencies (stdlib + the
existing allowlist only).

## 1. Purpose and non-goals

**Purpose.** `yolo run [message..]` runs a prompt headlessly: it boots the
core server in-process (or attaches to a running `yolo serve` via
`--attach`), sends one user message (optionally with attached local files),
consumes the turn's SSE event stream, and prints the result — plain text on
stdout by default, or NDJSON with `--format json` — then exits. It exists so
yolo's turn pipeline can be driven from scripts and from other harnesses
over the wire contract (v0.4.0+ purpose), and so the TUI parity tip pool can
re-adopt the `opencode run` entries.

**Non-goals (v1).**

- No interactive input: piped stdin is appended to the message; a TTY
  stdin with no positionals is a usage error.
- No `--fork`, `--command`, `--variant`, `--password/--username` (upstream
  auth is out of scope — `yolo serve` has no auth), no run timeout, no
  per-message model/agent override (the wire has no such surface —
  `--model`/`--agent` seed the session at creation only).
- No binary/media (image) support: non-text files are inlined as a
  placeholder line; driver image blocks are a deferred follow-up.
- No `--profile` flag on run: profile selection uses the existing chain
  (`YOLO_PROFILE` env → active marker → `default`) via `buildDeps(wd, "")`.
- No changes to the TUI app itself beyond one transcript render case
  (§8). The TUI stays a pure client (root principle 4).
- No telemetry of any kind (§12).
- No version pin: this ships in the next minor release (0.9.0, pending the
  0.8.0 tag); the spec does not pin the version number.

## 2. Commands and flags

```
yolo run [message..]
```

Cobra leaf `newRunCmd` (cmd/yolo/run.go), sibling to the existing leaves
(`serve`, `auth`, `profile`, `version`); `Args: cobra.ArbitraryArgs`;
registered in `newRootCmd` (`root.AddCommand(...)`). `RunE: runRunE`.
The root `PersistentPreRunE` (`checkOutputFormat`) applies to run
unchanged: `--output json` on run prints
`yolo run: --output is not supported by run` + usage and exits 2
(the D2 ruling — run defines its own `--format`).

Message composition (upstream `resolveRunInput` referent), exactly:
positionals joined with a single space (`strings.Join(args, " ")`); if
stdin is not a TTY and its contents are non-empty: positionals
non-empty → `<joined> + "\n" + <stdin>`; positionals empty → `<stdin>`
alone (no leading newline). The composed message must be non-empty
(`strings.TrimSpace`); absent (no positionals, empty/absent stdin) →
stderr `yolo run: message required` + run usage + exit 2. (Files
without a message are not accepted: text is required on the wire.)

Pre-flight order (pinned — the first failing step wins):

1. cobra flag parse (unknown flag → `yolo run: <pflag error>` + usage,
   exit 2 — the `run()` default path).
2. `--format` value check (`default`|`json`; other →
   `yolo run: unknown format value "<v>"` + run usage + exit 2).
3. workDir resolution from `--dir` (missing/non-dir →
   `yolo run: not a directory: <abs>` + exit 2 — the `workDir` idiom with
   the `yolo run:` prefix).
4. File validation for every `--file` (§3) — local, pre-server; first
   failing file → its pinned stderr line + exit 1.
5. Message-presence check → exit 2 (above).
6. Boot: in-process server (unless `--attach`, §7.4) — a boot failure →
   `yolo run: <err>` + exit 1.
7. `--agent` validation (GET /agent) — MINT PATH ONLY (skipped with
   `--session`/`--continue`, where the agent is ignored): a LIST
   failure (server error) → `yolo run: <err>` + exit 1; an unknown name
   → stderr warning + fallback (below), never an exit.
8. Session resolution (`--session` / `--continue` / mint, §7.1) —
   `--session` 404 → `yolo run: Session not found: <id>` + exit 1.
9. Send (POST message) — 409 → `yolo run: session busy: <id>` + exit 1;
   other 4xx/5xx or connection errors → `yolo run: <server message>` +
   exit 1.
10. Event loop until idle (§6) → exit 0 or 1 per the turn-error flag
    (§7.3); SIGINT → 130 (§7.4).

| Flag | Value | Default | Behavior | Error | Exit |
|---|---|---|---|---|---|
| `message..` (positional) | strings | — | Joined `" "`; non-TTY stdin appended `"\n"+stdin`; must be non-empty after TrimSpace | `yolo run: message required` | 2 |
| `--model <ref>` | `provider/model` | `""` | Seeds the minted session's model via the existing seed path (`CreateSessionWith` → `POST /session {title, agent, model}`; blank → the catalog default). Ignored with `--session`/`--continue` (the session's own model stays). There is no per-message model override on the wire (documented limitation, not an error) | — | — |
| `--agent <name>` | agent name | `""` | Validated against `GET /agent` (name match over the returned `protocol.Agent` list — all yolo agents are `mode: "primary"`, so the upstream subagent leg has no yolo referent). Unknown name → stderr `agent "<name>" not found. Falling back to default agent` + proceed with the server default (upstream warn-and-fallback parity; yolo emits the warning once, pre-run). Seeds the minted session's agent (blank → the storage column default `build`). Ignored with `--session`/`--continue` | (agent LIST failure → `yolo run: <err>`) | 1 (list failure only) |
| `--title <string>` | title | `""` | Session title on minted sessions (blank → the server default `"New session"`). NO upstream 50-char derivation (deviation 11). Ignored with `--session`/`--continue` (the existing title stays; v1 does not PATCH it) | — | — |
| `--dir <path>` | directory | cwd | `workDir` referent: scope dir for the server (`x-yolo-directory`), the base for `--file` resolution, and the `--attach` directory header | `yolo run: not a directory: <abs>` | 2 |
| `--session <id>` | session id | — | `GET /session/{id}` (scoped); 404 → stderr + exit 1. Takes precedence over `--continue` (both given → `--session` wins, `--continue` silently ignored — upstream order) | `yolo run: Session not found: <id>` | 1 |
| `--continue` | bool | false | `GET /session` → the FIRST row of the response is the selected session. The server lists `ORDER BY time_updated DESC` scoped to the request directory (`storage.ListSessions`), so the selection rule is: the session most recently updated in the scope dir. yolo's `protocol.Session` has no parent field (upstream filters `!parentID`), so the list order IS the rule — pinned against the verified `ORDER BY time_updated DESC`. An empty list degrades to minting a new session (upstream parity — its `--continue` also falls through to create), silently, no stderr line | (list 4xx/5xx → `yolo run: <err>`) | 1 |
| `--attach <url>` | server URL | — | The client points at the given URL (`client.New(url, workdir)`) instead of booting the in-process server; no auth (yolo `serve` has none); the `x-yolo-directory` header is still sent with the LOCAL workdir (the attached server's `scope()` requires the dir to exist there — a 400 `not a directory: <dir>` is a user error surfaced per the row below); the process exits when the turn completes (no serve teardown). First wire failure (any client error incl. connection refused) → `yolo run: <err>` + exit 1. No explicit health ping — the first real call (agent list / session op / send) is the liveness check | `yolo run: <err>` (the client error text carries the server's envelope message on 4xx/5xx) | 1 |
| `--file`, `-f <path>` | path, repeatable | — | Client-side, pre-request (§3): resolve → validate → read → data URL. No count cap. Files precede the text in the model input (§5) | `yolo run: File not found: <path-as-given>` · `yolo run: Cannot attach local file larger than 10 MiB or a special file: <path-as-given>` | 1 |
| `--format <default\|json>` | string | `default` | `default` = the plain contract (§6.1); `json` = NDJSON on stdout (§6.2). Validated in pre-flight step 2 | `yolo run: unknown format value "<v>"` | 2 |
| `--auto` | bool | false | Permission asks on the run's session are answered `once` (upstream parity) instead of `reject`; no stderr line (vs the default policy's note) | — | — |
| `--thinking` | bool | false | Gates reasoning output in BOTH formats (upstream non-interactive default is OFF — `run.ts:275`). Off: no reasoning line on stderr and no `reasoning` NDJSON line. On: one stderr line per finalized reasoning part (`default`) / `reasoning` NDJSON line (`json`) | — | — |
| `--output` | (persistent) | — | UNSUPPORTED on run (D2, root `checkOutputFormat`) | `yolo run: --output is not supported by run` | 2 |

## 3. File attachment (`--file`)

All handling is client-side, pre-request (upstream `run.ts:357-414`
parity), for both the in-process and `--attach` paths — yolo decision:
uniform data URLs (upstream uses `file:` URLs for local non-attach sends;
deviation 2). The pure function `resolveFiles(base string, paths
[]string) ([]protocol.FileRef, error)` in run.go implements it; `base` is
the resolved workdir.

Per path, in flag order:

1. **Resolve.** Absolute path → as-is; relative → `filepath.Join(base,
   path)`. (Symlinks are followed — `os.Stat`, mirroring upstream's
   `open()` referent.)
2. **Existence.** `os.Stat` error → `yolo run: File not found:
   <path-as-given>` (the path exactly as the user typed it, not the
   resolved one — upstream parity) → exit 1.
3. **Regularity + size.** `!st.Mode().IsRegular()` or
   `st.Size() > protocol.AttachFileMaxBytes` (10 MiB — the shared
   constant pinned in `internal/protocol/send.go`, §4.2; the cap
   includes special files: the same line covers both) → `yolo run:
   Cannot attach local file larger than 10 MiB or a special file:
   <path-as-given>` → exit 1. A symlink to a regular file within the
   size is accepted (stat follows it).
4. **Read once.** `os.ReadFile` (the bytes are read exactly once and
   reused for mime + URL).
5. **MIME.** No sniffing (upstream sniffs via `FSUtil.mimeType` on the
   attach path — yolo does not): `text/plain` if `utf8.Valid(bytes)`,
   else `application/octet-stream`.
6. **Filename.** `filepath.Base` of the RESOLVED path.
7. **URL.** `data:<mime>;base64,` + standard base64 (RFC 4648, no URL
   alphabet) of the bytes.

No count cap; no duplicate detection.

**Wire shape.** `POST /session/{id}/message` body:

```json
{"text":"What do these say?","files":[{"mime":"text/plain","filename":"notes.txt","url":"data:text/plain;base64,aGVsbG8="},{"mime":"application/octet-stream","filename":"bin.dat","url":"data:application/octet-stream;base64,JQk="}]}
```

- `text` remains required (empty → 400 `empty message`, unchanged).
- `files` is OPTIONAL and **marshals omitted when empty** — the client
  request struct is `Files []protocol.FileRef \`json:"files,omitempty"\``,
  so a no-files send is byte-identical to today's body (`{"text":"hi"}`);
  zero behavior change for existing clients (deviation 10).
- One entry per `--file`, in flag order.

## 4. Wire and storage changes

### 4.1 protocol diff (internal/protocol)

`part.go`:

```go
const (
	PartTypeText      = "text"
	PartTypeReasoning = "reasoning"
	PartTypeTool      = "tool"
	PartTypeFile      = "file"
)

type Part struct {
	ID          string         `json:"id"`
	SessionID   string         `json:"sessionID"`
	MessageID   string         `json:"messageID"`
	Type        string         `json:"type"` // PartTypeText | PartTypeReasoning | PartTypeTool | PartTypeFile
	Text        string         `json:"text,omitempty"`
	CallID      string         `json:"callID,omitempty"`
	Tool        string         `json:"tool,omitempty"`
	State       *ToolState     `json:"state,omitempty"`
	IsSynthetic *bool          `json:"isSynthetic,omitempty"`
	IsIgnored   *bool          `json:"isIgnored,omitempty"`
	Time        PartTime       `json:"time"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	// file parts (yolo run, 2026-09-07): the attached file's content and
	// identity. Omitted (zero values) on every non-file part, so existing
	// wire bytes are unchanged.
	MIME     string `json:"mime,omitempty"`
	Filename string `json:"filename,omitempty"`
	URL      string `json:"url,omitempty"`
}
```

New file `send.go` (the send-request wire DTO — wire shapes live only in
protocol, per `internal/protocol/AGENTS.md`):

```go
// FileRef is one attached file of a send request (client-validated data
// URL, §3).
type FileRef struct {
	MIME     string `json:"mime"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

// SendMessageRequest is the POST /session/{id}/message body: text is
// required; files is omitted when empty (wire compatibility — today's
// body is unchanged for no-file sends).
type SendMessageRequest struct {
	Text  string    `json:"text"`
	Files []FileRef `json:"files,omitempty"`
}
```

### 4.2 Server (internal/server)

`handleSend` (handlers_session.go):

- Decodes `protocol.SendMessageRequest` under an ENDPOINT-SPECIFIC body
  cap: `maxSendBodyBytes = 20 << 20` (20 MiB). The global
  `maxBodyBytes` (10 MiB, deviation 267) is a decoded-BODY bound, and a
  10 MiB file base64-encodes to ≈13.3 MiB inside the JSON body — the
  global cap would 413 the maximum legal file, contradicting the
  design's 10 MiB file bound. 20 MiB covers one max file with JSON
  overhead (≈13.4 MiB) plus headroom for a second file; bodies beyond it
  still 413 via the existing `http.MaxBytesReader` mechanism. All other
  endpoints keep the 10 MiB global cap. Mechanism:
  `decodeWithLimit(w, r, v, limit int64)` in server.go; the existing
  `decode` = `decodeWithLimit(…, maxBodyBytes)`; `handleSend` calls
  `decodeWithLimit(…, maxSendBodyBytes)`. (Deviation 9.)
- Validation (pinned envelope strings, lowercase per the existing
  `invalid body`/`empty message`/`session busy` convention):
  1. `strings.TrimSpace(in.Text) == ""` → 400 `empty message` (unchanged).
  2. Any file entry with `MIME == ""` or `URL == ""` → 400
     `invalid file entry`.
  3. Each data-URL entry (`strings.HasPrefix(URL, "data:")`): parse
     `data:<mime>;base64,<b64>` and base64-decode; a decode failure or
     `len(decoded) > attachFileMaxBytes` (10 MiB, shared constant — see
     the note below) → 400 `file too large`. Non-`data:` URLs skip the
     size re-check (hardening for direct API users; yolo's client always
     sends data URLs).
- Then `s.Engine.Send(ctx, id, in.Text, in.Files, onDone)` (the new files
  parameter, §5.1); the 202/409/404/500 mapping is unchanged
  (`{"message_id": …}` / `session busy` / `session not found` / 500).

Note on the 10 MiB constant: the client (run.go) and the server must not
drift; both read it from ONE place. The spec pins: the constant
`AttachFileMaxBytes = 10 << 20` is defined in `internal/protocol/send.go`
(next to `FileRef`/`SendMessageRequest` — protocol stays wire-level, and
a wire constant is wire-level); run.go and handlers_session.go both use
`protocol.AttachFileMaxBytes`. Package main already imports
`internal/protocol` (deps.go), so no import-cycle or purity concern.

### 4.3 Storage (internal/storage/part_convert.go)

**No migration**: `part.type` is `TEXT NOT NULL` with no check constraint
(`migrate.go:42`), so the new type value `file` needs no schema change.

`ProtocolToPart` gains a `file` case (before the `default` text case):
`StateJSON` = the merged envelope document — the file fields plus the
existing `end`/`synthetic` envelope shape, keys in alphabetical order
(`end` < `filename` < `mime` < `synthetic` < `url`), compact separators,
`end` present iff non-zero, `synthetic` present iff true (the same
convention as the existing hot-path text document). Exact bytes for a
fresh file part (no end/synthetic — file parts carry
`Time.Start` only, same shape as the user text part):

```json
{"filename":"notes.txt","mime":"text/plain","url":"data:text/plain;base64,aGVsbG8="}
```

With the envelope set (not produced by run's persist path, but decoded
by the reader):

```json
{"end":1725676800123,"filename":"notes.txt","mime":"text/plain","synthetic":true,"url":"data:text/plain;base64,aGVsbG8="}
```

`PartToProtocol` gains the matching `case "file":` — decodes
`{filename, mime, url, end?, synthetic?}` into `p.Filename`/`p.MIME`/
`p.URL`/`p.Time.End`/`p.IsSynthetic`; `p.Text` stays `""` (file parts
carry no text column content). The existing `default` case (text/
reasoning) and the `tool` case are untouched.

### 4.4 TUI client (internal/tui/client)

`SendMessage` becomes request-struct based:

```go
// SendMessage is POST /session/{id}/message (202); ErrBusy on 409.
func (c *Service) SendMessage(ctx context.Context, id string, req protocol.SendMessageRequest) (string, error)
```

The body is `req` directly (was `map[string]string{"text": text}`); the
response decode and sentinel mapping (`ErrNotFound`/`ErrBusy`/
`ErrBadRequest`, `httpErr`) are unchanged. Existing call sites compile
with a text-only request — the TUI senders (`internal/tui/commands.go`
`sendMessageCmd`, `homeSubmitCmd`) pass
`protocol.SendMessageRequest{Text: text}`; test call sites
(`cmd/yolo/main_test.go`, `internal/tui/client/client_test.go`,
`internal/tui/app_test.go`, `permission_test.go`, `resync_test.go`)
likewise. All other client methods (`CreateSessionWith`,
`ListSessions`, `GetSession`, `ReplyPermission`, `Events`, `Abort`,
`Status`, `ListMessages`) are reused as-is. A neutral-package extraction
of this client is a DEFERRED follow-up (§11); cmd importing
`internal/tui/client` is accepted for v1 (cmd already imports it — no
purity violation; the purity rule binds `internal/tui` non-test files,
not cmd).

## 5. Engine and model consumption

### 5.1 Engine (internal/session/engine.go)

`Send` gains the file inputs:

```go
func (e *Engine) Send(ctx context.Context, sessionID, text string, files []protocol.FileRef, onDone func(error)) (SendResult, error)
```

It persists the user text part exactly as today, then ONE `file` part
per entry, in flag order (insertion order after the text part):

```go
protocol.Part{
	ID: protocol.NewID("prt"), SessionID: sessionID, MessageID: msgID,
	Type: protocol.PartTypeFile, MIME: f.MIME, Filename: f.Filename, URL: f.URL,
	Time: protocol.PartTime{Start: now}, // no End — same shape as the user text part
}
```

each persisted via `storage.ProtocolToPart` + `UpsertPart` and published
as one `message.part.updated` (the publish path is type-agnostic —
verified: `e.publish` wraps whatever props; `eventSuppressed` switches
on the five prop SHAPES, not part types). The user `message.updated`,
the turn spawn, and the busy/ErrSessionBusy semantics are unchanged.
Existing `Send` call sites (the server handler, the session test
harness) gain the `files` argument (`nil` in the harness's text-only
turns).

### 5.2 History seam (internal/session/history.go)

`mapHistory` (LOCKED mapping) is the ONLY place user parts become model
input; the llm layer receives rendered text (`llm.Message{Role,
Content}`) and needs NO changes for v1. The user case changes from
`content := joinTextParts(mw.Parts)` to `content := userContent(mw.Parts)`:

**Rule.** Let `files` = the message's parts with `Type == PartTypeFile`,
in parts-list order (= insertion order = flag order), and `text` =
`joinTextParts` (text parts joined `"\n"`, unchanged):

1. For each file part, in order:
   - `strings.HasPrefix(MIME, "text/")` (the client sends `text/plain`;
     the rule is written generally on the mime prefix) → DECODE the
     data-URL content (`data:<mime>;base64,<b64>` → base64.StdEncoding
     decode) and inline a block (exact byte format below). A malformed
     data URL (decode failure — an invariant violation, since the client
     and server both validate) degrades to the placeholder line, never a
     turn failure.
   - otherwise → the literal placeholder line
     `[Attached <mime>: <filename>]` (upstream `stripMedia` placeholder
     referent). Binary/media support (image blocks in
     openai.go/anthropic.go, `llm.Message` media lists) is OUT OF SCOPE
     v1 — the placeholder IS the pin (deviation 6).
2. Blocks are joined with `"\n\n"`; if `text` is non-empty, the result
   is `<blocks>\n\n<text>`; if there are no file parts, the content is
   `<text>` ALONE — byte-identical to today for every file-less session
   (the zero-change guarantee).
3. `appendReminders` (plan reminders on the last user message) applies
   to the resulting content, as today.

**Inline block format (yolo-specific pin; upstream's inline format is
not a pin).** For a file with filename `F` and decoded content `C`:

```
--- BEGIN FILE F ---
C
--- END FILE F ---
```

byte-exactly: `"--- BEGIN FILE " + F + " ---\n"` + `C` + (a single
`"\n"` if `C` does not already end with one) + `"--- END FILE " + F +
" ---"`. The markers carry the filename, and bound the content; the
content is UTF-8 (guaranteed by the `text/plain` mime rule). `F` is
`filepath.Base` of the resolved path — NO sanitization: a filename
containing newlines or `---` is embedded as-is (the block markers
degrade, but the format is defined by these bytes and nothing more; the
client pins `F` from the OS, so this is an accepted edge, not an error
case). Multiple file blocks are separated by exactly one blank line
(`"\n\n"`), and exactly one blank line separates the last block from the
text.

**Full example.** Files (flag order): `a.txt` = `hello` (no trailing
newline), `b.txt` = `world\n`; binary `bin.dat` (`application/
octet-stream`); text = `What do these say?`. The user message's model
Content is:

```
--- BEGIN FILE a.txt ---
hello
--- END FILE a.txt ---

--- BEGIN FILE b.txt ---
world
--- END FILE b.txt ---

[Attached application/octet-stream: bin.dat]

What do these say?
```

**Replay pin.** `mapHistory` maps the persisted snapshot on EVERY turn
(`loadHistory` at turn start; `roundAsMessage` appends completed rounds
in memory). Later turns of the same session (`--continue`/`--session`
reuse) inline from the STORED data URL — deterministic, with NO
filesystem access at turn time: the original file can be deleted (or
edited) after the send without breaking replay. Turn time never reads
the original file.

## 6. Output contract

The renderer is a pure event-loop state machine over
`protocol.Event` in cmd/yolo/run_output.go: `newRenderer(format,
thinking bool, sessionID string)` + `apply(ev protocol.Event)`, writing
to injected `io.Writer` sinks (os.Stdout/os.Stderr in production;
`bytes.Buffer` in tests). It filters every event on `sessionID == the
run's session`. No ANSI/SGR bytes ever — plain text, TTY and piped
output byte-identical.

### 6.1 `--format default`

**stdout (the clean channel) — assistant text parts only:**

- `message.part.delta` with `Field == "text"`: the `Delta` bytes are
  appended to stdout VERBATIM as they arrive (live per-token stream).
- `message.part.updated` with `Part.Type == PartTypeText` and
  `Part.Time.End != 0` (the part finalizes): emit exactly one `"\n"` to
  stdout IFF the part's accumulated text (tracked per part id from its
  deltas) does not already end with `"\n"`. Parts that end with a
  newline get no extra one; a part with no preceding deltas (finalized
  without streaming) with text not ending in `"\n"` gets the `"\n"`.
- Reasoning is NOT on stdout EVER. Nothing else (headers, tool lines,
  permission notes, errors) reaches stdout.

**stderr (always, both TTY and piped):**

1. **Header** — printed ONCE, on the FIRST `message.updated` with
   `Info.Role == "assistant"` (upstream parity):
   `> <agent> · <providerID>/<modelID>`. The line is built ONCE by the
   orchestrator from the session object resolved in pre-flight step 8
   (`protocol.Session.Agent`;
   `Session.Model.ProviderID + "/" + Session.Model.ModelID`) and passed
   to the renderer (`newRenderer`'s `header` argument, §8): the session
   row is stable for the run, and yolo's assistant `message.updated`
   info carries `Model *MessageModel` (not a flat modelID), so the event
   stream carries no fields to reconstruct the header from. If
   `Session.Model` is nil/empty, the header is `> <agent>` (no
   separator). Example: `> build · kido/qwen`.
2. **Tool line** — one per FINALIZED tool part
   (`message.part.updated`, `Part.Type == PartTypeTool`, `State != nil`,
   `State.Status == "completed"` or `"error"`):
   - completed: `tool: <tool> <title>` — `title` from
     `State.Title`; when `State.Title` is empty, the line is
     `tool: <tool>` (no trailing space).
   - error: the same line plus ` (error: <State.Error>)` when
     `State.Error` is non-empty (an error status with an empty error
     string prints the plain line).
   Examples: `tool: bash ls` · `tool: read notes.txt (error: command
   aborted)`.
3. **Permission note** — one per REJECTED ask (the default policy;
   `--auto` suppresses it): byte-exactly
   `permission requested: ` + `permission.Permission` + (when
   `len(Patterns) >= 1`: ` (` + strings.Join(Patterns, ", ") + `)`) +
   `; auto-rejecting` (no space before the semicolon; when Patterns is
   empty the line ends at the permission name:
   `permission requested: <permission>; auto-rejecting`). Example:
   `permission requested: edit (notes.txt); auto-rejecting`.
4. **Error line** — one per turn failure: `error: <message>` where
   message = `Info.Error.Message` from a `message.updated` with
   `Info.Role == "assistant"` and `Info.Error != nil` (the
   `surfaceTurnError` shape
   `{type: "unknown"|"aborted"|"overflow", message}` — the engine
   attaches the error to the LAST round's assistant row only). Example:
   `error: model provider unavailable`.
5. **Reasoning line** — ONLY with `--thinking`; one per FINALIZED
   reasoning part (`message.part.updated`, `PartTypeReasoning`,
   `Time.End != 0`): `thinking: <Part.Text>` — the text VERBATIM
   (multi-line reasoning spans multiple physical lines, first line
   prefixed). Example: `thinking: Let me check the file first.`
6. **Agent fallback warning** (pre-run, §2 row `--agent`): `agent
   "<name>" not found. Falling back to default agent`.

Order on stderr is event order; a turn that never reaches an assistant
`message.updated` prints no header.

### 6.2 `--format json` (NDJSON)

One line per FINALIZED event on stdout; nothing else goes to stdout.
Deltas (`message.part.delta`) are NEVER emitted. Envelope (Go struct
marshal — field order IS the byte order):

```json
{"type":"<t>","timestamp":<unix-epoch-ms-as-JSON-number>,"sessionID":"<id>",<data>}
```

`timestamp` = `time.Now().UnixMilli()` at line emission. Lines
(`<data>` after the envelope keys):

| `type` | Trigger | `<data>` |
|---|---|---|
| `step_start` | SYNTHESIZED on each NEW assistant message: a `message.updated` with `Info.Role == "assistant"` whose `Info.ID` was not seen earlier in this run | `{"part":{"type":"step-start","messageID":"<Info.ID>"}}` |
| `text` | `message.part.updated`, text part, `Time.End != 0` | `{"part":<Part JSON>}` (the full `protocol.Part`, its own json tags) |
| `reasoning` | same trigger for a reasoning part; GATED by `--thinking` (flag off → no reasoning lines in either format) | `{"part":<Part JSON>}` |
| `tool_use` | `message.part.updated`, tool part at TERMINAL status (`completed` or `error`) | `{"part":<Part JSON>}` |
| `error` | `message.updated` with `Info.Role == "assistant"` and `Info.Error != nil` (the `surfaceTurnError` surface — yolo's turn-error shape; upstream's `{name,data}` shape is NOT used) | `{"error":{"type":"<unknown\|aborted\|overflow>","message":"<message>"}}` |
| `step_finish` | SYNTHESIZED at turn end: `session.status` idle for the run's session, only if at least one assistant `message.updated` was seen in this run (a failure before any round → no line) | `{"part":{"type":"step-finish","reason":"<finish>","tokens":<Tokens JSON>,"cost":<cost>}}` — field mapping from the LAST assistant `message.updated` info (the engine publishes `finishRound`'s full info before the idle — the order is load-bearing and engine-verified): `reason` = `Info.Finish` (the key OMITTED when `""`); `tokens` = `Info.Tokens` (`{input,output,reasoning,cache:{read,write}}`, the `protocol.Tokens` shape as-is — the key OMITTED when the pointer is nil; yolo has no `total` field — omitted, not invented); `cost` = `Info.Cost` (the key OMITTED when 0 — the wire `cost` is `omitempty`, so 0 means absent on the source too). Every mapped key that is absent on the yolo `Message` is omitted from the line — the pin is the omission rule, not invented defaults |

Exact NDJSON byte examples (one scripted turn, `--format json`,
`--thinking` on):

```json
{"type":"step_start","timestamp":1725676800000,"sessionID":"ses_abc","part":{"type":"step-start","messageID":"msg_1"}}
{"type":"reasoning","timestamp":1725676800050,"sessionID":"ses_abc","part":{"id":"prt_r","sessionID":"ses_abc","messageID":"msg_1","type":"reasoning","text":"Let me check.","time":{"start":1725676800010,"end":1725676800050}}}
{"type":"tool_use","timestamp":1725676800100,"sessionID":"ses_abc","part":{"id":"prt_1","sessionID":"ses_abc","messageID":"msg_1","type":"tool","callID":"prt_1","tool":"bash","state":{"status":"completed","input":{"command":"ls"},"title":"ls","output":"notes.txt","time":{"start":1725676800060,"end":1725676800100}},"time":{"start":0}}}
{"type":"text","timestamp":1725676800150,"sessionID":"ses_abc","part":{"id":"prt_2","sessionID":"ses_abc","messageID":"msg_1","type":"text","text":"Hello","time":{"start":1725676800110,"end":1725676800150}}}
{"type":"step_finish","timestamp":1725676800200,"sessionID":"ses_abc","part":{"type":"step-finish","reason":"stop","tokens":{"input":42,"output":7,"reasoning":0,"cache":{"read":0,"write":0}},"cost":0.001}}
```

and a failed turn's terminal line:

```json
{"type":"error","timestamp":1725676800400,"sessionID":"ses_abc","error":{"type":"unknown","message":"model provider unavailable"}}
```

Field order inside a `Part` follows the struct (id, sessionID,
messageID, type, text, callID, tool, state, isSynthetic, isIgnored,
time, metadata; `ToolState`: status, input, title, output, error,
metadata, time) with `omitempty` fields dropped — the examples above
are byte-exact marshal output for the given values.

Two verified engine facts the emitter must NOT "normalize" away (it
marshals the `Part` as-received, whatever bytes those are):

- TOOL parts carry the real timestamps in `state.time` only; the
  PART-level `time` is the zero value, so every tool part JSON ends in
  `,"time":{"start":0}}` (verified in-tree: `saveToolPart` builds
  `protocol.Part{ID, SessionID, MessageID, Type, Tool, CallID, State}`
  and never sets `Part.Time`; tool part `ID == CallID`).
- `state.metadata` is `omitempty`: the bash tool's completion state
  carries `Metadata` only when the tool emits meta (the example above
  has none; a meta-bearing state appends `"metadata":{...}` before
  `"time"` per the `ToolState` field order).

**json-mode stderr** = the permission note (6.1-3) and the error line
(6.1-4) ONLY — the header, tool lines, reasoning lines, and the agent
warning are SUPPRESSED in json mode (pinned: scripts get one machine
channel).

### 6.3 SSE drop resilience

The bus has no replay (the established SSE drop contract). The client's
`Events` auto-reconnects with backoff and pings `resync` on every drop.
On the resync ping, the loop performs ONE `client.Status()` check: if
the run's session reports `idle` in the returned map, the turn is over —
break (the exit-code settle read below still runs after the break); if
the session is ABSENT from the map (unknown status), keep consuming the
reconnected stream. A turn whose idle event was lost to the drop ends on
that status check. The
exit-code error flag is settled from EVENTS plus, after the loop breaks,
one final `ListMessages` read: the last assistant message with
`Error != nil` in the response sets exit 1 (the persisted row is the
source of truth; this catches an error event lost to a drop).

## 7. Permissions, errors, exit codes, signals

### 7.1 Session resolution

- `--session <id>`: `client.GetSession` (scoped) → the session; 404 →
  `yolo run: Session not found: <id>`, exit 1.
- `--continue`: `client.ListSessions` → first row (§2 row
  `--continue`); empty list → mint.
- otherwise / empty `--continue` list: mint via
  `client.CreateSessionWith(ctx, title, agent, model)` with
  `title = --title or ""` (server default `"New session"`),
  `agent = validated --agent or ""` (storage default `build`),
  `model = --model or ""` (catalog default) — the existing seed path.

### 7.2 Permissions (headless)

On `permission.asked` for the run's session (`Props.SessionID == the
run's session` — pinned filter: asks on OTHER sessions in the same
server, e.g. a concurrent TUI, are IGNORED):

- default policy: `client.ReplyPermission(id, "reject")` + the stderr
  note (6.1-3).
- `--auto`: `client.ReplyPermission(id, "once")`, no stderr line
  (upstream parity).

The builtins' default denies apply unchanged (server-side — the reply
only answers asks that reached the bus). A reply failure (the ask
already resolved/expired, 4xx) is swallowed: no stderr, no exit-code
change — the turn proceeds through the server's own resolution path.

### 7.3 Exit codes

| Condition | Exit | stderr |
|---|---|---|
| Turn completed, no `info.error` | 0 | (decoration only) |
| Turn failure — any assistant `info.error` with type `unknown` or `overflow` | 1 | the `error:` line (6.1-4) |
| Turn failure — type `aborted` arriving via the ENGINE (a turn aborted elsewhere, e.g. a concurrent TUI esc), NOT via run's own SIGINT path | 1 | the `error:` line |
| `--session` 404 | 1 | `yolo run: Session not found: <id>` |
| Send 409 (session busy) | 1 | `yolo run: session busy: <id>` |
| File validation failures (§3) | 1 | `yolo run: File not found: <path>` / `yolo run: Cannot attach local file larger than 10 MiB or a special file: <path>` |
| Any server 4xx/5xx or connection error on `--attach` (and any wire failure on the agent list / session ops / send / abort) | 1 | `yolo run: <server error message>` (the client's `httpErr` text carries the server envelope message) |
| Boot failure (in-process: buildDeps / listen) | 1 | `yolo run: <err>` |
| Missing message (no positionals, empty/absent stdin) | 2 | `yolo run: message required` + run usage |
| Unknown `--format` value | 2 | `yolo run: unknown format value "<v>"` + run usage |
| `--output` on run | 2 | `yolo run: --output is not supported by run` + run usage (the existing `checkOutputFormat` handler, unchanged) |
| `--dir` missing/not a directory | 2 | `yolo run: not a directory: <abs>` |
| Cobra flag parse errors | 2 | `yolo run: <pflag error>` + usage |
| SIGINT | 130 | §7.4 |

Note: the file-error exit 1 sits beside yolo's usage=2 rule by design —
it IS an argument-validation failure, but upstream pins 1 for its file
errors and script compatibility wins (deviation 3).
SIGTERM is NOT specially handled in v1: the default process kill
applies (the in-process server dies with the process; an attached server
is unaffected). No timeout flag in v1 (upstream has none; deferred).

### 7.4 Signals (SIGINT)

`signal.Notify` on `syscall.SIGINT` only, armed after boot, before the
send:

- **First SIGINT, turn not yet started** (no sessionID, or the send not
  yet issued): plain cleanup — `os.Exit(130)`.
- **First SIGINT, turn in flight**: `client.Abort(ctx, sessionID)`
  (POST `/session/{id}/abort`, fresh ctx capped at 10 s — the server
  endpoint waits ≤2 s for the aborted turn to settle idle,
  `handleAbort`'s `WaitIdle` bound, verified in-tree). After the abort
  call RETURNS (or the 10 s cap fires), `os.Exit(130)`. No drain, no
  serve teardown: the in-process server dies with the process; an
  attached server is left running with the turn aborted.
- **Second SIGINT** (at any point): immediate force-kill —
  `os.Exit(130)`, no abort, no drain.
- The `error:` line on the SIGINT path is NOT guaranteed: the event loop
  stops with the process, and whether the engine's aborted
  `message.updated` (`{type:"aborted", message:"aborted by the user"}`)
  is observed before exit is a race the spec does not pin.

## 8. Code layout

**New files.**

- `cmd/yolo/run.go` — `newRunCmd` (cobra leaf; the §2 flag set;
  `Args: cobra.ArbitraryArgs`), `runRunE` (pre-flight order §2, boot
  mirroring `tuiRunE`: `workDir` → `buildDeps(wd, "")` →
  `server.NewServer(*deps)` → `srv.Start("127.0.0.1:0")` →
  `client.New("http://"+ln.String(), wd)`; `--attach` instead:
  `client.New(attachURL, wd)`, no in-process server), `resolveFiles`
  (§3 pure function), the run orchestration (session resolution, send,
  the event loop over `client.Events` with the resync leg §6.3, the
  `ListMessages` settle read, the signal wiring §7.4).
- `cmd/yolo/run_output.go` — the renderer: the event-loop state machine
  + BOTH format renderers as pure functions over `protocol.Event`
  (unit-test without a server; §6 byte pins live in its tests):
   ```go
   type format int // formatDefault, formatJSON

   type renderer struct { /* per-part text, seen messageIDs, last
                            assistant info, header-once, turn-error flag,
                            sinks (io.Writer stdout/stderr) */ }

   func newRenderer(f format, thinking bool, sessionID, header string, now func() int64, out, errw io.Writer) *renderer
   func (r *renderer) apply(ev protocol.Event)
   ```

   `header` is the pre-built `> <agent> · <model>` line (§6.1-1; `""`
   prints nothing — the orchestrator passes it in default format only,
   json mode suppresses it). `now` is `time.Now().UnixMilli` in
   production and a fixed counter in tests, so the NDJSON `timestamp`
   field is byte-pinnable.

**Changed files** (the plan slices these):

| File | Change |
|---|---|
| `cmd/yolo/main.go` | `newRootCmd` registers `newRunCmd()` in `AddCommand` (one line) |
| `internal/protocol/part.go` | `Part` flat `MIME`/`Filename`/`URL` omitempty fields + the four `PartType*` constants (existing literals stay byte-compatible; the comment on `Type` extended) |
| `internal/protocol/send.go` (new) | `FileRef`, `SendMessageRequest`, `AttachFileMaxBytes` |
| `internal/server/server.go` | `decodeWithLimit(w, r, v, limit int64)`; `decode` = the 10 MiB instance |
| `internal/server/handlers_session.go` | `handleSend`: `SendMessageRequest` body, `maxSendBodyBytes = 20 << 20`, the three 400 legs (§4.2), `Engine.Send` files arg |
| `internal/session/engine.go` | `Send(ctx, sessionID, text string, files []protocol.FileRef, onDone)` + the file-part persist/publish loop (§5.1) |
| `internal/session/history.go` | `userContent(parts)` (§5.2 rule + block format + placeholder) replacing the `joinTextParts` call in the user case; `joinTextParts` unchanged |
| `internal/storage/part_convert.go` | the `"file"` case both directions (§4.3) |
| `internal/tui/client/client.go` | `SendMessage` request-struct surface (§4.4) |
| `internal/tui/session.go` | `renderUser` gains a minimal `"file"` render case — one chip line per file part, in part order, rendered AFTER the message's text lines (yolo's own client always sends a text part with file parts, so a file-only message — reachable only via the direct API — renders the `User:` line plus chips): byte-exactly `file: <filename> (<mime>)`, plain/unstyled (`renderUser` produces plain text), e.g. `file: notes.txt (text/plain)` — so resumed sessions don't silently drop attachments from view. TUI layering rule UNCHANGED (no new imports; `renderUser` already takes `protocol.MessageWithParts`) |
| call-site ripple | `internal/tui/commands.go` (2 sends → `SendMessageRequest{Text}`); test call sites: `cmd/yolo/main_test.go`, `internal/tui/client/client_test.go`, `internal/tui/app_test.go`, `permission_test.go`, `resync_test.go`; `internal/session/*_test.go` harness `Send` calls gain `nil` files |

The TUI detail-dialog part switch (`messagedlg.go` `messageView`) is
intentionally NOT touched (its `default: continue` skips file parts
there — out of the approved scope).

## 9. Test strategy

Fake driver throughout (`YOLO_LLM=fake` + `YOLO_FAKE_SCRIPT`); NO network
in any test (root rule). Verified in-tree idioms the legs cite:

- **cmd/yolo**: in-process `run([]string{…}) int` (exit codes) + the
  stdout/stderr pipe-capture helper idiom (`runProfile`,
  main_test.go:906) for output assertions; XDG temp homes via
  `t.Setenv` (`withConfigHome`); the built-binary `exec` idiom
  (`buildBinary`, `TestDispatchExitCodes`) reserved for dispatch-level
  legs; fake-driver env plumbing per
  `TestDrainCancelsBusyTurnAndClosesListener`.
- **internal/server**: `testutil.TestServer` harness + the
  `waitShellPart` poll idiom (server_test.go:25-29 — poll the harness DB
  until the part reaches its terminal state; the wire-side twin,
  client_test.go:100-104, polls `ListMessages`).
- **internal/session**: `newHarness`/`h.build(t)` (harness_test.go) +
  `h.drv.Turns = []fake.Turn{…}` + `h.startSession` + `waitIdle` + the
  `nonTitle(h.drv.Requests())` recorded-request assertion pattern
  (engine_test.go:112+).
- **internal/storage**: direct DAO round-trips over a temp DB
  (`storage.Open(t.TempDir())`).
- **Golden strings**: in-test string literals (the repo idiom — no
  golden files; e.g. `TestOutputUnsupported` byte-pins exact stderr
  lines, the session tests pin exact part contents).

Legs:

- **(a) file-validation table** (cmd package): `resolveFiles` over a
  tempdir — missing file, size `10*1024*1024+1` (rejected) vs exactly
  `10*1024*1024` (accepted), a directory (non-regular), a symlink to a
  regular file (accepted), relative vs absolute path, UTF-8 content →
  `text/plain` vs invalid-UTF-8 bytes → `application/octet-stream`, and
  the DATA-URL bytes pinned exactly (`data:text/plain;base64,` +
  `base64.StdEncoding` of a known byte slice, for both mimes). Plus
  run-level legs via `run(…)` + pipe capture: the two stderr lines
  verbatim + exit 1.
- **(b) NDJSON emitter byte-pins** (cmd package): scripted
  `[]protocol.Event` sequences (a full turn: user `message.updated`,
  busy, assistant `message.updated`, file `message.part.updated`,
  reasoning delta + finalize, tool part running→completed, text deltas +
  finalize, `finishRound`-shaped assistant `message.updated` with
  cost/tokens/finish, idle) → exact expected NDJSON lines (in-test
   goldens per §6.2, the `timestamp` field pinned via the renderer's
   injected `now` counter); the
  `--thinking` OFF leg asserts zero `reasoning` lines; the
  no-assistant-failure leg asserts no `step_finish`; the `error` line
  shape pinned (§6.2 example).
- **(c) default-format renderer** (cmd package): the same scripted
  events → pinned stdout bytes (delta verbatim + the finalize-newline
  rule: a part ending in `"\n"` gets none, one that doesn't gets
  exactly one) and pinned stderr lines (header-once across multiple
  assistant `message.updated`s, tool lines incl. the error append,
  permission note, error line, reasoning lines gated by `--thinking`).
- **(d) server integration** (internal/server, `testutil.TestServer` +
  the `waitShellPart` idiom): POST `/message` with `files` → 202;
  DB-read: the user text part + one `file` part per entry (decoded via
  `PartToProtocol`: `MIME`/`Filename`/`URL`/`Type == "file"`); SSE:
  `message.updated` + one `message.part.updated` per file part; 400
  legs: empty mime, empty url, a data-URL with decoded size
  `10*1024*1024+1` (`file too large`), and the 413 leg beyond
  `maxSendBodyBytes`.
- **(e) engine/fake-driver** (internal/session harness): a `Send` with
  files → the recorded request's user `Content` (the
  `nonTitle(h.drv.Requests())` pattern) equals the §5.2 block format
  byte-for-byte (incl. the placeholder for a non-text entry); REPLAY
  leg: a second scripted turn on the same session, the original temp
  files DELETED between turns → the second request's history still
  carries the inlined blocks from the stored data URL (deterministic,
  no filesystem).
- **(f) storage round-trip** (internal/storage): a file part through
  `ProtocolToPart` → `UpsertPart` → `ListParts` → `PartToProtocol` —
  `state_json` bytes pinned to §4.3's exact document; fields recovered
  exactly; the no-end/synthetic and end/synthetic variants both.
- **(g) `--session`/`--continue`/`--attach` legs** (cmd): `--session`
  against a seeded store (found + 404); `--continue` over two seeded
  sessions with distinct `time_updated` (the newer selected);
  `--attach` against a SECOND in-process server started in-test
  (the `testutil.TestServer` / `server.NewServer` idiom) — the run's
  wire calls land on that server (the turn completes there).
- **(h) SIGINT/abort leg** (cmd, bounded): the extracted signal-path
  function (the first-SIGINT handler §7.4) driven against a real
  in-process server + fake-driver busy turn (the harness
  `waitBusy`-equivalent window): the abort POST is issued, the
  endpoint's ≤2 s settle is observed, the loop stops, and the returned
  exit code is 130. Wall-clock bound: the leg asserts < 10 s (the cap),
  typically < 3 s.
- **(i) exit-code legs** (cmd, table over `run(…)` + capture): the §7.3
  rows — 0 (clean fake turn), 1 (fake-driver failure script →
  `error:` line; busy → `session busy: <id>`; `--session` 404; file
  errors), 2 (missing message; `--format bogus`; `--output json` on
  run; unknown flag).

## 10. Deviations

Logged per root principle 2; the plan lands them as continuous entries
in `docs/superpowers/DEVIATIONS.md` (numbering continues from 295).
Severity low unless noted.

1. **Send body is `{text, files:[…]}`, not upstream's `parts[]` array**
   (wire/low): yolo's POST /message carries the text plus a flat file
   list; upstream posts an array of part objects.
2. **Local files are data URLs in BOTH in-process and attach modes**
   (wire/low): upstream uses `file:` URLs for local non-attach sends;
   yolo always base64-inlines (uniform client path; the server re-checks
   the decoded size).
3. **File-validation errors exit 1 inside yolo's 0/1/2 scheme**
   (cli/low): argument-validation failures exit 2 in yolo (usage), but
   upstream pins 1 for its file errors and script compatibility wins.
4. **SIGINT aborts the turn and exits 130** (cli/low): upstream has no
   SIGINT handling in run (the process dies mid-turn); yolo POSTs
   `/abort` (≤2 s settle) then exits 130, second SIGINT force-kills.
5. **Synthesized `step_start`/`step_finish` NDJSON lines + the yolo-shaped
   `error` line** (wire/low): yolo's wire has no step-start/step-finish
   part types (upstream does) — run synthesizes them from
   `message.updated`/idle, and its `error` line carries yolo's
   `info.error` shape `{type,message}`, not upstream's `{name,data}`
   (upstream's `session.error` event has no yolo referent — turn errors
   surface via `message.updated` `Info.Error`, `surfaceTurnError`).
6. **No binary/media support** (scope/low): non-text files inline as the
   placeholder line `[Attached <mime>: <filename>]`; driver image blocks
   are deferred (upstream inlines/sends media per driver).
7. **`--title` without the 50-char derivation** (cli/low): upstream
   derives the session title from the first 50 chars of the prompt when
   `--title` is empty; yolo leaves the server default (`"New session"`).
8. **`--agent`/`--model` seed the session only** (cli/low): both apply
   at mint via the existing seed path and are ignored with
   `--session`/`--continue` — the wire has no per-message override
   (upstream behaves the same; the wire fact is pinned).
9. **Client-side file validation pre-request + server-side decoded re-cap
   + the 20 MiB send-body cap** (security/low): yolo validates files
   client-side (parity) AND re-validates server-side (hardening for
   direct API users — upstream validates only client-side), and the send
   endpoint's body cap is 20 MiB (the global 10 MiB decoded-body cap of
   deviation 267 would 413 a base64-encoded max-size file); all other
   endpoints keep 10 MiB.
10. **`files` omitted when empty** (wire/low): `json:"files,omitempty"`
    keeps the no-file body byte-identical to today's `{"text":…}` —
    zero behavior change for existing clients (the additive-field rule).
11. **Reasoning output goes to stderr, never stdout, in default format**
    (cli/low): upstream prints `Thinking: …` to stdout (piped and TTY);
    yolo's stdout is the clean text channel, so `--thinking` reasoning
    lands on stderr as `thinking: <text>`.
12. **`--continue` selection = first row of `GET /session`** (cli/low):
    upstream picks the first session WITHOUT a `parentID` from its list;
    yolo's `Session` has no parent field and its list is
    `ORDER BY time_updated DESC`, so the rule is "the most recently
    updated session in the scope dir" (pinned against the verified list
    ordering); an empty list mints a new session (upstream parity).

## 11. Deferred

P4 follow-up beads, discovered-from the `yolo-26j` epic slice (the plan
creates them):

- `--fork` (fork the session before continuing).
- `--command` (server-side slash-command execution as the run's input).
- `--variant` (model variant / reasoning-effort seed).
- Binary/media file support (driver image blocks in
  internal/llm/openai.go + anthropic.go; `llm.Message` media lists —
  replaces the §5.2 placeholder for non-text mimes).
- Run timeout flag (a max-turn-duration bound; v1 has none, upstream
  has none).
- Neutral client package extraction (moving `internal/tui/client` out
  from under `internal/tui/` so cmd owns it directly; v1 reuses it in
  place — accepted, no purity violation).

## 12. Zero telemetry

Root principle 1 applies to run: it adds NO telemetry surface of any
kind — no OTEL, no usage reporting, no new outbound endpoint.
`--attach` connects only to a user-named address (nominally a local
`yolo serve`) with NO auth, and nothing else phones home; the
in-process path binds 127.0.0.1:0.

Env vars run consults — exactly the family `buildDeps`/config/auth/log
already consult; NO new env var: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`,
`XDG_CACHE_HOME` (+ `HOME` fallback), `YOLO_PROFILE` (profile
selection), `YOLO_LLM` + `YOLO_FAKE_SCRIPT` (the scripted fake driver),
`YOLO_LOG_LEVEL` + `YOLO_PRINT_LOGS` (the log package), and the
provider API-key env vars the auth package already resolves (the
existing auth surface, e.g. `OPENCODE_API_KEY` for the zen provider).

## 13. Open questions

None — all rulings were settled at spec-writing time: the 20 MiB
send-body cap (§4.2, deviation 9), the `--continue` selection rule
(§2, deviation 12), SIGTERM unhandled (§7.3), `--title`/`--agent`/
`--model` ignored on `--session`/`--continue` (§2), `--continue` on an
empty session list mints silently (§2), the SSE-drop settle read (§6.3),
and the swallowed permission-reply failure (§7.2).
