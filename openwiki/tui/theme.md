---
type: reference
title: Theme Engine
description: "internal/tui/theme: the TUI-local theme engine — 33 embedded upstream theme assets, the config > KV > default selection chain and dark/light mode resolution, OSC-based terminal palette discovery (raw-mode /dev/tty), system-theme generation, custom discovery from .yolo/themes, lipgloss style generation, and the glamour GFM transcript renderer."
tags: [theme, lipgloss, glamour, chroma, palette, osc, discovery, selection, markdown]
verified:
  - by: openwiki/0.4.0
    at: 2026-08-26T18:04:14.871Z
sources:
  - id: openwiki-source-028d026d26511c1f750984fb
    resource: repo://internal/tui/theme/discover.go
  - id: openwiki-source-e0d28d47f0b0d7d2450a0f0d
    resource: repo://internal/tui/theme/embed.go
  - id: openwiki-source-0dc5d1a5f5010d918a777b19
    resource: repo://internal/tui/theme/engine.go
  - id: openwiki-source-954680b87bfc6228e70f7194
    resource: repo://internal/tui/theme/palette.go
  - id: openwiki-source-dce39dde299f8db7fc1419d9
    resource: repo://internal/tui/theme/resolve.go
  - id: openwiki-source-d0216349c9e0aeab0faa3bc0
    resource: repo://internal/tui/theme/styles.go
  - id: openwiki-source-371ed13c1b640a7af2fa1258
    resource: repo://internal/tui/theme/syntax_test.go
  - id: openwiki-source-3069fa4c7f1a198e3d38723a
    resource: repo://internal/tui/theme/syntax.go
  - id: openwiki-source-03973d46c62210cb4574d233
    resource: repo://internal/tui/theme/system.go
  - id: openwiki-source-2325305be7fade63b9401bc2
    resource: repo://internal/tui/theme/theme.go
generated: {by: "opencode", at: "2026-08-26T18:04:14.871Z"}
---

# Theme Engine

`internal/tui/theme` is the **TUI-local theme engine** (root principle 4: it
imports nothing outside `internal/tui`, so every filesystem path is injected by
`cmd/yolo`). It provides the **33 embedded upstream themes**, **resolution**,
**system-theme generation**, **terminal palette detection**, **custom
discovery**, and the **selection chain** (config > KV > default).

## Embedded assets

The **33 upstream theme assets** are embedded verbatim (`//go:embed
assets/*.json` → `assetsFS`, the strict-copy bar of spec §1). `AllThemes`
parses them; names are the asset file stems with kebab-case preserved
(`catppuccin-frappe`, `one-dark`, …). `DefaultName` is `"opencode"`. `IsTheme`
is the upstream check (a non-array object with a non-array object `theme`
member).

## The Engine: selection chain

`Engine` is the theme selection store: **`active = config > KV "theme" >
"opencode"`** and **`mode = KV lock > terminal luminance > "dark"`**
(`engine.go`). `New` mirrors the upstream init block: `lock =
theme_mode_lock`; `mode = lock ?? "dark"`; the one-shot `theme_mode` is cleared
when unlocked (only when it holds a valid mode); `active = config > KV theme >
"opencode"`. The KV is loaded synchronously so the first Get is race-free.

`Resolve` runs the startup sequence (the upstream `onMount`:
`resolveSystemTheme` + `syncCustomThemes`, ported sequentially): the **system
theme first** — the **palette is probed exactly here, exactly once** (the S0
scoping rule) — then custom discovery. It always returns nil (both upstream
catch paths are swallowed). Mode re-resolves as `lock ?? terminalMode(colors) ??
mode`; an empty `palette[0]` or a failed probe with `active == "system"` falls
back to the default.

`ActiveTheme` is the upstream values memo: `themes[active] ?? themes[KV
"theme"] ?? themes[opencode]`, where `themesMap` = builtins + customs +
(optional) `"system"` (priority defaults < custom < system: later entries win).
`themesFromRaw` filters the raw custom map to theme objects (`IsTheme`) and
round-trips each to a `ThemeJson`.

Mutations: **`Set`** switches active and persists to KV `"theme"` (unknown name
→ false, KV untouched); **`Pin`** sets the mode lock and applies it; **`Free`**
clears the lock + one-shot KV keys and re-resolves the mode from the **cached**
terminal luminance (no re-probe); **`Apply`** switches the mode and persists to
KV `"theme_mode"` **only while locked**, regenerating the system theme at the
new mode from the cached palette.

### The two-leg refresh

- **`Reapply`** is the **250 ms leg**: regenerates the system theme at the
  current mode from the cached palette (the upstream `refreshSystemTheme`, minus
  the palette cache clear + re-probe).
- **`RefreshCustoms`** is the **1000 ms leg**: re-discovers customs; on error it
  takes the catch path — `active = "opencode"`, customs emptied (the custom set
  is derived state, never persisted).

## Terminal palette discovery

`DetectPalette` ports `TerminalPalette.detect`: **(1)** an **OSC `4;0` support
probe**, then **(2)** the **16 palette + 9 special-color queries** (indices
10–17, 19) with per-group idle timers, a hard timeout, and an 8192→keep-last-
4096 buffer cap; stored slots are first-wins. The response shapes, `toHex8` and
`scaleComponent` are verbatim ports of `@opentui/core 0.4.5`. **`DetectStd`**
probes via an **owned `/dev/tty` in raw mode** (`x/term`): the fd is restored
and closed on exit and the pump goroutine is **joined** (close alone does not
wake a kernel-blocked read), so no reader lingers; **no controlling terminal →
`(TerminalColors{}, false)`** (no system theme, no probe, no goroutines).
`LegacyTmux` (TMUX set && TMUX_PANE unset) double-escapes OSC and limits the
special queries to 10–12. Timeouts are spec-pinned to 100 ms.

## Custom discovery

`ThemeDirs` is the upstream directory list: the **injected global config dir
first**, then **`<dir>/.yolo`** for every dir from cwd up to and including the
filesystem root (no dedupe; later-dir-wins). `Discover` scans each
`<dir>/themes/` for `*.json` (dotfiles included, symlinks followed); name = base
minus `.json`; later dirs override earlier; a missing themes dir is skipped, but
an unreadable or unparseable file is a **hard error**; values are returned RAW
(the `IsTheme` filter is the caller's job). `WatchThemeSignals` forwards
**SIGUSR2 → refresh** (the 250/1000 ms debounce lives in the engine).

## Resolution and system theme

`ResolveTheme` ports the upstream resolver: **defs refs, `#hex`,
`transparent`/`none`, ANSI ints, and `{dark,light}` variants**; optional
`selectedListItemText` (default: background), `backgroundMenu` (default:
`backgroundElement`), and `thinkingOpacity` (default: 0.6); circular- and
missing-reference errors keep the upstream wording. `Rgba` is stored as
**0–255 uint8** — upstream RGBA is float 0–1, but every color is int-derived or
produced by float ops on 0–255 values rounded at the end, so uint8 is exact and
the operation order is preserved for **bit-identical results** (strict-copy
bar).

`GenerateSystem` turns the terminal palette + default fg/bg into a generated
`ThemeJson` (values are `Rgba`, mirroring upstream's RGBA-instance values;
missing palette entries fall back to the ANSI table, missing default bg/fg to
`palette[0]`/`palette[7]`). `TerminalMode` is the luminance rule (bg luminance
> 0.5 → `"light"`, else `"dark"`).

## lipgloss styles

`Theme` is a resolved theme + name/mode, **exposing lipgloss styles — components
never see hex** (spec §3). `fg`/`bg` yield an empty style for an absent or
transparent (alpha 0) token. `SelectedForeground` is the upstream contrast rule:
explicit `selectedListItemText` wins; a transparent background contrasts via the
luminance rule (`0.299r+0.587g+0.114b > 0.5 → black`, else white); else the
background. The token accessors (`Text`, `TextMuted`, `Primary`, …, `Markdown*`,
`Syntax*`, `Diff*`) map each semantic token to a lipgloss style.

## Transcript rendering (glamour)

`StyleConfig` builds the **glamour element styles** from the `markdown*` tokens
— `base` is `"markdownText"` for text parts and `"textMuted"` for reasoning —
with the word-wrap width also driving the horizontal-rule line length
(`<4` → the 8-dash fallback). `NewTranscriptRenderer` builds a glamour
`TermRenderer` (word-wrapped at `width`); a **zero Theme** (a nil-engine run)
skips `WithStyles`, so glamour renders plain with no SGR. The SGR profile is
verified: in a plain unit context glamour's plain text is 24-bit while the
code-block path is 256-color, but **under teatest glamour emits 256-color for
both**, so teatest goldens pin `38;5;N`.

The **chroma token map + the global `"charm"` style-slot workaround** for
per-language code highlighting is the **planned S1.4 work** (the `glamour` /
`glamour/ansi` / `chroma/v2/styles` allowlist entry); in the current tree
**`CodeBlock.Chroma` is still nil** (pinned by `syntax_test.go`).
