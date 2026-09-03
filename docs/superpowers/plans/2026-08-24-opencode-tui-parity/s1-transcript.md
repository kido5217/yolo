# S1 — transcript rendering (slice bead `yolo-oae.2`)

Render the session transcript with glamour — GFM markdown + per-language
chroma syntax highlighting driven by the resolved S0 theme — and restyle
the reasoning, tool-row, and error surfaces to the upstream tokens.

**State: fully detailed** — this file holds the full 5-step TDD detail for
all nine tasks (S1.1–S1.9, `## S1 detail` below). Execution proceeds
bead-by-bead per the binding table; each task ends gate-green + committed
with its pinned message.

## Binding task table

Pointer only (FROZEN — Slice Detail Protocol rule 1): `plan.md` →
`## Task inventory` → `### S1 — transcript rendering (slice bead yolo-oae.2)`.
Bead titles, scope, and pinned commit messages live there and may not be
changed by a detail pass; any required change = STOP + explicit user
approval + re-record in plan.md.

## Dep gate

`charm.land/glamour/v2` v2.0.1 — dep-proposal bead first (root AGENTS.md
dependency policy: evidence from live web search — maintenance, license,
pure Go, transitive surface; approval gate = STOP before `go get`; lands as
task S1.1).

## Exact upstream sources (the detail pass reads these AT DETAIL TIME)

`/tmp/opencode-upstream` @ v1.18.18:

- `packages/tui/src/routes/session/index.tsx` — the assistant text-part
  markdown render 1591–1720 (`createSyntaxStyleMemo(generateSubtleSyntax(theme))`
  1607, `<markdown fg={theme.markdownText} syntaxStyle={syntax()}>`
  1700–1707), further markdown usages 2114–2129, the assistant error box
  1534–1548, `InlineTool`/`InlineToolRow` 1850–2000, `BlockTool` 2040–2050
  (S0.10 restyled yolo's tool-row CHROME; S1.7 ports the upstream tool-row
  rendering semantics — read the S0.10 notes in `s0-theme-engine.md` for the
  handoff), the reasoning block (grep `reasoning` in this file).
- `packages/tui/src/theme/index.ts` — `generateSubtleSyntax` + the
  markdown/syntax token map (the port source for the glamour `StyleConfig`
  + chroma token map in S1.2).
- `packages/tui/src/theme/assets/*.json` — the `markdown*` and `syntax*`
  keys (flat: `markdownText`, `markdownCodeBlock`, `syntaxComment`,
  `syntaxKeyword`, … — the S0.1 embedded `ThemeJson` model may need
  extending; the detail pass decides and says so).
- the opentui `<markdown>` element (GFM feature set + chroma token names):
  the bundled core at `/tmp/opencode/.opentui-core/` (grep `markdown` for
  the chunk) or `node_modules/@opentui/core` in the upstream tree — map its
  options + token list onto glamour v2.0.1's `StyleConfig`.

## yolo anchors

- `internal/tui/session.go` — the transcript render path; S1.3 replaces the
  plain wrap for text parts.
- `internal/tui/theme/` — renderer home; `syntax.go` (ported
  `generateSyntax`/`generateSubtleSyntax`) lands here with Task S1.2 per the
  S0 package-layout note.
- `internal/tui/session_bench_test.go` — S1.9 extends it (re-render benchmark
  + budget gate; spec §4: 100 KB part re-render).
- `internal/tui/AGENTS.md` — the V1 wrap/scroll pins must not break.

## Detail pass (protocol)

One writing-plans pass, one subagent, `thinking=high`, dispatched by the
root session strictly sequentially (root principle 7) — per the Slice
Detail Protocol in plan.md: it fills this file (after this section) with the
full 5-step TDD detail for each task in the binding table (failing test
code, implementation code, gate, pinned commit), reading the named upstream
files at that moment. It commits as
`docs: TUI parity plan — detail S1 tasks` on its own bead
(`bd create "detail S1 plan tasks" --parent=yolo-oae.2 --json`).

## S1 detail (deviations tail at detail time = 147; S1 entries start at 148)

### Detail-pass findings (read AT DETAIL TIME, 2026-08-26 — binding)

1. **glamour v2.0.1 API verified from module source** (extracted to
   `/tmp/opencode/glamour-inspect/pkg/charm.land/glamour/v2@v2.0.1/` for this
   pass; treat memory as stale):
   - `ansi.StyleConfig` (ansi/style.go:97-138): `Document`/`BlockQuote`/
     `Paragraph`/`List` (StyleBlock/StyleList), `Heading`–`H6`,
     `Text`/`Strikethrough`/`Emph`/`Strong`/`HorizontalRule` (StylePrimitive),
     `Item`/`Enumeration`/`Task` (StyleTask), `Link`/`LinkText`,
     `Image`/`ImageText`, `Code` (StyleBlock) / `CodeBlock` (StyleCodeBlock),
     `Table` (StyleTable).
   - `StylePrimitive` (ansi/style.go:38-59): `BlockPrefix`/`BlockSuffix`/
     `Prefix`/`Suffix string`; `Color`/`BackgroundColor *string`;
     `Underline`/`Bold`/`Upper`/`Lower`/`Title`/`Italic`/`CrossedOut`/`Faint`/
     `Conceal`/`Inverse`/`Blink *bool`; `Format string`.
   - `StyleCodeBlock` = StyleBlock + `Theme string` + `Chroma *Chroma`;
     `Chroma` (ansi/style.go:4-36) = 30 `StylePrimitive` fields +
     `Background` (Text, Error, Comment, CommentPreproc, Keyword,
     KeywordReserved, KeywordNamespace, KeywordType, Operator, Punctuation,
     Name, NameBuiltin, NameTag, NameAttribute, NameClass, NameConstant,
     NameDecorator, NameException, NameFunction, NameOther, Literal,
     LiteralNumber, LiteralDate, LiteralString, LiteralStringEscape,
     GenericDeleted, GenericEmph, GenericInserted, GenericStrong,
     GenericSubheading).
   - `glamour.NewTermRenderer(opts ...TermRendererOption)` +
     `WithStyles(ansi.StyleConfig)` + `WithWordWrap(int)` (glamour.go:68,146,
     173); `(*TermRenderer).Render(in string) (string, error)` (glamour.go:267).
     GFM (tables / task lists / strikethrough) is ON by default (goldmark
     `extension.GFM`).
2. **The global "charm" chroma slot (the S1.4 workaround).**
   ansi/codeblock.go:85-115: when `CodeBlock.Chroma != nil`, glamour registers
   that map under the HARDCODED name `"charm"` in chroma's package-level
   `styles.Registry` (`if !ok` — first-write-wins, never updated). Two
   renderers with different chroma maps (transcript full vs reasoning subtle)
   cannot coexist: whichever renders a code block first wins for both, and a
   mid-session theme switch (SIGUSR2) would keep the OLD theme's colors.
   **Workaround (S1.4):** `Renderer` keeps its own chroma map and calls
   `delete(styles.Registry, "charm")` before every `Render` — the next code
   block re-registers THIS renderer's map (glamour's own `styles.Register`
   call). Safe: bubbletea renders on a single goroutine; the teatest suites
   run sequentially. yolo therefore imports `github.com/alecthomas/chroma/v2/
   styles` directly (named in the S1.1 dep proposal).
3. **lipgloss v2.0.6 `parseHex` takes 6- or 3-digit hex ONLY** — 8-digit
   alpha is unparseable, so the upstream subtle syntax
   (`RGBA.fromInts(round(r*255), round(g*255), round(b*255),
   round(thinkingOpacity*255))`, theme/index.ts:560-584) is PRE-BLENDED over
   the theme background: `out = round(fg·α + bg·(1−α))` (half-up, per channel,
   α = `ThinkingOpacity`). opencode.dark expectations (α=0.6, bg #0a0a0a):
   syntaxComment #808080→#515151, syntaxKeyword #9d7cd8→#624e86,
   syntaxString #7fd88f→#50865a, syntaxNumber #f5a742→#97682c,
   syntaxOperator #56b6c2→#387178. (If the S1 pty diff shows the upstream
   tokens at FULL alpha — terminals that ignore SGR alpha — flip
   `SubtleChroma` to the plain `Chroma` map and log a deviation in the gate
   commit.)
4. **chroma category hierarchy** (chroma v2.14.0 `tokentype_enumer.go`):
   `Name=2000`, `NameVariable=2019 → Parent()=Name → 0`;
   `LiteralString=3100`, `LiteralStringEscape=3109 → 3100`. glamour registers
   only its fixed field set (codeblock.go:89-114) — **no `NameVariable`** —
   so variables fall back to `Chroma.Name`. Hence `Name` maps to
   `syntaxVariable` (the upstream `"variable"` scope), covering
   NameVariable/NameProperty/NameEntity; explicitly registered subtypes
   (NameFunction/NameClass/NameConstant/NameBuiltin/NameAttribute) override.
5. **No `ThemeJson` model change.** `ThemeJson.Theme` is `map[string]any`;
   the 14 flat `markdown*` + 9 `syntax*` keys flow through `ResolveTheme`
   into `Resolved.Colors` unchanged (the S0.2 resolver maps every `j.Theme`
   key; the S0.2 goldens already carry all 23 — hexes below). The S1.2 test
   asserts all 33 embedded themes × both modes carry the full set.
6. **Upstream `<markdown>` element quirks** (opentui core 0.4.5,
   `/tmp/opencode/.opentui-core/package/index.node.js`,
   `class MarkdownRenderable` @ 9307):
   - Task-list CHECKBOXES ARE HIDDEN: `createListItemRenderable` skips
     `checkbox` tokens (index.node.js ~9423) — a task item renders as a plain
     bullet. glamour mapping: `Task.Ticked`/`Task.Unticked = "• "` (S1.5).
   - HR = a full-width `height:1, border:["top"]` box, `borderColor` =
     conceal/fg (index.node.js ~9460). glamour mapping:
     `HorizontalRule.Format = "\n" + strings.Repeat("─", width) + "\n"`
     (S1.2; the border-char parity is pty-diff arbitrable).
   - Tables = `TextTable style:"grid"` box-drawn borders → glamour default
     lipgloss `NormalBorder`; S1 pty diff arbitrates (spec §4: gaps become
     per-element `StyleConfig` overrides or a logged custom renderer).
7. **Upstream transcript semantics re-read fresh** (index.tsx @ v1.18.18):
   `TextPart` 1699-1716 (`paddingLeft=3`, `fg=markdownText`, FULL syntax via
   `useTheme().syntax`, `bg=theme.background`); `ReasoningPart` 1584-1651 +
   `ReasoningHeader` 1653-1690 (SPINNER row while `time.end` undefined —
   `"Thinking: <title>"` / `"Thinking"` in warning@thinkingOpacity; done row
   `"+/- Thought: <title> · <duration>"` in warning, warning@thinkingOpacity
   when open; body = markdown `<code filetype="markdown" fg=textMuted
   syntaxStyle=subtle>` of `reasoningSummary(content).body` at
   paddingLeft 2); `reasoningSummary` (context/thinking.ts:12) = leading
   `**title**` regex; `Locale.duration` (util/locale.ts:39) = `<1s: Nms`,
   `<1m: X.Xs`, `<1h: Nm Ns`, `<24h: Nh Nm`, else `Nd Nh`; `InlineTool`
   1844-1900 + `InlineToolRow` 1922-2000 (2-col icon —
   `INLINE_TOOL_ICON_WIDTH=2`, index.tsx:1587 — pending row `~ <pending>` at
   paddingLeft 3 fg=text, completed row `<icon> <children>` fg=textMuted,
   failed row `<icon> <failure ?? children>` fg=error, expanded error box
   paddingLeft 2 fg=error); the assistant error box 1534-1548
   (`border=["left"]`, `borderColor=error`, `backgroundColor=backgroundPanel`,
   paddingTop/bottom 1, paddingLeft 2, marginTop 1, text fg=textMuted —
   rendered ONLY when `error.name !== "MessageAbortedError"`); tool icons +
   pending strings: bash `"$"`/"Writing command..." (2105), write `"←"`/
   "Preparing write..." (2138), glob `"✱"`/"Finding files..." (2153), read
   `"→"`/"Reading file..." (2177, `spinner={isRunning()}`), grep `"✱"`/
   "Searching content..." (2201), edit `"←"`/"Preparing edit..." (2448),
   todowrite `"⚙"`/"Updating todos..." + failure "Todo update failed" (2545);
   the toast (ui/toast.tsx:22-52): `border=["left","right"]`,
   `borderColor=theme[variant]`, `backgroundColor=backgroundPanel`,
   paddingLeft/Right 2, paddingTop/Bottom 1, message fg=text.

### Task S1.1: Dep proposal glamour v2.0.1 (approval gate) → `go get` + smoke render (`yolo-oae.2.1`)

**Files:**
- Modify: `go.mod`, `go.sum` (Step 2, AFTER approval)
- Modify: `AGENTS.md` (root — allowlist line, Step 4)
- Modify: `docs/superpowers/PROGRESS.md` (one-line fact, Step 4)
- No repo test file: the smoke render is a throwaway under /tmp (the
  persistent coverage is the S1.2 fixture tests).

**Interfaces:**
- Consumes: the root dependency policy (AGENTS.md "Project"); the S0 module
  graph (go.mod direct: bubbletea v2.0.9, lipgloss v2.0.6, bubbles v2.2.1,
  go-udiff v0.4.1, x/term v0.2.2, jsonc v0.3.3, sqlite v1.57.0).
- Produces: `charm.land/glamour/v2 v2.0.1` as a direct require (+ its
  transitive closure in go.sum) — the S1.2 factory's dependency target.

**Evidence (Step 1 — the agent MUST treat its own memory as stale; run live,
fill the checklist, paste it into the bead):**

```sh
# 1) repo maintenance + license (charm.land vanity -> charmbracelet/glamour)
curl -s https://api.github.com/repos/charmbracelet/glamour \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["license"]["spdx_id"], d["pushed_at"], d["open_issues_count"], d["default_branch"])'
# 2) the v2.0.1 tag object (date via the commit)
curl -s https://api.github.com/repos/charmbracelet/glamour/git/refs/tags/v2.0.1 \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["object"]["sha"])'
# 3) live version list
go list -m -versions charm.land/glamour/v2
# 4) the module's own requires, WITHOUT touching the repo (scratch module)
mkdir -p /tmp/opencode/glamour-evidence && cd /tmp/opencode/glamour-evidence
go mod init tmp 2>/dev/null || true
go mod download charm.land/glamour/v2@v2.0.1
cat "$(go env GOMODCACHE)/cache/download/charm.land/glamour/v2/@v/v2.0.1.mod"
# 5) pure-Go check over the module source (after step 4's download)
grep -rl 'import "C"\|#include' "$(go env GOMODCACHE)/charm.land/glamour/v2@v2.0.1/" --include='*.go' || echo "no cgo"
```

Checklist (fill with the live values):
- Maintenance: SPDX id (expect MIT), `pushed_at`, `v2.0.1` tag + commit date.
- Versions: the `go list -m -versions` output; `v2.0.1` is the exact pin.
- Pure Go / no cgo: step 5 clean for glamour; AFTER the Step-2 tidy, repeat
  the same grep over every NEW module dir in the module cache (expect clean).
- License: MIT — the same org as the allowlisted `charm.land/*` stack.
- Transitive surface — EXPECTED new modules (10; the live count is
  authoritative and is verified in Step 2 by diffing `go list -m all | wc -l`
  before/after tidy; the spec's "≈8" was an estimate):
  `github.com/alecthomas/chroma/v2 v2.14.0`, `github.com/yuin/goldmark v1.7.8`,
  `github.com/yuin/goldmark-emoji v1.0.5`, `github.com/microcosm-cc/bluemonday
  v1.0.27`, `github.com/aymerick/douceur v0.2.0`, `github.com/gorilla/css
  v1.0.1`, `github.com/dlclark/regexp2 v1.11.0`, `golang.org/x/text v0.24.0`,
  `golang.org/x/net v0.39.0`, `github.com/charmbracelet/x/exp/slice`
  (exact version live). Already in the graph (MVS keeps the higher): lipgloss
  v2.0.6, x/ansi v0.11.8, x/term v0.2.2, go-udiff v0.4.1 (allowlisted),
  ultraviolet, colorprofile, x/exp/golden, go-colorful v1.4.1, go-runewidth
  v0.0.27, uniseg, cancelreader, terminfo, displaywidth, uax29, x/sync,
  x/sys.
- Direct imports yolo will add (name them in the bead):
  `charm.land/glamour/v2` (+ `/ansi`), `github.com/alecthomas/chroma/v2/
  styles` (the S1.4 global "charm" slot workaround).
- Why stdlib / hand-rolling is inadequate: the stdlib has no markdown parser
  or lexer; hand-rolling GFM (tables/task lists/strikethrough) plus lexers
  for 300+ languages is weeks of work for a solved problem; upstream itself
  uses a dedicated markdown+syntax element (the opentui `<markdown>`);
  glamour v2 is built on the allowlisted lipgloss v2 (glamour v1 was
  lipgloss-v1-based — the v2 line is the one that composes with this stack).
- go directive: glamour's go.mod declares `go 1.25.8` → `go mod tidy` will
  bump the module line `go 1.25.0` → `go 1.25.8` (still ≥ 1.25, ≤ the
  installed 1.26.7 toolchain — note it in the gate report).

- [ ] **Step 1: Collect the live evidence, file the dep-proposal bead, then STOP for approval**

Run the evidence commands above. Then:

```sh
bd create "dep proposal: add charm.land/glamour/v2 v2.0.1 (transcript rendering)" \
  --parent=yolo-oae.2 -t task -p 1 \
  --description="<paste the filled evidence checklist verbatim>" --json
```

**STOP** — report the bead id + the evidence summary; wait for the user's
explicit approval. (On approval: continue to Step 2. On rejection: STOP and
report — the task is blocked, the slice gate needs a design change the user
must call.)

- [ ] **Step 2: (after approval) land the dep**

```sh
go list -m all | wc -l                # record the baseline module count
go get charm.land/glamour/v2@v2.0.1
go mod tidy
go list -m all | wc -l                # delta = the exact new-module count (report it)
grep glamour go.mod                   # expect: charm.land/glamour/v2 v2.0.1 (no // indirect)
grep -rl 'import "C"\|#include' "$(go env GOMODCACHE)"/github.com/alecthomas/chroma/ \
  "$(go env GOMODCACHE)"/github.com/yuin/ --include='*.go' 2>/dev/null || echo "no cgo in new tree"
go build ./...
```

- [ ] **Step 3: Smoke render (throwaway, /tmp — never hits the network)**

```sh
mkdir -p /tmp/opencode/glamour-smoke && cd /tmp/opencode/glamour-smoke
cat > main.go <<'EOF'
package main

import (
	"fmt"
	"os"
	"strings"

	"charm.land/glamour/v2"
)

func main() {
	tr, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(60))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, err := tr.Render("# Head\n\n**bold** ~~strike~~ and a table:\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n- [x] done\n- [ ] todo\n\n```go\nvar x = 1\n```\n")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// glamour renders the table as a bordered grid (no raw pipes), so pin
	// the SGR escapes, the bold run, and a table column separator.
	for _, want := range []string{"\x1b[", "bold", "\u2502"} {
		if !strings.Contains(out, want) {
			fmt.Fprintln(os.Stderr, "missing:", want)
			os.Exit(1)
		}
	}
	fmt.Println("smoke ok:", len(out), "bytes")
}
EOF
go mod init smoke 2>/dev/null || true
go mod edit -require charm.land/glamour/v2@v2.0.1 && go mod tidy
go run .
```

Expect `smoke ok: N bytes` (markdown parses, SGR escapes emit, the GFM table
renders a grid).

- [ ] **Step 4: Gate + allowlist + PROGRESS fact**

```sh
go vet ./... && go test ./... && gofmt -l .
```

Root `AGENTS.md`, allowlist paragraph — add:
`charm.land/glamour/v2` v2.0.1 (proposal approved <date> — GFM markdown +
chroma syntax highlighting for the transcript; direct imports: `glamour`,
`glamour/ansi`, `chroma/v2/styles` for the global "charm" slot workaround).
`docs/superpowers/PROGRESS.md`, Key verified facts — one line:
`glamour v2.0.1 landed (S1.1, N new modules); its custom chroma map registers
under the global "charm" slot (first-write-wins) — yolo deletes the slot
before every Render (Renderer.Render, internal/tui/theme/syntax.go) so the
transcript (full) and reasoning (subtle) renderers + SIGUSR2 theme switches
never cross-color.`

- [ ] **Step 5: Commit + close the bead**

```sh
git add go.mod go.sum AGENTS.md docs/superpowers/PROGRESS.md
git commit -m "deps: add glamour v2.0.1 (transcript rendering)"
bd close yolo-oae.2.1 --reason "glamour v2.0.1 approved + landed; smoke render ok; N new modules" --json
```

**STOP** (per-task cadence): report the gate result, the commit, `git status`.

### Task S1.2: `TermRenderer` from resolved theme (`StyleConfig` + chroma token map) + fixture unit tests (`yolo-oae.2.2`)

**Files:**
- Create: `internal/tui/theme/syntax.go`
- Create: `internal/tui/theme/syntax_test.go`

**Interfaces:**
- Consumes: S0.1 `AllThemes`/`ThemeJson`; S0.2 `ResolveTheme` →
  `Resolved{Colors map[string]Rgba, ThinkingOpacity float64}`; S0.3
  `Theme{R Resolved, Name, Mode string}`; S1.1 glamour v2.0.1
  (`glamour.NewTermRenderer`, `WithStyles`, `WithWordWrap`,
  `(*TermRenderer).Render`, `ansi.StyleConfig`).
- Concludes the s1-transcript.md model question: **no `ThemeJson` change**
  (finding 5 above — the flat keys already resolve into
  `Resolved.Colors`; the S1.2 test asserts the 33×2 token matrix).
- Produces (binding for S1.3–S1.6):
  - `Theme.Zero() bool` — `Name == ""` (the plain-path sentinel; the S0.7
    nil-engine contract extended to the markdown path).
  - `Theme.StyleConfig(base string, width int) ansi.StyleConfig` — `base` =
    the base text TOKEN NAME (`"markdownText"` text parts / `"textMuted"`
    reasoning); `width` = the word-wrap width (also the HR line length;
    <4 → 8-dash fallback). Element styles only — the chroma map attaches in
    S1.4.
  - `NewTranscriptRenderer(th Theme, width int) (*Renderer, error)`;
    `type Renderer struct{ tr *glamour.TermRenderer }`;
    `(*Renderer).Render(md string) (string, error)` (the chroma field +
    the "charm" slot delete land in S1.4).

**Upstream parity notes (binding):**
- Element style map (markdown* tokens; opencode.dark hexes from the S0.2
  goldens):
  | ansi field | token (opencode.dark) | attrs |
  |---|---|---|
  | `Document` | markdownText (#eeeeee) | BlockPrefix/BlockSuffix `"\n"` |
  | `Text` | the `base` token | — |
  | `Heading` (cascades to H1–H6 via glamour's `cascadeStyles`) | markdownHeading (#9d7cd8) | Bold |
  | `BlockQuote` | markdownBlockQuote (#e5c07b) | — |
  | `Emph` | markdownEmph (#e5c07b) | Italic |
  | `Strong` | markdownStrong (#f5a742) | Bold |
  | `HorizontalRule` | markdownHorizontalRule (#808080) | `Format "\n" + Repeat("─",width) + "\n"` (upstream: full-width top-border box, finding 6) |
  | `Item` | markdownListItem (#fab283) | BlockPrefix `"• "` |
  | `Enumeration` | markdownListEnumeration (#56b6c2) | BlockPrefix `". "` |
  | `Link` | markdownLink (#fab283) | Underline |
  | `LinkText` | markdownLinkText (#56b6c2) | — |
  | `Image` / `ImageText` | markdownImage (#fab283) / markdownImageText (#56b6c2) | `Format "Image: {{.text}} →"` |
  | `Code` (inline span) | markdownCode (#7fd88f) | — |
  | `CodeBlock` | markdownCodeBlock (#eeeeee) | `Chroma: nil` until S1.4 |
  `Strikethrough`/`Task`/`Table` land in S1.5 (the GFM trio). Absent token
  (zero Theme, custom theme missing a key) → nil `*string` → glamour's own
  defaults (never a panic).
- **Deviation 148** (render/low, lands in the S1.3 commit with the wiring —
  the spec §4 pty diff arbitrates): upstream `<markdown fg= bg=>` passes
  `bg={theme.background}` (index.tsx:1705) — a terminal background hint with
  no glamour equivalent (no SGR block background without per-line
  backgrounds; the yolo frame is transparent by S0 design). yolo omits the
  Document BackgroundColor.

- [ ] **Step 1: Write the failing test**

Create `internal/tui/theme/syntax_test.go`:

```go
package theme

import (
	"strings"
	"testing"
)

// markdownTokens + syntaxTokens are the flat upstream theme keys
// (theme/index.ts; every one of the 33 embedded assets carries them).
var (
	markdownTokens = []string{
		"markdownText", "markdownHeading", "markdownCode", "markdownCodeBlock",
		"markdownBlockQuote", "markdownEmph", "markdownStrong",
		"markdownHorizontalRule", "markdownListItem", "markdownListEnumeration",
		"markdownLink", "markdownLinkText", "markdownImage", "markdownImageText",
	}
	syntaxTokens = []string{
		"syntaxComment", "syntaxKeyword", "syntaxFunction", "syntaxVariable",
		"syntaxString", "syntaxNumber", "syntaxType", "syntaxOperator",
		"syntaxPunctuation",
	}
)

func resolveOpencodeDark(t *testing.T) Theme {
	t.Helper()
	all, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := ResolveTheme(all["opencode"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	return Theme{R: r, Name: "opencode", Mode: "dark"}
}

// TestAllThemesHaveMarkdownSyntaxTokens pins the token matrix: every
// embedded theme × both modes resolves all 23 markdown*/syntax* tokens
// (finding 5: no ThemeJson model change needed).
func TestAllThemesHaveMarkdownSyntaxTokens(t *testing.T) {
	all, err := AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	for name, tj := range all {
		for _, mode := range []string{"dark", "light"} {
			r, err := ResolveTheme(tj, mode)
			if err != nil {
				t.Fatalf("%s/%s: %v", name, mode, err)
			}
			for _, tok := range append(append([]string{}, markdownTokens...), syntaxTokens...) {
				if _, ok := r.Color(tok); !ok {
					t.Errorf("%s/%s: missing token %s", name, mode, tok)
				}
			}
		}
	}
}

// TestStyleConfigMapping pins the markdown* → ansi.StyleConfig field map
// (the opencode.dark goldens; the SGR quantization is pinned by the S1.3
// teatest golden, the 24-bit hex here).
func TestStyleConfigMapping(t *testing.T) {
	cfg := resolveOpencodeDark(t).StyleConfig("markdownText", 77)
	check := func(name string, got *string, want string) {
		t.Helper()
		if got == nil || *got != want {
			t.Errorf("%s = %v, want %s", name, got, want)
		}
	}
	check("Document.Color", cfg.Document.Color, "#eeeeee")
	if cfg.Document.BlockPrefix != "\n" || cfg.Document.BlockSuffix != "\n" {
		t.Errorf("Document block prefix/suffix = %q/%q, want \\n/\\n",
			cfg.Document.BlockPrefix, cfg.Document.BlockSuffix)
	}
	check("Text.Color", cfg.Text.Color, "#eeeeee")
	check("Heading.Color", cfg.Heading.Color, "#9d7cd8")
	if cfg.Heading.Bold == nil || !*cfg.Heading.Bold {
		t.Error("Heading.Bold = false/nil, want true")
	}
	check("BlockQuote.Color", cfg.BlockQuote.Color, "#e5c07b")
	check("Emph.Color", cfg.Emph.Color, "#e5c07b")
	if cfg.Emph.Italic == nil || !*cfg.Emph.Italic {
		t.Error("Emph.Italic = false/nil, want true")
	}
	check("Strong.Color", cfg.Strong.Color, "#f5a742")
	if cfg.Strong.Bold == nil || !*cfg.Strong.Bold {
		t.Error("Strong.Bold = false/nil, want true")
	}
	check("HorizontalRule.Color", cfg.HorizontalRule.Color, "#808080")
	if want := "\n" + strings.Repeat("─", 77) + "\n"; cfg.HorizontalRule.Format != want {
		t.Errorf("HorizontalRule.Format = %q (len %d), want a 77-dash line",
			cfg.HorizontalRule.Format, len(cfg.HorizontalRule.Format))
	}
	check("Item.Color", cfg.Item.Color, "#fab283")
	if cfg.Item.BlockPrefix != "• " {
		t.Errorf("Item.BlockPrefix = %q, want '• '", cfg.Item.BlockPrefix)
	}
	check("Enumeration.Color", cfg.Enumeration.Color, "#56b6c2")
	if cfg.Enumeration.BlockPrefix != ". " {
		t.Errorf("Enumeration.BlockPrefix = %q, want '. '", cfg.Enumeration.BlockPrefix)
	}
	check("Link.Color", cfg.Link.Color, "#fab283")
	if cfg.Link.Underline == nil || !*cfg.Link.Underline {
		t.Error("Link.Underline = false/nil, want true")
	}
	check("LinkText.Color", cfg.LinkText.Color, "#56b6c2")
	check("Image.Color", cfg.Image.Color, "#fab283")
	check("ImageText.Color", cfg.ImageText.Color, "#56b6c2")
	check("Code.Color", cfg.Code.Color, "#7fd88f")
	check("CodeBlock.Color", cfg.CodeBlock.Color, "#eeeeee")
	if cfg.CodeBlock.Chroma != nil {
		t.Error("CodeBlock.Chroma set before S1.4")
	}
}

// TestStyleConfigReasoningBase pins the reasoning base token (S1.6 consumes
// it): the Text style takes the base TOKEN NAME, not a hard-coded color.
func TestStyleConfigReasoningBase(t *testing.T) {
	cfg := resolveOpencodeDark(t).StyleConfig("textMuted", 77)
	if cfg.Text.Color == nil || *cfg.Text.Color != "#808080" {
		t.Errorf("reasoning base Text.Color = %v, want #808080", cfg.Text.Color)
	}
}

// TestZeroThemeStyleConfigIsNilColors pins the S0.7 zero-Theme contract on
// the markdown path: absent tokens → nil *string → glamour defaults.
func TestZeroThemeStyleConfigIsNilColors(t *testing.T) {
	var th Theme
	cfg := th.StyleConfig("markdownText", 77)
	if cfg.Text.Color != nil || cfg.Heading.Color != nil {
		t.Error("zero Theme must yield nil *string colors")
	}
}

// TestTranscriptRendererRenders pins the factory: a themed renderer emits
// SGR, a zero-Theme renderer degrades to plain output (no SGR). The exact
// 38;5;N parameters are pinned by the S1.3 teatest golden (xterm-256color).
func TestTranscriptRendererRenders(t *testing.T) {
	r, err := NewTranscriptRenderer(resolveOpencodeDark(t), 77)
	if err != nil {
		t.Fatalf("NewTranscriptRenderer: %v", err)
	}
	out, err := r.Render("hello **world**")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "world") {
		t.Errorf("output missing text: %q", out)
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("themed renderer emitted no SGR escapes")
	}
	zr, err := NewTranscriptRenderer(Theme{}, 77)
	if err != nil {
		t.Fatalf("NewTranscriptRenderer(zero): %v", err)
	}
	zout, err := zr.Render("hello **world**")
	if err != nil {
		t.Fatalf("Render(zero): %v", err)
	}
	if strings.Contains(zout, "\x1b[") {
		t.Errorf("zero-Theme renderer emitted SGR: %q", zout)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```sh
go test ./internal/tui/theme/ -run 'TestStyleConfig|TestTranscriptRenderer|TestAllThemesHaveMarkdownSyntax|TestZeroThemeStyleConfig' -v
```

Expect FAIL — `th.StyleConfig` / `NewTranscriptRenderer` undefined.

- [ ] **Step 3: Write the minimal implementation**

Create `internal/tui/theme/syntax.go`:

```go
// syntax.go — the glamour element styles + TermRenderer factory for the
// transcript. S1.2: the markdown* element styles; S1.4: the chroma token
// map (per-language highlighting) + the global "charm" slot workaround;
// S1.5: the GFM trio (Strikethrough/Task/Table); S1.6: the reasoning
// variant (textMuted base + the pre-blended chroma).

package theme

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
)

// Zero reports whether t is the zero Theme (nil-engine runs, S0.7): the
// transcript render path degrades to the plain wrap.
func (t Theme) Zero() bool { return t.Name == "" }

// hex6 is the 6-digit RGB hex of c. lipgloss v2 parseHex takes #rrggbb or
// #rgb only — 8-digit alpha is unparseable, so subtle (pre-blended) colors
// always land as 6-digit hex (finding 3).
func hex6(c Rgba) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// md returns the color string for a token (nil when absent: glamour falls
// back to its own defaults for unset styles).
func (t Theme) md(name string) *string {
	c, ok := t.R.Color(name)
	if !ok {
		return nil
	}
	s := hex6(c)
	return &s
}

func boolPtr(b bool) *bool { return &b }

// StyleConfig builds the glamour element styles from the markdown* tokens.
// base is the base text token name ("markdownText" for text parts,
// "textMuted" for reasoning); width is the word-wrap width (also the HR
// line length; <4 → the 8-dash fallback). The chroma map is attached in
// S1.4 (CodeBlock.Chroma stays nil until then).
func (t Theme) StyleConfig(base string, width int) ansi.StyleConfig {
	cfg := ansi.StyleConfig{}
	cfg.Document = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
		Color:       t.md("markdownText"),
		BlockPrefix: "\n",
		BlockSuffix: "\n",
	}}
	cfg.Text = ansi.StylePrimitive{Color: t.md(base)}
	cfg.Heading = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
		Color: t.md("markdownHeading"),
		Bold:  boolPtr(true),
	}}
	cfg.BlockQuote = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
		Color: t.md("markdownBlockQuote"),
	}}
	cfg.Emph = ansi.StylePrimitive{Color: t.md("markdownEmph"), Italic: boolPtr(true)}
	cfg.Strong = ansi.StylePrimitive{Color: t.md("markdownStrong"), Bold: boolPtr(true)}
	cfg.HorizontalRule = ansi.StylePrimitive{
		Color:  t.md("markdownHorizontalRule"),
		Format: "\n" + strings.Repeat("─", hrWidth(width)) + "\n",
	}
	cfg.Item = ansi.StylePrimitive{Color: t.md("markdownListItem"), BlockPrefix: "• "}
	cfg.Enumeration = ansi.StylePrimitive{
		Color:       t.md("markdownListEnumeration"),
		BlockPrefix: ". ",
	}
	cfg.Link = ansi.StylePrimitive{Color: t.md("markdownLink"), Underline: boolPtr(true)}
	cfg.LinkText = ansi.StylePrimitive{Color: t.md("markdownLinkText")}
	cfg.Image = ansi.StylePrimitive{
		Color:  t.md("markdownImage"),
		Format: "Image: {{.text}} \u2192",
	}
	cfg.ImageText = ansi.StylePrimitive{Color: t.md("markdownImageText")}
	cfg.Code = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: t.md("markdownCode")}}
	cfg.CodeBlock = ansi.StyleCodeBlock{StyleBlock: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{Color: t.md("markdownCodeBlock")},
	}}
	return cfg
}

// hrWidth is the HorizontalRule line length (the word-wrap width; a
// sub-4-column viewport gets the 8-dash fallback — the upstream element
// renders a full-width top-border box, finding 6).
func hrWidth(width int) int {
	if width < 4 {
		return 8
	}
	return width
}

// Renderer is a glamour TermRenderer bound to one theme + width. The TUI
// renders single-threaded (bubbletea View), so the app builds one per
// renderMessages call — no cache (the construct cost is ~20–50µs; the S1.9
// budget covers the whole re-render).
type Renderer struct {
	tr *glamour.TermRenderer
}

// NewTranscriptRenderer builds the text-part renderer: the markdownText
// base, word-wrap at width (the caller passes w-3 — the post-indent width;
// <=0 disables wrapping).
func NewTranscriptRenderer(th Theme, width int) (*Renderer, error) {
	opts := []glamour.TermRendererOption{
		glamour.WithStyles(th.StyleConfig("markdownText", width)),
	}
	if width > 0 {
		opts = append(opts, glamour.WithWordWrap(width))
	}
	tr, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return nil, err
	}
	return &Renderer{tr: tr}, nil
}

// Render renders md to an ANSI string. SGR profile (verified, sgrprobe):
// in a plain unit context glamour's plain text is 24-bit (38;2;R;G;B) while
// the chroma code-block path is 256-color (38;5;N); under teatest (the
// TUI's program environment) glamour emits 256-color for both. Teatest
// goldens therefore pin 38;5;N; direct renderer unit tests pin whichever
// profile the path uses.
func (r *Renderer) Render(md string) (string, error) { return r.tr.Render(md) }
```

- [ ] **Step 4: Run to verify it passes, then gate**

```sh
go test ./internal/tui/theme/ -v
go vet ./... && go test ./... && gofmt -l .
```

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/theme/syntax.go internal/tui/theme/syntax_test.go
git commit -m "feat: glamour TermRenderer from resolved theme tokens"
bd close yolo-oae.2.2 --reason "StyleConfig (14 markdown* tokens) + Renderer factory; 33x2 token matrix pinned" --json
```

**STOP**: report the gate result, the commit, `git status`.

### Task S1.3: Wire renderer into text parts (replaces plain wrap) + teatest goldens (`yolo-oae.2.3`)

**Files:**
- Modify: `internal/tui/session.go` — `renderMessages` builds the transcript
  renderer (nil for a zero Theme); the text `case` in `renderAssistant`
  renders through it (3-column indent).
- Create: `internal/tui/session_markdown_test.go` — the teatest SGR golden.
- Modify: `internal/tui/AGENTS.md` — re-baseline the V1 transcript wrap pin
  (same commit; DOX pass).
- Modify: `docs/superpowers/DEVIATIONS.md` — entry 148 (same commit; root
  principle 2).

**Interfaces:**
- Consumes: S1.2 `NewTranscriptRenderer`/`(*Renderer).Render`/
  `Theme.Zero()`; the S0.10 teatest convention (TTY_FORCE=1 +
  TERM=xterm-256color, one merged `WaitFor`, strip-ANSI + SGR tokens).
- Produces: text parts render as 3-column-indented themed markdown; the
  zero-Theme plain path is untouched (the existing `TestRenderMessages` +
  `TestRenderMessagesWrapsLongLines` goldens stay green — asserted in
  Step 4).

**Upstream parity notes (binding):**
- The upstream TextPart (index.tsx:1699-1716): `<box paddingLeft={3}
  marginTop={1}><markdown syntaxStyle={syntax()} internalBlockMode=
  "top-level" content={text.trim()} tableOptions={{style:"grid"}} conceal
  fg={theme.markdownText} bg={theme.background}>`. yolo mapping:
  - 3 spaces indent EVERY rendered line (the renderer already word-wraps at
    w-3 via `WithWordWrap` — the styled output NEVER reaches `wrapLine`,
    the V1 "styled lines wrap before styling" contract survives, only the
    "plain text ONLY" clause changes);
  - the Document `BlockPrefix/BlockSuffix "\n"` (S1.2) trims to nothing:
    `strings.Trim(out, "\n")`;
  - `bg=` omitted — **deviation 148** (render/low);
  - `conceal` (upstream user toggle) has no yolo analog — out of scope, no
    deviation (S4 owns the keymap; if a conceal binding is added, wire it);
  - `marginTop=1` ≈ yolo's existing inter-part `"\n"` (writeRaw).
- SGR tokens under xterm-256color (x/ansi v0.11.8 `Convert256`, the S0.10
  derivation): markdownText #eeeeee → **255**; markdownStrong #f5a742 →
  **215** (bold run); markdownCodeBlock #eeeeee → **255** (code blocks are
  NOT chroma-highlighted yet — S1.4). Substring form (the pen-diff merges
  changed params into ONE CSI, inner order not pinned — the S0.10
  precedent).
- **Re-baseline:** NO existing golden changes in this task (zero-Theme
  tests stay plain; the S0.10 chrome tokens are untouched — its final text
  part "all done" now renders through the renderer, but a plain one-word
  line carries only markdownText=255, which that test does not pin).

- [ ] **Step 1: Write the failing test**

Create `internal/tui/session_markdown_test.go`:

```go
package tui

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// TestMarkdownTextPartSGR pins the S1.3 transcript rendering under the
// pinned TTY_FORCE=1 + TERM=xterm-256color env: the text part renders
// through the glamour renderer — the markdown markers are stripped, the
// base text takes markdownText (38;5;255), the bold run takes
// markdownStrong (38;5;215), and the rendered lines carry the upstream
// 3-column indent (index.tsx:1701).
func TestMarkdownTextPartSGR(t *testing.T) {
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "text", Text: "Here is **bold** text\n\nsome more\n"},
		}},
	)
	ts := testutil.BootWithDriverConfig(t, drv, &protocol.Config{})
	dir := t.TempDir()
	e, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}

	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "hi")
	tm.Send(press(tea.KeyEnter))

	// ONE merged terminal state (suite convention): the markers stripped,
	// the 3-column indent, both SGR tokens, and the help line.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		if !strings.Contains(s, "   Here is bold text") ||
			!strings.Contains(s, "   some more") ||
			!strings.Contains(s, "esc abort/back") {
			return false
		}
		if strings.Contains(s, "**") {
			t.Error("markdown markers not stripped")
			return false
		}
		return bytes.Contains(b, []byte("38;5;255")) &&
			bytes.Contains(b, []byte("38;5;215"))
	}, teatest.WithDuration(10*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

- [ ] **Step 2: Run to verify it fails**

```sh
go test ./internal/tui/ -run TestMarkdownTextPartSGR -v
```

Expect FAIL — the text part still renders plain (no `38;5;215`, no
3-column indent).

- [ ] **Step 3: Write the minimal implementation**

`internal/tui/session.go` — two edits.

(1) `renderMessages` builds the renderer once per transcript render (nil =
the plain path; a build error degrades the same way — never block a frame):

```go
func renderMessages(st *store.State, expanded map[string]bool, w int, th theme.Theme) string {
	var r *theme.Renderer
	if !th.Zero() {
		if built, err := theme.NewTranscriptRenderer(th, w-3); err == nil {
			r = built
		}
	}
	blocks := make([]string, 0, len(st.Messages))
	for _, m := range st.Messages {
		if m.Info.Role == "user" {
			blocks = append(blocks, renderUser(m, w))
		} else {
			blocks = append(blocks, renderAssistant(m, expanded, w, th, r))
		}
	}
	// ... (the rest of renderMessages is unchanged)
}
```

(2) `renderAssistant` gains the renderer arg and the text case:

```go
func renderAssistant(m protocol.MessageWithParts, expanded map[string]bool, w int, th theme.Theme, r *theme.Renderer) string {
	// ... (writeRaw / writePlain / writeStyled closures unchanged)
	for _, p := range m.Parts {
		switch p.Type {
		case "text":
			if p.Text == "" {
				continue
			}
			if r == nil {
				for _, l := range strings.Split(p.Text, "\n") {
					writePlain(l)
				}
				continue
			}
			// The upstream TextPart is a 3-column-indented markdown block
			// (index.tsx:1700-1707). The renderer word-wraps at w-3
			// (WithWordWrap), so the indented lines already fit w — the
			// styled output never reaches wrapLine.
			if out, err := r.Render(p.Text); err == nil {
				for _, l := range strings.Split(strings.Trim(out, "\n"), "\n") {
					writeRaw("   " + l)
				}
			} else {
				for _, l := range strings.Split(p.Text, "\n") {
					writePlain(l)
				}
			}
		// ... (reasoning / tool / error cases unchanged in this task)
	}
	// ...
}
```

`internal/tui/AGENTS.md` — re-baseline the V1 pin (Local Contracts):
"Transcript word-wrap: `renderMessages` wraps every PLAIN transcript line at
the viewport width via `wrapLine` …; assistant TEXT parts render through the
glamour renderer (S1.3) — the renderer's `WithWordWrap(w-3)` is their wrap
strategy, every rendered line carries the upstream 3-column indent, and the
styled output never reaches `wrapLine`. The viewport's hard clip remains the
backstop."

`docs/superpowers/DEVIATIONS.md` — append:

```
148. [render/low] S1.3: the upstream TextPart passes bg=theme.background to
    the <markdown> element (index.tsx:1705) — a terminal background hint
    with no glamour equivalent (no SGR block background without per-line
    backgrounds; the yolo frame is transparent by S0 design). yolo omits
    the Document BackgroundColor; the S1 pty diff arbitrates (spec §4).
```

- [ ] **Step 4: Run to verify it passes, then gate**

```sh
go test ./internal/tui/ -run 'TestMarkdownTextPartSGR|TestRenderMessages' -v
go vet ./... && go test ./... && gofmt -l .
```

(The zero-Theme `TestRenderMessages` + `TestRenderMessagesWrapsLongLines`
must stay green — the plain path is untouched.)

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/session.go internal/tui/session_markdown_test.go internal/tui/AGENTS.md docs/superpowers/DEVIATIONS.md
git commit -m "feat: render text parts through the glamour renderer"
bd close yolo-oae.2.3 --reason "text parts themed markdown (3-col indent, wrap w-3); deviation 148 logged; AGENTS.md re-baselined" --json
```

**STOP**: report the gate result, the commit, `git status`.

### Task S1.4: Syntax-highlighted code blocks (per-language chroma styles) + tests (`yolo-oae.2.4`)

**Files:**
- Modify: `internal/tui/theme/syntax.go` — `Theme.Chroma()`/
  `Theme.SubtleChroma()`; the `Renderer.chroma` field + the "charm" slot
  delete in `Render`; `NewTranscriptRenderer` attaches the map.
- Modify: `internal/tui/theme/syntax_test.go` — the chroma map fixtures +
  the highlight render tests.

**Interfaces:**
- Consumes: S1.2 `StyleConfig`/`Renderer`; S1.1 `chroma/v2/styles` (the
  global registry — finding 2); upstream `getSyntaxRules` (theme/index.ts:
  586-760, re-read at this pass — finding 7) + `generateSubtleSyntax`
  (560-584).
- Produces (binding for S1.6):
  - `Theme.Chroma() ansi.Chroma` — the full syntax token map.
  - `Theme.SubtleChroma() ansi.Chroma` — the pre-blended map (finding 3).
  - `(*Renderer).Render` now safe across themes/renderers (the slot
    workaround).

**Upstream parity notes (binding) — the chroma field map**
(upstream scope → chroma category → yolo token; opencode.dark hexes):

| `ansi.Chroma` field | upstream scope (getSyntaxRules) | token (hex) | attrs |
|---|---|---|---|
| `Text` | `"default"` | text (#eeeeee) | — |
| `Comment` | `"comment"`, `"comment.documentation"` | syntaxComment (#808080) | Italic |
| `CommentPreproc` | — (upstream has none; chroma emits it for `//go:build`) | syntaxComment (#808080) | Italic |
| `Keyword` | `"keyword"`, `"keyword.return/conditional/repeat/coroutine"` | syntaxKeyword (#9d7cd8) | Italic |
| `KeywordReserved` | — (a `"keyword"` subset) | syntaxKeyword (#9d7cd8) | Italic |
| `KeywordNamespace` | `"keyword.import"` | syntaxKeyword (#9d7cd8) | — (no italic upstream) |
| `KeywordType` | `"keyword.type"` | syntaxType (#e5c07b) | Bold + Italic |
| `Operator` | `"operator"`, `"keyword.operator"`, `"punctuation.delimiter"`, `"keyword.conditional.ternary"` | syntaxOperator (#56b6c2) | — |
| `Punctuation` | `"punctuation"`, `"punctuation.bracket"` | syntaxPunctuation (#eeeeee) | — |
| `Name` | base for the Name family (finding 4: NameVariable/NameProperty/NameEntity fall back here) | syntaxVariable (#e06c75) | — |
| `NameBuiltin` | `"variable.builtin"`, `"type.builtin"`, `"function.builtin"`, `"module.builtin"`, `"constant.builtin"`, `"variable.super"` | error (#e06c75) | — (upstream colors builtins with the ERROR token) |
| `NameAttribute` | `"property"` | syntaxVariable (#e06c75) | — |
| `NameClass` | `"class"` | syntaxType (#e5c07b) | — |
| `NameConstant` | `"constant"` | syntaxNumber (#f5a742) | — |
| `NameFunction` | `"keyword.function"`, `"function.method"`, `"variable.member"`, `"function"`, `"constructor"` | syntaxFunction (#fab283) | — |
| `LiteralNumber` | `"number"`, `"boolean"` | syntaxNumber (#f5a742) | — |
| `LiteralString` | `"string"`, `"symbol"` | syntaxString (#7fd88f) | — |
| `LiteralStringEscape` | — (upstream has no escape scope; chroma emits it inside strings) | syntaxString (#7fd88f) | — |

All other `Chroma` fields stay zero (chroma's parent-chain fallback covers
them — e.g. NameTag/NameDecorator/NameException have no upstream scope and
fall back to `Name` → syntaxVariable, the closest upstream default).
**Subtle map:** `generateSubtleSyntax` keeps the RGB and sets alpha =
`thinkingOpacity` — pre-blended per finding 3 (attributes preserved; only
the foreground is blended). `Theme` with a missing `background` token →
blend over `#000000`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/theme/syntax_test.go`:

```go
// TestChromaMapping pins the syntax* → ansi.Chroma field map (finding: the
// upstream getSyntaxRules scope table; opencode.dark hexes).
func TestChromaMapping(t *testing.T) {
	ch := resolveOpencodeDark(t).Chroma()
	check := func(name string, p ansi.StylePrimitive, want string) {
		t.Helper()
		if p.Color == nil || *p.Color != want {
			t.Errorf("%s = %v, want %s", name, p.Color, want)
		}
	}
	check("Text", ch.Text, "#eeeeee")
	check("Comment", ch.Comment, "#808080")
	if ch.Comment.Italic == nil || !*ch.Comment.Italic {
		t.Error("Comment.Italic = false/nil, want true")
	}
	check("Keyword", ch.Keyword, "#9d7cd8")
	if ch.Keyword.Italic == nil || !*ch.Keyword.Italic {
		t.Error("Keyword.Italic = false/nil, want true")
	}
	check("KeywordNamespace", ch.KeywordNamespace, "#9d7cd8")
	if ch.KeywordNamespace.Italic != nil {
		t.Error("KeywordNamespace.Italic set, want nil (upstream keyword.import has no italic)")
	}
	check("KeywordType", ch.KeywordType, "#e5c07b")
	if ch.KeywordType.Bold == nil || !*ch.KeywordType.Bold || ch.KeywordType.Italic == nil || !*ch.KeywordType.Italic {
		t.Error("KeywordType = bold+italic, got", ch.KeywordType.Bold, ch.KeywordType.Italic)
	}
	check("Operator", ch.Operator, "#56b6c2")
	check("Punctuation", ch.Punctuation, "#eeeeee")
	check("Name", ch.Name, "#e06c75")
	check("NameBuiltin", ch.NameBuiltin, "#e06c75")
	check("NameAttribute", ch.NameAttribute, "#e06c75")
	check("NameClass", ch.NameClass, "#e5c07b")
	check("NameConstant", ch.NameConstant, "#f5a742")
	check("NameFunction", ch.NameFunction, "#fab283")
	check("LiteralNumber", ch.LiteralNumber, "#f5a742")
	check("LiteralString", ch.LiteralString, "#7fd88f")
	check("LiteralStringEscape", ch.LiteralStringEscape, "#7fd88f")
	if ch.NameTag.Color != nil || ch.NameDecorator.Color != nil {
		t.Error("NameTag/NameDecorator must stay zero (no upstream scope)")
	}
}

// TestSubtleChroma pins the pre-blend (finding 3): fg = round(fg*α +
// bg*(1-α)) over the theme background, α = ThinkingOpacity (0.6 for
// opencode dark; bg #0a0a0a).
func TestSubtleChroma(t *testing.T) {
	th := resolveOpencodeDark(t)
	if th.R.ThinkingOpacity != 0.6 {
		t.Fatalf("ThinkingOpacity = %v, want 0.6", th.R.ThinkingOpacity)
	}
	sub := th.SubtleChroma()
	full := th.Chroma()
	check := func(name string, got, want string) {
		t.Helper()
		if got.Color == nil || *got.Color != want {
			t.Errorf("%s = %v, want %s", name, got.Color, want)
		}
	}
	check("Comment", sub.Comment, "#515151")  // #808080 @0.6 over #0a0a0a
	check("Keyword", sub.Keyword, "#624e86")  // #9d7cd8
	check("LiteralString", sub.LiteralString, "#50865a") // #7fd88f
	check("LiteralNumber", sub.LiteralNumber, "#97682c") // #f5a742
	check("Operator", sub.Operator, "#387178")  // #56b6c2
	// attributes survive the blend (only the foreground changes upstream)
	if sub.Keyword.Italic == nil || !*sub.Keyword.Italic {
		t.Error("subtle Keyword lost its Italic")
	}
	if *sub.Comment.Color == *full.Comment.Color {
		t.Error("subtle map identical to full — the blend did nothing")
	}
}

// TestChromaSlotWorkaround pins the finding-2 contract: two renderers with
// different chroma maps, rendered in EITHER order, each get their own
// colors (the global "charm" slot is deleted before every Render).
func TestChromaSlotWorkaround(t *testing.T) {
	th := resolveOpencodeDark(t)
	full, err := NewTranscriptRenderer(th, 77)
	if err != nil {
		t.Fatalf("full renderer: %v", err)
	}
	sub, err := NewReasoningRenderer(th, 77)
	if err != nil {
		t.Fatalf("subtle renderer: %v", err)
	}
	const md = "\n```go\nvar x = 1\n```\n"
	// The keyword token ("var") is color+italic. CHROMA code blocks render
	// through chroma's own terminal formatter (quick.Highlight), which
	// quantizes to 256-COLOR SGR even in a unit context — glamour's plain
	// text stays 24-bit, but the highlighted code is 38;5;N (verified:
	// full keyword #9d7cd8 -> 140, subtle pre-blended #624e86 -> 60).
	// Pin the 256 index as a substring.
	for _, order := range []struct {
		name string
		r    *Renderer
		want string
	}{
		{"full", full, "38;5;140"},
		{"subtle", sub, "38;5;60"},
	} {
		out, err := order.r.Render(md)
		if err != nil {
			t.Fatalf("Render(%s): %v", order.name, err)
		}
		if !strings.Contains(out, order.want) {
			t.Errorf("Render(%s) missing %q in: %q", order.name, order.want, out)
		}
	}
	// order matters in BOTH directions: render subtle first, then full —
	// the full renderer must still emit its OWN keyword (the slot delete
	// re-registers on the next code block; without it the full renderer
	// leaks the subtle 38;5;60, verified in the detail pass).
	if _, err := sub.Render(md); err != nil {
		t.Fatalf("Render(subtle, first): %v", err)
	}
	out, err := full.Render(md)
	if err != nil {
		t.Fatalf("Render(full, again): %v", err)
	}
	if !strings.Contains(out, "38;5;140") {
		t.Errorf("full renderer cross-colored by the subtle render: %q", out)
	}
}
```

Note: `NewReasoningRenderer` lands WITH this task (its only difference from
the transcript factory is the base token + the subtle chroma — the S1.6
reasoning render consumes it; the factory is a two-line delegate):

```go
// NewReasoningRenderer builds the expanded-reasoning renderer: the
// textMuted base + the pre-blended chroma map (upstream
// generateSubtleSyntax, theme/index.ts:560-584).
func NewReasoningRenderer(th Theme, width int) (*Renderer, error) {
	return newRenderer(th, width, "textMuted", th.SubtleChroma())
}
```

- [ ] **Step 2: Run to verify it fails**

```sh
go test ./internal/tui/theme/ -run 'TestChroma|TestSubtleChroma' -v
```

Expect FAIL — `Chroma`/`SubtleChroma`/`NewReasoningRenderer` undefined.

- [ ] **Step 3: Write the minimal implementation**

`internal/tui/theme/syntax.go` — add:

```go
import (
	// ... existing imports ...
	"math"

	"github.com/alecthomas/chroma/v2/styles"
)

// Chroma builds the full syntax token map (upstream getSyntaxRules,
// theme/index.ts:586-760 — the scope table in the S1.4 plan notes).
func (t Theme) Chroma() ansi.Chroma {
	c := ansi.Chroma{}
	c.Text = ansi.StylePrimitive{Color: t.md("text")}
	c.Comment = ansi.StylePrimitive{Color: t.md("syntaxComment"), Italic: boolPtr(true)}
	c.CommentPreproc = ansi.StylePrimitive{Color: t.md("syntaxComment"), Italic: boolPtr(true)}
	c.Keyword = ansi.StylePrimitive{Color: t.md("syntaxKeyword"), Italic: boolPtr(true)}
	c.KeywordReserved = ansi.StylePrimitive{Color: t.md("syntaxKeyword"), Italic: boolPtr(true)}
	c.KeywordNamespace = ansi.StylePrimitive{Color: t.md("syntaxKeyword")}
	c.KeywordType = ansi.StylePrimitive{Color: t.md("syntaxType"), Bold: boolPtr(true), Italic: boolPtr(true)}
	c.Operator = ansi.StylePrimitive{Color: t.md("syntaxOperator")}
	c.Punctuation = ansi.StylePrimitive{Color: t.md("syntaxPunctuation")}
	c.Name = ansi.StylePrimitive{Color: t.md("syntaxVariable")}
	c.NameBuiltin = ansi.StylePrimitive{Color: t.md("error")}
	c.NameAttribute = ansi.StylePrimitive{Color: t.md("syntaxVariable")}
	c.NameClass = ansi.StylePrimitive{Color: t.md("syntaxType")}
	c.NameConstant = ansi.StylePrimitive{Color: t.md("syntaxNumber")}
	c.NameFunction = ansi.StylePrimitive{Color: t.md("syntaxFunction")}
	c.LiteralNumber = ansi.StylePrimitive{Color: t.md("syntaxNumber")}
	c.LiteralString = ansi.StylePrimitive{Color: t.md("syntaxString")}
	c.LiteralStringEscape = ansi.StylePrimitive{Color: t.md("syntaxString")}
	return c
}
```

`SubtleChroma` (the reasoning variant — upstream generateSubtleSyntax,
theme/index.ts:560-584: RGB kept, alpha set to ThinkingOpacity. SGR 24-bit
carries no alpha and lipgloss v2 parseHex takes 6-digit hex only, so each
foreground is PRE-BLENDED over the theme background: `out = round(fg*α +
bg*(1-α))`, half-up per channel; absent background → #000000) blends the
TOKEN colors directly (the token name is the source of truth):

```go
func (t Theme) SubtleChroma() ansi.Chroma {
	full := t.Chroma()
	alpha := t.R.ThinkingOpacity
	if alpha <= 0 || alpha >= 1 {
		return full
	}
	bg := Rgba{0, 0, 0, 255}
	if c, ok := t.R.Color("background"); ok {
		bg = c
	}
	// pairs is the (field pointer, token) set — exactly the fields
	// Chroma() sets.
	type pair struct {
		p   *ansi.StylePrimitive
		tok string
	}
	pairs := []pair{
		{&full.Text, "text"}, {&full.Comment, "syntaxComment"},
		{&full.CommentPreproc, "syntaxComment"}, {&full.Keyword, "syntaxKeyword"},
		{&full.KeywordReserved, "syntaxKeyword"}, {&full.KeywordNamespace, "syntaxKeyword"},
		{&full.KeywordType, "syntaxType"}, {&full.Operator, "syntaxOperator"},
		{&full.Punctuation, "syntaxPunctuation"}, {&full.Name, "syntaxVariable"},
		{&full.NameBuiltin, "error"}, {&full.NameAttribute, "syntaxVariable"},
		{&full.NameClass, "syntaxType"}, {&full.NameConstant, "syntaxNumber"},
		{&full.NameFunction, "syntaxFunction"}, {&full.LiteralNumber, "syntaxNumber"},
		{&full.LiteralString, "syntaxString"}, {&full.LiteralStringEscape, "syntaxString"},
	}
	for _, pr := range pairs {
		if pr.p.Color == nil {
			continue
		}
		fg, ok := t.R.Color(pr.tok)
		if !ok {
			continue
		}
		out := Rgba{
			R: uint8(math.Round(float64(fg.R)*alpha + float64(bg.R)*(1 - alpha))),
			G: uint8(math.Round(float64(fg.G)*alpha + float64(bg.G)*(1 - alpha))),
			B: uint8(math.Round(float64(fg.B)*alpha + float64(bg.B)*(1 - alpha))),
			A: 255,
		}
		s := hex6(out)
		pr.p.Color = &s
	}
	return full
}
```

(No cross-file change — `syntax.go` is the whole surface of this task.)

`Renderer` + factories (REPLACES the S1.2 `Renderer`/`NewTranscriptRenderer`/
`Render`):

```go
// Renderer is a glamour TermRenderer bound to one theme + width. The
// chroma field is the map this renderer registered under the GLOBAL
// "charm" slot (finding 2): Render deletes the slot first, so this
// renderer's map (re-)registers on the next code block — transcript (full)
// and reasoning (subtle) renderers + SIGUSR2 theme switches never
// cross-color. The TUI renders single-threaded (bubbletea View), so
// sequential re-registration is safe.
type Renderer struct {
	tr     *glamour.TermRenderer
	chroma *ansi.Chroma
}

func newRenderer(th Theme, width int, base string, ch ansi.Chroma) (*Renderer, error) {
	cfg := th.StyleConfig(base, width)
	cfg.CodeBlock.Chroma = &ch
	opts := []glamour.TermRendererOption{glamour.WithStyles(cfg)}
	if width > 0 {
		opts = append(opts, glamour.WithWordWrap(width))
	}
	tr, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return nil, err
	}
	return &Renderer{tr: tr, chroma: &ch}, nil
}

// NewTranscriptRenderer builds the text-part renderer: the markdownText
// base + the FULL chroma map, word-wrap at width (w-3; <=0 disables it).
func NewTranscriptRenderer(th Theme, width int) (*Renderer, error) {
	return newRenderer(th, width, "markdownText", th.Chroma())
}

// NewReasoningRenderer builds the expanded-reasoning renderer: the
// textMuted base + the pre-blended chroma map (upstream
// generateSubtleSyntax, theme/index.ts:560-584).
func NewReasoningRenderer(th Theme, width int) (*Renderer, error) {
	return newRenderer(th, width, "textMuted", th.SubtleChroma())
}

// Render renders md to an ANSI string. It clears the global "charm" chroma
// slot first (finding 2) so THIS renderer's map (re-)registers.
func (r *Renderer) Render(md string) (string, error) {
	if r.chroma != nil {
		delete(styles.Registry, "charm")
	}
	return r.tr.Render(md)
}
```

(Replace the S1.2 `Renderer`/`NewTranscriptRenderer`/`Render` — the S1.2
tests stay green: `TestStyleConfigMapping` asserts `CodeBlock.Chroma ==
nil` on the raw `StyleConfig()` output, which is unchanged; the factories
now attach the map.)

- [ ] **Step 4: Run to verify it passes, then gate**

```sh
go test ./internal/tui/theme/ -v
go vet ./... && go test ./... && gofmt -l .
```

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/theme/syntax.go internal/tui/theme/syntax_test.go
git commit -m "feat: syntax-highlighted code blocks (theme syntax tokens)"
bd close yolo-oae.2.4 --reason "Chroma + SubtleChroma (pre-blend @thinkingOpacity); global 'charm' slot workaround (delete-before-Render); both render orderings pinned" --json
```

**STOP**: report the gate result, the commit, `git status`.

### Task S1.5: GFM in transcript - tables, task lists, strikethrough + tests (`yolo-oae.2.5`)

**Files:**
- Modify: `internal/tui/theme/syntax.go` — the GFM trio in `StyleConfig`
  (`Strikethrough`, `Task`, `Table`).
- Modify: `internal/tui/theme/syntax_test.go` — the GFM fixture tests.

**Interfaces:**
- Consumes: S1.2 `StyleConfig`; S1.4 `Renderer`; glamour's goldmark GFM
  extension (default-on — no option to set).
- Produces: tables render as bordered grids, task lists as (hidden-checkbox)
  bullets, strikethrough with SGR 9 — all theme-tokened where a token
  exists.

**Upstream parity notes (binding):**
- Task lists: upstream HIDES the checkbox (finding 6 — the opentui
  `createListItemRenderable` skips `checkbox` tokens) → glamour
  `Task.Ticked`/`Task.Unticked = "• "` (the Item bullet). Glamour's
  `TaskElement` (ansi/task.go:16-28) applies `Task.StylePrimitive` to the
  (empty) checkbox element ONLY — the item TEXT renders in the base Text
  color: upstream parity (opentui list items carry the markdown base fg).
  `Task.Color = markdownListItem` is set anyway (configuration pin; no
  visible effect in v2.0.1). (The spec's "task lists" = task items render
  in lists, NOT checkbox glyphs.)
- Strikethrough: upstream has no token (no `markdownStrikethrough` key) →
  the attribute only (`CrossedOut: true`); the color inherits the surrounding
  text. Verified v2.0.1 emission: the run resets first (`\x1b[m`), then
  standalone `\x1b[9m`.
- Tables: the upstream `TextTable style:"grid"` box-drawn borders vs the
  glamour default grid (verified v2.0.1: `─` rule lines, `│` column
  separators, `┼` joins, NO corner glyphs, full word-wrap width) —
  structurally the same grid; the S1 pty diff arbitrates border chars
  (spec §4). No token exists for table borders (upstream uses the element
  fg) → left at the glamour default.

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/theme/syntax_test.go`:

```go
// TestStyleConfigGFM pins the S1.5 GFM trio (opencode.dark).
func TestStyleConfigGFM(t *testing.T) {
	cfg := resolveOpencodeDark(t).StyleConfig("markdownText", 77)
	if cfg.Strikethrough.CrossedOut == nil || !*cfg.Strikethrough.CrossedOut {
		t.Error("Strikethrough.CrossedOut = false/nil, want true")
	}
	if cfg.Strikethrough.Color != nil {
		t.Error("Strikethrough.Color set, want nil (no upstream token)")
	}
	if cfg.Task.Ticked != "• " || cfg.Task.Unticked != "• " {
		t.Errorf("Task ticks = %q/%q, want '• '/'• ' (upstream hides the checkbox)",
			cfg.Task.Ticked, cfg.Task.Unticked)
	}
	if cfg.Task.Color == nil || *cfg.Task.Color != "#fab283" {
		t.Errorf("Task.Color = %v, want #fab283 (markdownListItem)", cfg.Task.Color)
	}
}

// TestGFMRender pins the three GFM features end-to-end (theme opencode
// dark; the 24-bit SGR is asserted directly — the 38;5;N quantization is
// the teatest layer's job). Verified against glamour v2.0.1 behavior:
// the Task element's StylePrimitive styles only the checkbox (the item
// TEXT renders in the base Text color — upstream parity: opentui's list
// items carry the markdown base fg); the table grid is the │ / ┼ / ─
// column layout (no corner glyphs); the strikethrough run resets to
// default first, so SGR 9 is standalone.
func TestGFMRender(t *testing.T) {
	r, err := NewTranscriptRenderer(resolveOpencodeDark(t), 77)
	if err != nil {
		t.Fatalf("NewTranscriptRenderer: %v", err)
	}
	// 1) table: the glamour grid column borders (│ separator, ┼ join).
	out, err := r.Render("| a | b |\n|---|---|\n| 1 | 2 |\n")
	if err != nil {
		t.Fatalf("Render(table): %v", err)
	}
	for _, want := range []string{"\u2502", "\u253C"} { // │ ┼
		if !strings.Contains(out, want) {
			t.Errorf("table missing border %q in %q", want, out)
		}
	}
	// 2) task list: hidden checkbox, "• " bullets, the item text in the
	// base text color (38;2;238;238;238 = markdownText #eeeeee).
	out, err = r.Render("- [x] done\n- [ ] todo\n")
	if err != nil {
		t.Fatalf("Render(task): %v", err)
	}
	if !strings.Contains(out, "\u2022 done") || !strings.Contains(out, "\u2022 todo") {
		t.Errorf("task list = %q, want '• done' / '• todo'", out)
	}
	if strings.Contains(out, "[x]") || strings.Contains(out, "[ ]") {
		t.Errorf("checkbox visible: %q", out)
	}
	if !strings.Contains(out, "38;2;238;238;238") { // base text color
		t.Errorf("task item missing the base text color: %q", out)
	}
	// 3) strikethrough: SGR 9 (crossed-out), standalone after a reset
	// (\x1b[9m) — or merged, if glamour ever changes the pen handling.
	out, err = r.Render("a ~~gone~~ word\n")
	if err != nil {
		t.Fatalf("Render(strike): %v", err)
	}
	if !strings.Contains(out, "\x1b[9m") && !strings.Contains(out, ";9m") {
		t.Errorf("strikethrough missing SGR 9: %q", out)
	}
	if !strings.Contains(out, "gone") {
		t.Errorf("strikethrough lost its text: %q", out)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```sh
go test ./internal/tui/theme/ -run 'TestStyleConfigGFM|TestGFMRender' -v
```

Expect FAIL — `TestStyleConfigGFM`: `Strikethrough.CrossedOut` nil +
`Task.Ticked`/`Unticked` empty; `TestGFMRender`: the task list renders
glamour's default (hidden checkbox AND no bullet — bare "done"/"todo"
lines, no "• "), and the strikethrough carries no SGR 9 (the `~~` markers
are stripped, plain text).

- [ ] **Step 3: Write the minimal implementation**

`internal/tui/theme/syntax.go`, `StyleConfig` — insert after the `Strong`
line:

```go
	cfg.Strikethrough = ansi.StylePrimitive{CrossedOut: boolPtr(true)}
```

and after the `Enumeration` line:

```go
	// Task: the upstream <markdown> element HIDES the checkbox
	// (opentui createListItemRenderable skips checkbox tokens) — a task
	// item is a plain bullet in the item color.
	cfg.Task = ansi.StyleTask{
		StylePrimitive: ansi.StylePrimitive{Color: t.md("markdownListItem")},
		Ticked:         "• ",
		Unticked:       "• ",
	}
```

(`Table` stays zero — the glamour default lipgloss grid, finding 7.)

- [ ] **Step 4: Run to verify it passes, then gate**

```sh
go test ./internal/tui/theme/ -v
go vet ./... && go test ./... && gofmt -l .
```

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/theme/syntax.go internal/tui/theme/syntax_test.go
git commit -m "feat: GFM in transcript - tables, task lists, strikethrough"
bd close yolo-oae.2.5 --reason "GFM trio: hidden-checkbox bullets (upstream parity), SGR-9 strikethrough, default lipgloss table grid" --json
```

**STOP**: report the gate result, the commit, `git status`.

### Task S1.6: Reasoning block restyle (dimmed collapsible) + tests (`yolo-oae.2.6`)

**Files:**
- Modify: `internal/tui/session.go` — the reasoning `case` in
  `renderAssistant` (upstream ReasoningPart semantics); `renderMessages` +
  `sessionModel.sync` gain the spin-frame param.
- Modify: `internal/tui/view.go` — `viewSession` passes `a.spinFrame()`.
- Modify: `internal/tui/session_test.go` + `internal/tui/session_theme_
  test.go` + `internal/tui/session_bench_test.go` — every
  `renderMessages(...)` call site gains the `""` spin arg (session_test.go:
  198,237,352; session_theme_test.go:265; session_bench_test.go:117);
  `TestSessionChromeZeroThemeIsPlain` re-baselines the `▸ think` line →
  `Thinking` (this task's reasoning form, zero theme, empty spin).
- Create: `internal/tui/session_reasoning_test.go` — the unit header/
  duration/summary tests + a teatest golden.

**Interfaces:**
- Consumes: S1.4 `NewReasoningRenderer`; S1.3 the renderer wiring;
  `protocol.PartTime{Start, End int64}`; the S0.10 teatest convention;
  `a.spinFrame()` (footer.go — the locked 5-frame braille spinner).
- Produces: `renderMessages(st, expanded, w, th, spin string)`,
  `sessionModel.sync(st, w, h, th, spin)` — the spin frame threads into
  every render path (the running reasoning row + the S1.7 running tool
  rows reuse it).

**Upstream parity notes (binding)** (ReasoningPart, index.tsx:1584-1690):
- Content: `part.text` minus `"[REDACTED]"`, trimmed; an empty part renders
  NOTHING (the upstream `<Show when={content()}>`).
- `isDone = time.end != 0` (yolo `PartTime.End` is `omitempty` — 0 =
  unset); `duration = max(0, end-start)`.
- `reasoningSummary` (thinking.ts:12): the leading `**title**` block
  (`/^\*\*([^*\n]+)\*\*(?:\r?\n\r?\n|$)/`) → `{title, body}`; no match →
  `{null, whole content}`.
- Header row:
  - RUNNING: `<spinner> Thinking: <title>` / `<spinner> Thinking` — fg =
    warning @ thinkingOpacity → yolo: the pre-blended warning hex (the
    S1.4 blend math on the `warning` token) — yolo's spinner is the locked
    5-frame braille (`a.spinFrame()`), passed as the `spin` param
    (upstream's 10-frame/80ms spinner is NOT ported — the S0 footer spinner
    is the yolo spinner; no deviation: the spinner glyph set is a yolo
    shell element, S0-owned).
  - DONE: `"+ Thought: <title> · <duration>"` (closed) / `"- Thought:
    <title> · <duration>"` (open); title-only: `"+/- Thought: <title>"`;
    duration-only: `"+/- Thought: <duration>"`; both absent: `"+/-
    Thought"`. fg = warning (closed) / warning @ thinkingOpacity (open).
  - yolo: `expanded[p.ID]` = open; the alt+t binding (session.go:41) is the
    toggle (upstream's click + thinkingMode=hide).
- Body (only when open AND `summary.body != ""`): the markdown of
  `summary.body` through the REASONING renderer (textMuted base + subtle
  chroma — S1.4), indented 3 (the part box) + 2 (the body box, upstream
  `paddingLeft=2`) = 5 spaces per line; upstream `marginTop=1` ≈ the
  existing inter-part `"\n"`.
- The current yolo `▸/▾ think` rows are REPLACED. Re-baseline due in this
  commit: `TestSessionChromeZeroThemeIsPlain` (session_theme_test.go:268)
  pins `"\u25B8 think\n"` in its byte-exact want block → becomes
  `"Thinking\n"` (zero theme, empty spin). The themed teatest
  (TestSessionChromeThemeSGR) does NOT pin the reasoning row — its
  scenario carries no reasoning part, so no change there.

**Ported helpers** (new, in `session.go`):

```go
// reasoningSummary ports upstream thinking.ts:12: the leading **title**
// block is disclosure metadata; the rest is the markdown body.
func reasoningSummary(text string) (title string, body string) {
	content := strings.TrimSpace(text)
	i := strings.Index(content, "**")
	if i != 0 {
		return "", content
	}
	j := strings.Index(content[2:], "**")
	if j < 0 {
		return "", content
	}
	title = strings.TrimSpace(content[2 : 2+j])
	if title == "" {
		return "", content
	}
	rest := content[2+j+2:]
	// the upstream regex requires the title block to end at a blank line
	// or the end of the content.
	if rest == "" {
		return title, ""
	}
	if !strings.HasPrefix(rest, "\n\n") && !strings.HasPrefix(rest, "\r\n\r\n") {
		return "", content
	}
	return title, strings.TrimRight(rest, " \t\r\n")
}

// durationText ports upstream Locale.duration (util/locale.ts:39): ms <
// 1s, X.Xs < 1m, Nm Ns < 1h, Nh Nm < 24h, else Nd Nh.
func durationText(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	if ms < 3600000 {
		return fmt.Sprintf("%dm %ds", ms/60000, (ms%60000)/1000)
	}
	if ms < 86400000 {
		return fmt.Sprintf("%dh %dm", ms/3600000, (ms%3600000)/60000)
	}
	return fmt.Sprintf("%dd %dh", ms/86400000, (ms%86400000)/3600000)
}
```

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/session_reasoning_test.go`:

```go
package tui

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/llm"
	"github.com/kido5217/yolo/internal/llm/fake"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// TestReasoningSummary ports the thinking.ts:12 regex table.
func TestReasoningSummary(t *testing.T) {
	tests := []struct {
		in       string
		title    string
		body     string
	}{
		{in: "**Inspecting PR workflow**\n\nThe body here.", title: "Inspecting PR workflow", body: "The body here."},
		{in: "**Title only**", title: "Title only", body: ""},
		{in: "**No blank line**\ntext", title: "", body: "**No blank line**\ntext"},
		{in: "no title at all", title: "", body: "no title at all"},
		{in: "  **Padded**\n\nbody", title: "Padded", body: "body"},
	}
	for _, tc := range tests {
		gotT, gotB := reasoningSummary(tc.in)
		if gotT != tc.title || gotB != tc.body {
			t.Errorf("reasoningSummary(%q) = (%q, %q), want (%q, %q)",
				tc.in, gotT, gotB, tc.title, tc.body)
		}
	}
}

// TestDurationText ports the Locale.duration table (util/locale.ts:39).
func TestDurationText(t *testing.T) {
	tests := []struct {
		ms   int64
		want string
	}{
		{500, "500ms"}, {1000, "1.0s"}, {1234, "1.2s"},
		{61000, "1m 1s"}, {3600000, "1h 0m"}, {90061000, "1d 1h"},
	}
	for _, tc := range tests {
		if got := durationText(tc.ms); got != tc.want {
			t.Errorf("durationText(%d) = %q, want %q", tc.ms, got, tc.want)
		}
	}
}

// TestReasoningPartSGR pins the S1.6 reasoning restyle under the pinned
// TTY_FORCE=1 + TERM=xterm-256color env: the DONE reasoning row is the
// "+/- Thought: <title> · <duration>" line, and the expanded body renders
// through the subtle renderer (the base text takes the pre-blended
// textMuted #515151 → 38;5;239). The duration is NOT pinned to a value:
// the engine stamps PartTime itself (round.go:52,86 — Start at part
// creation, End at finalization), so the wall-clock ms is nondeterministic;
// the regex pins the row SHAPE. (The fake part carries no Time of its own
// — llm.Part has no Time field; the engine owns it.)
func TestReasoningPartSGR(t *testing.T) {
	thoughtRe := regexp.MustCompile(`[+-] Thought: Planning · \d+(ms|\.\ds)`)
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "reasoning", Text: "**Planning**\n\nvar x = 1"},
		}},
	)
	ts := testutil.BootWithDriverConfig(t, drv, &protocol.Config{})
	dir := t.TempDir()
	e, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	if err := e.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve: %v", err)
	}

	c := client.New(ts.URL, ts.Dir)
	a := NewApp(c, store.State{}, "", e)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a,
		teatest.WithInitialTermSize(80, 24),
		teatest.WithProgramOptions(tea.WithEnvironment([]string{
			"TTY_FORCE=1", "TERM=xterm-256color",
		})),
	)

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "hi")
	tm.Send(press(tea.KeyEnter))
	// the turn is done: the reasoning row shows (collapsed by default).
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return thoughtRe.Match([]byte(stripANSI(string(b))))
	}, teatest.WithDuration(10*time.Second))
	// alt+t expands: the body renders (subtle markdown).
	tm.Send(pressAlt('t'))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		s := stripANSI(string(b))
		if !strings.Contains(s, "- Thought: Planning · ") {
			return false
		}
		if !strings.Contains(s, "var x = 1") {
			return false
		}
		// subtle base text: the pre-blended textMuted (#515151 → 239).
		return bytes.Contains(b, []byte("38;5;239"))
	}, teatest.WithDuration(10*time.Second))

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
```

(`pressAlt` exists — prompt_test.go:27; the alt+t binding is session.go:41.
The test file imports add `regexp`.)

- [ ] **Step 2: Run to verify it fails**

```sh
go test ./internal/tui/ -run 'TestReasoning' -v
```

Expect FAIL — the rows still read `▸/▾ think` (no title/duration, no
subtle body).

- [ ] **Step 3: Write the minimal implementation**

`internal/tui/session.go`:

(1) `renderMessages` — the spin param + the reasoning renderer:

```go
func renderMessages(st *store.State, expanded map[string]bool, w int, th theme.Theme, spin string) string {
	var tr, rr *theme.Renderer
	if !th.Zero() {
		if built, err := theme.NewTranscriptRenderer(th, w-3); err == nil {
			tr = built
		}
		if built, err := theme.NewReasoningRenderer(th, w-5); err == nil {
			rr = built
		}
	}
	blocks := make([]string, 0, len(st.Messages))
	for _, m := range st.Messages {
		if m.Info.Role == "user" {
			blocks = append(blocks, renderUser(m, w))
		} else {
			blocks = append(blocks, renderAssistant(m, expanded, w, th, tr, rr, spin))
		}
	}
	// ... (unchanged)
}
```

(2) `renderAssistant` — signature `(m, expanded, w, th, tr, rr *theme.Renderer, spin string)`; the reasoning `case` becomes:

```go
		case "reasoning":
			text := strings.TrimSpace(strings.ReplaceAll(p.Text, "[REDACTED]", ""))
			if text == "" {
				continue
			}
			done := p.Time.End != 0
			dur := int64(0)
			if done {
				dur = p.Time.End - p.Time.Start
				if dur < 0 {
					dur = 0
				}
			}
			title, body := reasoningSummary(text)
			open := expanded[p.ID]
			// The upstream header fg: warning PRE-BLENDED at ThinkingOpacity
			// while running (the Spinner color, index.tsx:1660) and when
			// open (1657-1659); full warning when done+closed.
			var hdr lipgloss.Style
			if !done || open {
				hdr = th.WarningSubtle()
			} else {
				hdr = th.Warning()
			}
			row := ""
			switch {
			case !done:
				label := "Thinking"
				if title != "" {
					label = "Thinking: " + title
				}
				if spin != "" {
					row = spin + " " + label
				} else {
					row = label // zero-theme/unit runs pass "" — no leading space
				}
			case title != "" && dur > 0:
				row = openMark(open) + " Thought: " + title + " · " + durationText(dur)
			case title != "":
				row = openMark(open) + " Thought: " + title
			case dur > 0:
				row = openMark(open) + " Thought: " + durationText(dur)
			default:
				row = openMark(open) + " Thought"
			}
			writeStyled(hdr, row)
			if open && body != "" && rr != nil {
				if out, err := rr.Render(body); err == nil {
					for _, l := range strings.Split(strings.Trim(out, "\n"), "\n") {
						writeRaw("     " + l) // 3 (part box) + 2 (body box)
					}
				}
			}
```

with:

```go
func openMark(open bool) string {
	if open {
		return "- "
	}
	return "+ "
}
```

(3) `theme.Theme` gains the pre-blended warning accessor (in
`styles.go`, next to the S0.3 accessors — the same blend math as
`SubtleChroma`, one token):

```go
// WarningSubtle is warning pre-blended over the background at
// ThinkingOpacity (upstream RGBA.fromValues(warning.r, g, b,
// thinkingOpacity), index.tsx:1660) — the lipgloss hex takes 6 digits,
// so the alpha is pre-applied (finding 3).
func (t Theme) WarningSubtle() lipgloss.Style {
	return t.blended("warning")
}

// blended returns fg(token) with the color pre-blended over the
// background at ThinkingOpacity (identity when the token or background
// is absent or α is out of (0,1)).
func (t Theme) blended(token string) lipgloss.Style {
	c, ok := t.R.Color(token)
	if !ok {
		return lipgloss.NewStyle()
	}
	a := t.R.ThinkingOpacity
	if a <= 0 || a >= 1 {
		return t.fg(token)
	}
	bg := Rgba{0, 0, 0, 255}
	if bc, ok := t.R.Color("background"); ok {
		bg = bc
	}
	out := Rgba{
		R: uint8(math.Round(float64(c.R)*a + float64(bg.R)*(1 - a))),
		G: uint8(math.Round(float64(c.G)*a + float64(bg.G)*(1 - a))),
		B: uint8(math.Round(float64(c.B)*a + float64(bg.B)*(1 - a))),
		A: 255,
	}
	return lipgloss.NewStyle().Foreground(hex6(out))
}
```

(Refactor S1.4's `SubtleChroma` blend loop to call the same per-channel
math if it wants — optional, not required.)

(4) `sessionModel.sync` — the spin param:

```go
func (s *sessionModel) sync(st *store.State, w, h int, th theme.Theme, spin string) {
	// ... existing body, with:
	s.lines = strings.Split(renderMessages(st, s.expanded, w, th, spin), "\n")
	// ...
}
```

`internal/tui/view.go` — `a.sess.sync(&a.store, w, h, a.theme, a.spinFrame())`.

Re-baseline the call sites (this commit): every `renderMessages(...)` test
call gains the `""` spin arg — `session_test.go:198,237,352`,
`session_theme_test.go:265`, `session_bench_test.go:117`. In
`session_theme_test.go` `TestSessionChromeZeroThemeIsPlain`, the want
block's `"\u25B8 think\n"` line becomes `"Thinking\n"` (zero theme, empty
spin → the running label with no spinner, no leading space); the tool rows
and `!` line stay THIS task (they re-baseline in S1.7/S1.8).

- [ ] **Step 4: Run to verify it passes, then gate**

```sh
go test ./internal/tui/ -run 'TestReasoning|TestRenderMessages' -v
go vet ./... && go test ./... && gofmt -l .
```

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/session.go internal/tui/view.go internal/tui/theme/styles.go internal/tui/session_test.go internal/tui/session_theme_test.go internal/tui/session_bench_test.go internal/tui/session_reasoning_test.go
git commit -m "feat: reasoning block restyle (dimmed collapsible)"
bd close yolo-oae.2.6 --reason "upstream ReasoningPart ported: spinner/Thought rows, summary title, Locale.duration, subtle-markdown body; spin frame threads sync" --json
```

**STOP**: report the gate result, the commit, `git status`.

### Task S1.7: Tool-row restyle (glyphs + expand) + tests (`yolo-oae.2.7`)

**Files:**
- Modify: `internal/tui/session.go` — `toolRowLine` → `toolRow(p, th,
  spin)` on the upstream InlineTool semantics (per-tool glyphs, pending
  text, icon-width error expansion); the tool `case` in `renderAssistant`.
- Modify: `internal/tui/session_theme_test.go` — the S0.10
  `TestToolRowLineTheme` re-baselined to the new row form; the S0.10
  teatest `completedRowRe`/`errorRowRe` regexes re-anchored on the icons;
  `TestSessionChromeZeroThemeIsPlain` want block re-baselined (the three
  tool rows — see Step 1).
- Create: `internal/tui/session_toolrow_test.go` — the per-tool glyph/
  pending table test.

**Interfaces:**
- Consumes: S1.6 `spin` param; S0.10 `toolRowLine` (replaced); the S0.10
  teatest suite.
- Produces: `toolRow(p protocol.Part, th theme.Theme, spin string)
  (lipgloss.Style, string, bool)` — single style per row (the icon and
  text share the state color in every state — upstream's iconColor ??
  color and errorColor both resolve to the row's state color in yolo's
  three states).

**Upstream parity notes (binding)** (InlineTool 1844-1920 +
InlineToolRow 1922-2000 + the per-tool sites, finding 7):
- Icon map (per tool; 2-column slot — `INLINE_TOOL_ICON_WIDTH = 2`):
  bash `"$"`, write `"←"`, edit `"←"`, glob `"✱"`, grep `"✱"`, read `"→"`,
  todowrite `"⚙"`, default `"⚙"`.
- Indent (pty-diff arbitrates — NOT applied in S1.7): upstream wraps the
  whole row in `<box paddingLeft={3}>` (1942-1943, the same 3-column
  indent the S1.3 text parts take); the PENDING row adds a further
  `<text paddingLeft={3}>` (1964-1966) → 6 columns; the expanded ERROR
  block is `<box paddingLeft={2}>` (1993-1994) → 5 columns. yolo KEEPS
  the S0 column-0 baseline for the row (the byte-exact zero-theme +
  S0.10 SGR goldens are re-baselined on it) so the restyle task stays
  focused on glyphs/pending/spinner/failure/expansion — the 3-column
  tool indent is a layout consistency item against the S1.3 text indent,
  left to the S1 pty diff (spec §4).
- Pending text (the RUNNING row, upstream `<text paddingLeft=3>~
  <pending>` at fg=text): bash "Writing command...", write "Preparing
  write...", edit "Preparing edit...", glob "Finding files...", grep
  "Searching content...", read "Reading file...", todowrite "Updating
  todos...", default "Working...". The READ running row is the upstream
  exception: `spinner={isRunning()}` (index.tsx:2180) — yolo renders
  `<spin> Reading file...` (the S1.6 spin frame) instead of `~`.
- COMPLETED row: `<icon> <title>` at fg=textMuted — the icon takes a
  2-column slot (glyph + space), the title is `state.Title` (yolo's
  existing `toolTitleFallback` — the upstream per-tool `children` text
  ("Write <path>", `Grep "pat" (N matches)`, …) is NOT ported: yolo's
  server-side Title already carries the display text, and the upstream
  children are tool-specific JSX; the row form `<icon> <title>` is the
  parity mapping — pty-diff arbitrates).
- ERROR row (failed = error && !denied): `<icon> <failure ?? title>` at
  fg=error — per-tool `failure`: todowrite "Todo update failed" (2545),
  all others none → the title. yolo has no denied/permission state on the
  row (the S0.10 note: the ask is the overlay, S2–S3 owns it).
- Expansion (yolo alt+e = upstream click): the ERROR part shows the FULL
  error at paddingLeft 2 (the icon width) in fg=error (upstream
  InlineToolRow 1992-1999) — yolo currently shows it plain; the COMPLETED
  parts keep the S0 bash inline preview (10-line head) — unchanged.
- The row no longer carries the tool NAME (upstream shows icon + text
  only) — the S0.10 rows `✓/▶/✗ <tool> <title>` become the glyph form.

- [ ] **Step 1: Write the failing tests (+ re-baseline the S0.10 unit table)**

Create `internal/tui/session_toolrow_test.go`:

```go
package tui

import (
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// TestToolRowGlyphs pins the S1.7 per-tool icon + pending/complete/error
// row forms (the opencode dark tokens; the SGR quantization is the
// teatest layer's job).
func TestToolRowGlyphs(t *testing.T) {
	all, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(all["opencode"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := theme.Theme{R: r, Name: "opencode", Mode: "dark"}
	part := func(tool string, status, title, errMsg string) protocol.Part {
		return protocol.Part{ID: "t", Type: "tool", Tool: tool, CallID: "c",
			State: &protocol.ToolState{Status: status, Title: title, Error: errMsg}}
	}
	tests := []struct {
		name   string
		p      protocol.Part
		want   string
		fgWant string
	}{
		{"bash completed", part("bash", "completed", "ls -la", ""), "$ ls -la", "#808080"},
		{"bash running", part("bash", "running", "", ""), "~ Writing command...", "#eeeeee"},
		{"read running (spinner)", part("read", "running", "", ""), "⠋ Reading file...", "#eeeeee"},
		{"write completed", part("write", "completed", "f.go", ""), "← f.go", "#808080"},
		{"edit completed", part("edit", "completed", "f.go", ""), "← f.go", "#808080"},
		{"glob running", part("glob", "running", "", ""), "~ Finding files...", "#eeeeee"},
		{"grep error", part("grep", "error", "grep", "no match"), "✱ grep", "#e06c75"},
		{"todowrite error (failure text)", part("todowrite", "error", "todos", "boom"), "⚙ Todo update failed", "#e06c75"},
		{"unknown tool completed", part("webfetch", "completed", "url", ""), "⚙ url", "#808080"},
		{"read error", part("read", "error", "f.go", "not found"), "→ f.go", "#e06c75"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sty, row, ok := toolRow(tc.p, th, "⠋")
			if !ok || row != tc.want {
				t.Fatalf("toolRow = (%q, %v), want (%q, true)", row, ok, tc.want)
			}
			// the S0.10 mechanism (session_theme_test.go:241): lipgloss
			// GetForeground returns the 24-bit hex as an opaque RGBA.
			if got, want := sty.GetForeground(), hexColor(tc.fgWant); got != want {
				t.Errorf("fg = %v, want %v", got, want)
			}
		})
	}
}
```

(`hexColor` is the S0.10 helper in session_theme_test.go:252 — same
package, reused as-is.)

Re-baseline `internal/tui/session_theme_test.go` (this commit):

1. `TestToolRowLineTheme` table (old → new; the `fgWant` values are
   unchanged — the state→token chain is untouched):
   - `"\u2713 read f.go"` → `"← f.go"`
   - `"\u25B6 bash ls -la"` → `"~ Writing command..."`
   - `"\u2717 grep no match"` → `"✱ grep"` (the error text moves to the
     expansion — the row carries the title, as upstream `failure ??
     children` resolves to the title when `failure` is unset)
   - the call: `toolRowLine(tt.part, th)` → `toolRow(tt.part, th, "")`
2. `TestSessionChromeThemeSGR` (the tool NAME is gone — anchor on the
   icon + title):
   - `completedRowRe`: `m✓ read` → `m← hello.txt` (38;5;244 CSI; the
     engine title for the read part is the relative path "hello.txt")
   - the dialog-drain text check `strings.Contains(s, "\u2713 read")` →
     `strings.Contains(s, "← hello.txt")`
   - `errorRowRe`: `m✗ bash` → `m$ echo hi` (38;5;246 CSI; the rejected
     bash row is an ERROR row: icon `$` + title "echo hi")
   - the reject-drain text check `strings.Contains(s, "\u2717 bash")` →
     `strings.Contains(s, "$ echo hi")`
   - the color-token sets are unchanged (the running bash row at the
     dialog drain is now the "~ Writing command..." text row — it still
     carries 38;5;255)
3. `TestSessionChromeZeroThemeIsPlain` want block: the three tool rows
   become `"← src/main.go\n"`, `"~ Writing command...\n"`, `"✱ grep\n"`
   (the `Thinking\n` line from S1.6 stands; the `!` line re-baselines in
   S1.8).

- [ ] **Step 2: Confirm the tests FAIL**

```sh
go test ./internal/tui/ -run 'TestToolRow' -v
```

- [ ] **Step 3: Write the minimal implementation**

`internal/tui/session.go` — replace `toolRowLine` (+ keep
`toolTitleFallback`):

```go
// toolGlyph is the per-tool icon (upstream InlineTool icon props,
// index.tsx:2105-2545 — the 2-column slot is the glyph + space).
func toolGlyph(tool string) string {
	switch tool {
	case "bash":
		return "$"
	case "write", "edit":
		return "←"
	case "glob", "grep":
		return "✱"
	case "read":
		return "→"
	default:
		return "⚙"
	}
}

// toolPending is the upstream pending text (the running row, index.tsx
// pending= props).
func toolPending(tool string) string {
	switch tool {
	case "bash":
		return "Writing command..."
	case "write":
		return "Preparing write..."
	case "edit":
		return "Preparing edit..."
	case "glob":
		return "Finding files..."
	case "grep":
		return "Searching content..."
	case "read":
		return "Reading file..."
	case "todowrite":
		return "Updating todos..."
	default:
		return "Working..."
	}
}

// toolFailure is the upstream failure= prop (the error row text when the
// part has no title).
func toolFailure(tool string) string {
	if tool == "todowrite" {
		return "Todo update failed"
	}
	return ""
}

// toolRow renders the upstream InlineTool row: the running row is "~
// <pending>" at fg=text (read: "<spin> Reading file..."), the completed
// row "<icon> <title>" at fg=textMuted, the error row "<icon>
// <failure ?? title>" at fg=error (index.tsx:1882-1889, 1966-1990).
// A zero Theme degrades to plain rows (the S0.10 contract).
func toolRow(p protocol.Part, th theme.Theme, spin string) (lipgloss.Style, string, bool) {
	st := p.State
	status := "running"
	title := ""
	if st != nil {
		status = st.Status
		title = st.Title
	}
	if title == "" {
		title = toolTitleFallback(p)
	}
	icon := toolGlyph(p.Tool) + " "
	switch status {
	case "completed":
		return th.TextMuted(), icon + title, true
	case "error":
		text := toolFailure(p.Tool)
		if text == "" {
			text = title
		}
		return th.Error(), icon + text, true
	default:
		// read: the upstream spinner row (spinner={isRunning()}); a
		// zero-spin caller (zero-theme/unit runs) degrades to "~".
		if p.Tool == "read" && spin != "" {
			return th.Text(), spin + " " + toolPending("read"), true
		}
		return th.Text(), "~ " + toolPending(p.Tool), true
	}
}
```

The tool `case` in `renderAssistant`:

```go
		case "tool":
			sty, row, ok := toolRow(p, th, spin)
			if !ok {
				continue
			}
			writeStyled(sty, row)
			switch {
			case expanded[p.ID] && p.State != nil && p.State.Status == "error":
				// The upstream expanded error (InlineToolRow 1992-1999):
				// the FULL error at the icon width (2), fg=error.
				if p.State.Error != "" {
					for _, l := range strings.Split(p.State.Error, "\n") {
						writeStyled(th.Error(), "  "+l)
					}
				}
			case expanded[p.ID] && p.State != nil:
				block := tailLines(p.State.Output, 40)
				if block == "" {
					continue
				}
				for _, l := range strings.Split(block, "\n") {
					writePlain("  " + l)
				}
			case p.Tool == "bash" && p.State != nil && p.State.Status == "completed":
				// Inline preview (S0 lock): the 10-line head.
				if block := headPreview(p.State.Output, 10); block != "" {
					for _, l := range strings.Split(block, "\n") {
						writePlain("  "+l)
					}
				}
			}
```

- [ ] **Step 4: Run to verify it passes, then gate**

```sh
go test ./internal/tui/ -run 'TestToolRow|TestSessionChrome' -v
go vet ./... && go test ./... && gofmt -l .
```

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/session.go internal/tui/session_toolrow_test.go internal/tui/session_theme_test.go
git commit -m "feat: tool-row restyle (glyphs + expand)"
bd close yolo-oae.2.7 --reason "per-tool glyphs + pending text (read spinner), failure-text error rows, icon-width error expansion; S0.10 unit+SGR goldens re-baselined" --json
```

**STOP**: report the gate result, the commit, `git status`.

### Task S1.8: Error parts + toast restyle (theme tokens) + tests (`yolo-oae.2.8`)

**Files:**
- Modify: `internal/tui/session.go` — the message-error `!` line → the
  upstream error BOX (left border, panel bg, padding, textMuted text);
  the aborted case → the muted single line.
- Modify: `internal/tui/toast.go` — `toastsView` → the upstream toast
  chrome (left+right error border, panel bg, padding, the `• msg` line
  keeps the red text — LOCKED), gated on `!a.theme.Zero()` so the
  zero-engine toast tests (`TestToastsViewWraps`, `TestToastQueueCapAnd
  Order` — exact/line-count pins) stay green UNCHANGED.
- Modify: `internal/tui/session_theme_test.go` — the
  `TestSessionChromeZeroThemeIsPlain` want block: `"! something broke"` →
  `"something broke"` (the zero-theme degradation, below).
- Create: `internal/tui/session_error_test.go` — the unit box-shape test +
  the chrome style-accessor test (no teatest — see Step 1 note).

**Interfaces:**
- Consumes: S0.3 `th.Error()`/`th.TextMuted()`/`th.BackgroundPanel()`
  (lipgloss styles); `protocol.MessageError{Type, Message}` (Type ∈
  {"unknown","aborted","overflow"}).
- Produces: the message-error box + the toast chrome.

**Upstream parity notes (binding)** (finding 7):
- The assistant error box (index.tsx:1534-1548): rendered ONLY when
  `error.name !== "MessageAbortedError"` — yolo analog: `Type !=
  "aborted"`. Chrome: `border=["left"]` (a single left border line),
  `borderColor=theme.error`, `backgroundColor=theme.backgroundPanel`,
  `paddingTop=1 paddingBottom=1 paddingLeft=2`, `marginTop=1`; the message
  text is `fg=theme.textMuted` (the BORDER is the only error-colored part).
  yolo today: `! <message>` in `th.Error()` (the S0.10 line) — REPLACED.
  lipgloss shape: `lipgloss.NewStyle().Border(lipgloss.NormalBorder(),
  false, true, false, false).BorderForeground(th.Error().GetForeground
  (the error hex)).Background(backgroundPanel hex).Padding(1, 0, 1, 2)` —
  implement via a small `errorBoxStyle(th theme.Theme) lipgloss.Style`
  helper in session.go (the bg/fg hexes come from `th.Color("error")` /
  `th.Color("backgroundPanel")` → `hex6`).
- The ABORTED case (`Type == "aborted"`): the upstream route renders the
  "esc abort/back"-style continuation, NOT the box — yolo: the muted
  single line `~ <message>` in `th.TextMuted()` (the S0.10 `!` line's
  muted analog; the LOCKED red does not apply to aborted runs — the user
  caused them).
- The toast (ui/toast.tsx:22-52): `border=["left","right"]`,
  `borderColor=theme[variant]`, `backgroundColor=backgroundPanel`,
  `paddingLeft/Right=2 paddingTop/Bottom=1`, message `fg=theme.text`.
  yolo's toast block is ERROR-ONLY (LOCKED red — the S0.10 note stands:
  for the error-only block the variant color IS the error token), so the
  restyle adds the chrome (left+right error border, panel bg, 2/1
  padding) and keeps the `• <msg>` line in `th.Error()`. Multi-line: each
  wrapped line is a border row (lipgloss renders the border around the
  whole padded block).
- SGR tokens under xterm-256color (the S0.10 derivations): error
  #e06c75 → **246**, backgroundPanel #141414 → **233** (background:
  `48;5;233`).

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/session_error_test.go`:

```go
package tui

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
	"github.com/kido5217/yolo/internal/tui/client"
	"github.com/kido5217/yolo/internal/tui/store"
	"github.com/kido5217/yolo/internal/tui/theme"
)

// TestMessageErrorBox pins the S1.8 box SHAPE at the render level (pure
// render, no renderer): the non-aborted message error is the left-border
// box (the "│" border line is structural — it survives the non-TTY
// strip, so its presence is assertable); the aborted error is the muted
// "~ <message>" line with no border. Colors are NOT asserted here —
// without TTY_FORCE lipgloss strips SGR in a unit test — they are pinned
// through the style constructor in TestMessageErrorBoxStyle.
func TestMessageErrorBox(t *testing.T) {
	all, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(all["opencode"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := theme.Theme{R: r, Name: "opencode", Mode: "dark"}

	// non-aborted: the box (border + message).
	out := renderMessageError(protocol.MessageError{Type: "unknown", Message: "boom"}, th, 77)
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "boom") {
		t.Errorf("box missing the message: %q", stripped)
	}
	if !strings.Contains(stripped, "│") {
		t.Errorf("box missing the left border: %q", stripped)
	}
	// aborted: the muted line, no border.
	out = renderMessageError(protocol.MessageError{Type: "aborted", Message: "user interrupted"}, th, 77)
	stripped = stripANSI(out)
	if strings.Contains(stripped, "│") {
		t.Errorf("aborted must not box: %q", stripped)
	}
	if !strings.Contains(stripped, "~ user interrupted") {
		t.Errorf("aborted line = %q, want '~ user interrupted'", stripped)
	}
	// zero Theme: the bare plain message (S0.7 degradation), no border.
	out = renderMessageError(protocol.MessageError{Type: "unknown", Message: "boom"}, theme.Theme{}, 77)
	if got := stripANSI(out); got != "boom" {
		t.Errorf("zero-theme = %q, want bare \"boom\"", got)
	}
}

// TestMessageErrorBoxStyle pins the box CHROME via the style constructor's
// accessors (the S0.10 `fgWant` mechanism): left border only, in the error
// token; the panel background; the 1/2 padding; textMuted text.
func TestMessageErrorBoxStyle(t *testing.T) {
	all, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(all["opencode"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := theme.Theme{R: r, Name: "opencode", Mode: "dark"}
	st := messageErrorBoxStyle(th)
	_, top, right, bottom, left := st.GetBorder()
	if !left || top || right || bottom {
		t.Errorf("border = (top %v, right %v, bottom %v, left %v), want left only", top, right, bottom, left)
	}
	if got, want := st.GetBorderLeftForeground(), hexColor("#e06c75"); got != want {
		t.Errorf("border fg = %v, want error %v", got, want)
	}
	if got, want := st.GetBackground(), hexColor("#141414"); got != want {
		t.Errorf("bg = %v, want backgroundPanel %v", got, want)
	}
	if pt, pr, pb, pl := st.GetPadding(); pt != 1 || pr != 0 || pb != 1 || pl != 2 {
		t.Errorf("padding = (%d,%d,%d,%d), want (1,0,1,2)", pt, pr, pb, pl)
	}
}
```

(No teatest golden this task: the message-error box is fed by
`m.Info.Error`, which NO current test driver populates end-to-end (the
fake scripted turns complete cleanly), so a live teatest would need a new
driver seam — out of S1 scope. The shape is pinned by `TestMessageErrorBox`
and the chrome by `TestMessageErrorBoxStyle` against the same constructor
`renderMessageError` uses; the toast chrome is separately pinned by the
existing `TestToasts*` goldens.)

- [ ] **Step 2: Run to verify it fails**

```sh
go test ./internal/tui/ -run 'TestMessageErrorBox' -v
```

Expect FAIL — `renderMessageError` undefined (the `!` line is still inline
in `renderAssistant`).

- [ ] **Step 3: Write the minimal implementation**

`internal/tui/session.go` — replace the message-error tail of
`renderAssistant` (`writeStyled(th.Error(), "! "+m.Info.Error.Message)`
becomes):

```go
	if m.Info.Error != nil {
		writeRaw(renderMessageError(*m.Info.Error, th, w))
	}
	return b.String()
}

// rgbaHex is the 6-digit hex of a resolved token (the theme package's
// hex6 stays unexported — the TUI keeps its own one-liner; the surface
// stays S1.2/S1.4-locked).
func rgbaHex(c theme.Rgba) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// renderMessageError renders the assistant message error: the upstream
// error BOX for non-aborted errors (index.tsx:1534-1548 — left border in
// the error token, the panel background, 1/2 padding, textMuted text) and
// the muted "~ <message>" line for aborted runs (upstream renders no box
// for MessageAbortedError — the user caused them). A zero Theme degrades
// to the bare plain message (the S0.7 contract: no SGR, never a panic).
func renderMessageError(e protocol.MessageError, th theme.Theme, w int) string {
	if th.Zero() {
		return e.Message
	}
	if e.Type == "aborted" {
		return th.TextMuted().Render("~ " + e.Message)
	}
	// word-wrap the message at the inner width before boxing (the box
	// does not wrap).
	var lines []string
	inner := w - 4 // border(1) + paddingLeft(2) + margin(1)
	for _, l := range strings.Split(e.Message, "\n") {
		lines = append(lines, wrapLine(l, max(1, inner))...)
	}
	return messageErrorBoxStyle(th).Render(th.TextMuted().Render(strings.Join(lines, "\n")))
}

// messageErrorBoxStyle builds the error box chrome (the S1.8 test pins it
// through the lipgloss accessors): a single left border line in the error
// token over the panel background, padded 1/0/1/2.
func messageErrorBoxStyle(th theme.Theme) lipgloss.Style {
	box := lipgloss.NewStyle().
		Border(leftOnlyBorder(), false, true, false, false).
		Padding(1, 0, 1, 2)
	if c, ok := th.Color("error"); ok {
		box = box.BorderForeground(rgbaHex(c))
	}
	if c, ok := th.Color("backgroundPanel"); ok {
		box = box.Background(rgbaHex(c))
	}
	return box
}

func leftOnlyBorder() lipgloss.Border {
	return lipgloss.Border{
		Left: "│", // all other edges empty: a single left border line
	}
}
```

`internal/tui/toast.go` — `toastsView` gains the chrome (import gains
`lipgloss`; `rgbaHex` comes from session.go, same package):

```go
func (a *App) toastsView(w int) string {
	if len(a.toasts) == 0 {
		return ""
	}
	// Zero Theme (nil-engine runs): the LOCKED plain red lines, unchanged
	// (the zero-engine toast tests pin the exact/line form).
	if a.theme.Zero() {
		var b strings.Builder
		for i := len(a.toasts) - 1; i >= 0; i-- { // newest on top (LOCKED)
			if i != len(a.toasts)-1 {
				b.WriteByte('\n')
			}
			for j, l := range strings.Split(wrapLine("\u2022 "+a.toasts[i].msg, w), "\n") {
				if j > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(a.theme.Error().Render(l))
			}
		}
		return b.String()
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, true).
		Padding(1, 2, 1, 2)
	if c, ok := a.theme.Color("error"); ok {
		box = box.BorderForeground(rgbaHex(c))
	}
	if c, ok := a.theme.Color("backgroundPanel"); ok {
		box = box.Background(rgbaHex(c))
	}
	var lines []string
	for i := len(a.toasts) - 1; i >= 0; i-- { // newest on top (LOCKED)
		for _, l := range strings.Split(wrapLine("\u2022 "+a.toasts[i].msg, max(1, w-6)), "\n") {
			lines = append(lines, l)
		}
	}
	return box.Render(a.theme.Error().Render(strings.Join(lines, "\n")))
}
```

(The zero-Theme early-return keeps the zero-engine toast goldens byte
identical — `TestToastsViewWraps` / `TestToastQueueCapAndOrder` stay
green unchanged.)

- [ ] **Step 4: Run to verify it passes, then gate**

```sh
go test ./internal/tui/ -run 'TestMessageErrorBox|TestToasts' -v
go vet ./... && go test ./... && gofmt -l .
```

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/session.go internal/tui/toast.go internal/tui/session_error_test.go
git commit -m "feat: error parts + toast restyle (theme tokens)"
bd close yolo-oae.2.8 --reason "upstream error box (left border, panel bg, textMuted) + muted aborted line; toast chrome (error border, panel bg, padding); LOCKED red text kept" --json
```

**STOP**: report the gate result, the commit, `git status`.

### Task S1.9: Re-render benchmark on batched delta + budget gate (extends `session_bench_test.go`) (`yolo-oae.2.9`)

**Files:**
- Modify: `internal/tui/session_bench_test.go` — the 100 KB part
  re-render benchmark + the budget gate (spec §4).
- Modify: `internal/tui/session_test.go` (or the bench file) — the gate
  assertion as a test (a budget gate must FAIL the suite on regression,
  not just print).

**Interfaces:**
- Consumes: the S1.3/S1.4/S1.6 render path (`renderMessages` over a store
  state with a 100 KB part); the existing `benchStore` harness (the
  S0-era bench file's fixture builder).
- Produces: `BenchmarkRenderMessages_100KBPart` + `TestRenderMessages100KBBudget`
  (min-of-5 after 3 warmups < 50 ms — spec §4).

**Budget derivation (pinned):** spec §4: "a 100 KB part re-render must
stay under 50 ms" (the frame budget for a streaming delta batch). Measured
at detail time on the dev box (scratch module, glamour v2.0.1, opencode
dark, a 100426-byte part with a 20 KB fenced code block): one
`NewTranscriptRenderer` + `Render` ≈ 22 ms; construct ≈ 20–50 µs. The
gate asserts < 50 ms with ≥ 2× headroom — if the CI box is slower, the
gate value (not the code) is the knob: re-baseline with a DEVIATIONS.md
note if the user calls it.

- [ ] **Step 1: Write the failing test (the budget gate) + the benchmark**

Append to `internal/tui/session_bench_test.go`:

```go
// bigPartState wraps one part in the benchStore message envelope (one
// assistant message, the S0-era fixture shape) — the existing
// benchStore(n, expanded) builds per-message 5-part fixtures by count and
// cannot carry a sized part, so the 100 KB case builds its state directly.
func bigPartState(p protocol.Part) *store.State {
	return &store.State{Messages: []protocol.MessageWithParts{{
		Info:  protocol.Message{ID: "msg_big", SessionID: "ses_bench", Role: "assistant", Agent: "build"},
		Parts: []protocol.Part{p},
	}}}
}

// hundredKBPart builds the spec §4 fixture: a ~100 KB text part (prose +
// a fenced code block) — the worst-case re-render input for the S1.3/S1.4
// path.
func hundredKBPart() protocol.Part {
	var b strings.Builder
	b.WriteString("Here is a long analysis.\n\n")
	b.WriteString("```go\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n")
	for i := 0; i < 1800; i++ {
		fmt.Fprintf(&b, "\tfmt.Printf(\"line %04d: the quick brown fox jumps over the lazy dog\")\n", i)
	}
	b.WriteString("}\n```\n\n")
	for i := 0; i < 600; i++ {
		b.WriteString("A paragraph of supporting prose that wraps across several terminal lines and carries **bold** and `inline` spans for the renderer to style.\n\n")
	}
	text := b.String()
	if len(text) < 100*1024 {
		// pad to the 100 KB spec size (the gate is about the input size).
		text += strings.Repeat("padding prose to reach the spec size.\n", (100*1024-len(text))/44+1)
	}
	return protocol.Part{ID: "big", Type: "text", Text: text}
}

// TestRenderMessages100KBBudget is the spec §4 budget gate: re-rendering a
// 100 KB part (the streamed-delta batch case) must stay under 50 ms —
// measured as the min of 5 renders after 3 warmups (the renderer is built
// once per renderMessages call, S1.3; the gate measures the RENDER, the
// steady streaming cost).
func TestRenderMessages100KBBudget(t *testing.T) {
	all, err := theme.AllThemes()
	if err != nil {
		t.Fatalf("AllThemes: %v", err)
	}
	r, err := theme.ResolveTheme(all["opencode"], "dark")
	if err != nil {
		t.Fatalf("ResolveTheme: %v", err)
	}
	th := theme.Theme{R: r, Name: "opencode", Mode: "dark"}
	st := bigPartState(hundredKBPart())
	const (
		warmups = 3
		samples = 5
		budget  = 50 * time.Millisecond
	)
	var best time.Duration
	for i := 0; i < warmups+samples; i++ {
		start := time.Now()
		_ = renderMessages(st, nil, 80, th, "")
		if i >= warmups {
			if d := time.Since(start); i == warmups || d < best {
				best = d
			}
		}
	}
	if best >= budget {
		t.Fatalf("100 KB re-render = %v, budget %v (spec §4)", best, budget)
	}
}

// BenchmarkRenderMessages_100KBPart is the standing measurement (the gate
// above is the CI assertion; this tracks drift in `go test -bench`).
func BenchmarkRenderMessages_100KBPart(b *testing.B) {
	all, _ := theme.AllThemes()
	r, _ := theme.ResolveTheme(all["opencode"], "dark")
	th := theme.Theme{R: r, Name: "opencode", Mode: "dark"}
	st := bigPartState(hundredKBPart())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderMessages(st, nil, 80, th, "")
	}
}
```

(`bigPartState` mirrors the `benchStore(n, expanded)` message envelope —
the S0 harness builds count-sized 5-part fixtures and cannot carry a sized
part. The `renderMessages(st, nil, 80, th, "")` spin arg is the S1.6
signature; the bench file gains `time` to its imports.)

- [ ] **Step 2: Run to verify it fails (or measure)**

```sh
go test ./internal/tui/ -run TestRenderMessages100KBBudget -v
go test ./internal/tui/ -bench BenchmarkRenderMessages_100KBPart -benchtime 10x -run XXX
```

Expect: FAIL if the current (pre-S1.9) path is over budget — which it will
NOT be after S1.3–S1.6 (the render path already exists); the test's real
job is to FAIL on a FUTURE regression (e.g. a renderer rebuild per part, a
lost wrap). If it passes immediately, that IS the confirmed baseline
(~22 ms at detail time) — report the measured value in the close reason.

- [ ] **Step 3: Minimal implementation**

No production code — the task IS the measurement + the gate. If Step 2
shows the budget missed (a real regression): profile with
`go test -benchmem`, check the renderer construct cost per call (it must
stay one-per-renderMessages, not per-part), and fix the allocation before
closing.

- [ ] **Step 4: Run to verify it passes, then gate**

```sh
go test ./internal/tui/ -run TestRenderMessages100KBBudget -v
go test ./internal/tui/ -bench BenchmarkRenderMessages_100KBPart -benchtime 5x -run XXX
go vet ./... && go test ./... && gofmt -l .
```

- [ ] **Step 5: Commit + close the bead**

```sh
git add internal/tui/session_bench_test.go
git commit -m "perf: transcript re-render benchmark + budget gate"
bd close yolo-oae.2.9 --reason "100KB part re-render benchmarked (measured ~N ms, min-of-5 < 50 ms budget gate live)" --json
```

**STOP**: report the gate result, the measured value, the commit, `git status`.

## S1 slice gate (slice bead `yolo-oae.2`)

NOT a task bead; runs after all child beads close. Mirror the S0 slice gate
shape: (1) module gate `go vet ./... && go test ./...` + `gofmt -l .` empty
(incl. `TestImportsDirection` + the S1 teatest goldens); (2) user-run smoke
(NOT CI): in a real TTY, render a transcript with a fenced code block, a
table, a task list, and reasoning — theme-colored markdown, syntax-highlighted
code, and the reasoning/tool/error surfaces restyled; (3) append any forced
DEVIATIONS.md entries this slice named (with severity, same-commit rule —
root principle 2; spec §4: the transcript-fixture pty diff runs before the
slice closes — gaps become per-element `StyleConfig` overrides or a logged
custom renderer); (4) PROGRESS.md one-line status pointer; (5) commit
`docs: checkpoint — S1 done, next is S2 detail pass`; (6)
`bd close yolo-oae.2 --reason "all 9 child beads closed, gate green" --json`.
