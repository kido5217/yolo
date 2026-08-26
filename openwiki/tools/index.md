# Files

- [Permission Engine](permissions.md) - internal/permission: the ported decision engine (findLast rule evaluation), the build/plan/yolo builtin matrices, doom-loop detection, and the blocking Service that gates session tool execution, persists requests, and parks asks until the TUI replies once/always/reject.
- [Toolset](toolset.md) - internal/tool built-in tools (read, write, edit, glob, grep, bash, todowrite): the Tool interface and registry, permission-based visibility, the per-session persistent bash shell, output truncation with the full output persisted to <data>/tool-output (7-day retention, startup sweep), and the sha256-pinned desc/*.txt descriptions.
