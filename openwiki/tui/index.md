# Files

- [TUI Application](app.md) - internal/tui: the bubbletea v2 App model with home/session routes, the Update loop and SSE event/resync pumps, the in-memory store (REST hydration + SSE Apply with a per-part delta fast path), the HTTP/SSE client over the wire contract, the key ladder, dialogs, toasts, and the import-purity contract enforced by TestImportsDirection.
- [Theme Engine](theme.md) - internal/tui/theme: the TUI-local theme engine — 33 embedded upstream theme assets, the config > KV > default selection chain and dark/light mode resolution, OSC-based terminal palette discovery (raw-mode /dev/tty), system-theme generation, custom discovery from .yolo/themes, lipgloss style generation, and the glamour GFM transcript renderer.
