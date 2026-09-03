package tui

import (
	"context"
	"encoding/json"
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

// todowritePart builds the wire-shape todowrite part: the JSON-decoded
// State.Input (the []any of map[string]any).
func todowritePart(todos ...map[string]any) protocol.Part {
	in := make([]any, 0, len(todos))
	for _, td := range todos {
		in = append(in, td)
	}
	return protocol.Part{
		Type:  "tool",
		Tool:  "todowrite",
		State: &protocol.ToolState{Input: map[string]any{"todos": in}},
	}
}

// todoTodos is the 3-status fixture (completed / in_progress / pending).
func todoTodos(t *testing.T) []protocol.Todo {
	t.Helper()
	return []protocol.Todo{
		{Content: "design", Status: "completed", Priority: "high"},
		{Content: "implement the todo sidebar", Status: "in_progress", Priority: "medium"},
		{Content: "wire the toggle", Status: "pending", Priority: "low"},
	}
}

// TestTodosFromPartWireShape pins the decode over both the wire-decoded
// State.Input (the []any of map[string]any) and the in-memory
// State.Metadata (the []protocol.Todo); the Input wins when both are
// present; an undecodable value is !ok.
func TestTodosFromPartWireShape(t *testing.T) {
	wire := []any{
		map[string]any{"content": "a", "status": "in_progress", "priority": "high"},
		map[string]any{"content": "b", "status": "pending"},
	}
	got, ok := todosFromPart(protocol.Part{Type: "tool", Tool: "todowrite",
		State: &protocol.ToolState{Input: map[string]any{"todos": wire}}})
	if !ok || len(got) != 2 || got[0].Content != "a" || got[0].Status != "in_progress" || got[0].Priority != "high" {
		t.Fatalf("wire input decode = %+v (ok=%v), want the 2 todos", got, ok)
	}
	mem, ok := todosFromPart(protocol.Part{Type: "tool", Tool: "todowrite",
		State: &protocol.ToolState{Metadata: map[string]any{"todos": []protocol.Todo{{Content: "c", Status: "completed"}}}}})
	if !ok || len(mem) != 1 || mem[0].Content != "c" {
		t.Fatalf("metadata decode = %+v (ok=%v), want the 1 todo", mem, ok)
	}
	both, ok := todosFromPart(protocol.Part{Type: "tool", Tool: "todowrite",
		State: &protocol.ToolState{
			Input:    map[string]any{"todos": wire},
			Metadata: map[string]any{"todos": []protocol.Todo{{Content: "c"}}},
		}})
	if !ok || len(both) != 2 || both[1].Content != "b" {
		t.Fatalf("input-first decode = %+v (ok=%v), want the input's 2 todos", both, ok)
	}
	if _, ok := todosFromPart(protocol.Part{Type: "tool", Tool: "todowrite",
		State: &protocol.ToolState{Input: map[string]any{"todos": "not-a-list"}}}); ok {
		t.Fatal("an undecodable todos value must be !ok")
	}
}

// TestLatestTodos pins the last-wins referent: the LAST todowrite part
// with a decodable list wins; non-todowrite tool parts and undecodable
// parts are skipped (no shadow); an empty store yields nil.
func TestLatestTodos(t *testing.T) {
	if latestTodos(&store.State{}) != nil {
		t.Fatal("an empty store must be nil")
	}
	a := todowritePart(map[string]any{"content": "first", "status": "pending"})
	b := todowritePart(map[string]any{"content": "second", "status": "in_progress"})
	latest := latestTodos(&store.State{Messages: []protocol.MessageWithParts{
		{Info: protocol.Message{ID: "m1"}, Parts: []protocol.Part{
			{Type: "tool", Tool: "bash",
				State: &protocol.ToolState{Input: map[string]any{"todos": []any{map[string]any{"content": "not-me"}}}}},
			a,
		}},
		{Info: protocol.Message{ID: "m2"}, Parts: []protocol.Part{b}},
	}})
	if len(latest) != 1 || latest[0].Content != "second" {
		t.Fatalf("latest = %+v, want the last todowrite's todo", latest)
	}
	bad := protocol.Part{Type: "tool", Tool: "todowrite",
		State: &protocol.ToolState{Input: map[string]any{"todos": "nope"}}}
	latest2 := latestTodos(&store.State{Messages: []protocol.MessageWithParts{
		{Info: protocol.Message{ID: "m1"}, Parts: []protocol.Part{a}},
		{Info: protocol.Message{ID: "m2"}, Parts: []protocol.Part{bad}},
	}})
	if len(latest2) != 1 || latest2[0].Content != "first" {
		t.Fatalf("decode failure = %+v, want the earlier decodable part (no shadow)", latest2)
	}
}

// TestTodoBlockVisible pins the upstream show gate (todo.tsx:12): the
// block is hidden for an empty list and an all-completed list; a
// cancelled item counts as open (status != "completed").
func TestTodoBlockVisible(t *testing.T) {
	if todoBlockVisible(nil) {
		t.Fatal("a nil list must be hidden")
	}
	if todoBlockVisible([]protocol.Todo{{Status: "completed"}, {Status: "completed"}}) {
		t.Fatal("an all-completed list must be hidden")
	}
	if !todoBlockVisible([]protocol.Todo{{Status: "completed"}, {Status: "cancelled"}}) {
		t.Fatal("a cancelled item counts as open")
	}
	if !todoBlockVisible([]protocol.Todo{{Status: "in_progress"}}) {
		t.Fatal("an in_progress list must show")
	}
}

// TestTodoSidebarLines pins the header + the per-item glyph lines (the
// stripANSI unit idiom — the fake terminal has no TTY, the styles strip):
// the ▼ header only when len > 2, the [✓]/[•]/[ ] glyph runs, the
// word-wrap at w-4, the 4-column continuation indent.
func TestTodoSidebarLines(t *testing.T) {
	a := testApp()

	var short []string
	for _, l := range todoSidebarLines([]protocol.Todo{
		{Content: "done thing", Status: "completed"},
		{Content: "doing thing", Status: "in_progress"},
	}, 38, a.theme) {
		short = append(short, stripANSI(l))
	}
	if want := "Todo\n[✓] done thing\n[•] doing thing"; strings.Join(short, "\n") != want {
		t.Fatalf("lines = %q, want %q (the ≤2 items: the bare header)", strings.Join(short, "\n"), want)
	}

	var full []string
	for _, l := range todoSidebarLines(todoTodos(t), 38, a.theme) {
		full = append(full, stripANSI(l))
	}
	if full[0] != "▼ Todo" {
		t.Fatalf("header = %q, want '▼ Todo' (the len > 2 rule)", full[0])
	}
	if full[1] != "[✓] design" || full[2] != "[•] implement the todo sidebar" || full[3] != "[ ] wire the toggle" {
		t.Fatalf("item lines = %q, want the [✓]/[•]/[ ] glyph runs", full[1:])
	}

	// the word-wrap at w-4 (w=16 → contentW=12): "alpha beta gamma"
	// wraps; the continuation lines indent 4 columns.
	var wrapped []string
	for _, l := range todoSidebarLines([]protocol.Todo{
		{Content: "alpha beta gamma", Status: "pending"},
	}, 16, a.theme) {
		wrapped = append(wrapped, stripANSI(l))
	}
	if len(wrapped) != 3 {
		t.Fatalf("wrapped line count = %d, want 3 (header + 2 content lines)", len(wrapped))
	}
	if wrapped[1] != "[ ] alpha beta" || wrapped[2] != "    gamma" {
		t.Fatalf("wrap = %q / %q, want the 4-column continuation indent", wrapped[1], wrapped[2])
	}
}

// TestSidebarToggle pins the ported session.sidebar.toggle flip (upstream
// index.tsx:674-681): visible → "hide" + open=false; invisible → "auto" +
// open=true; the >120 auto-show rule; the home-route no-op (the dispatch
// guard). Driven through the real <leader>b dispatch (the S4.2
// leader-continuation idiom).
func TestSidebarToggle(t *testing.T) {
	// a narrow terminal (100 < 120): auto alone does not show.
	a := testApp()
	a.route = routeSession
	a.size = tea.WindowSizeMsg{Width: 100, Height: 30}
	if a.sidebarVisible() {
		t.Fatal("narrow + auto must be hidden (the >120 auto-show rule)")
	}
	a.handleKey(pressLeader())
	a.handleKey(press('b'))
	if !a.sidebarOpen || a.sidebarMode != "auto" || !a.sidebarVisible() {
		t.Fatalf("after <leader>b: open=%v mode=%q visible=%v, want auto+open+visible",
			a.sidebarOpen, a.sidebarMode, a.sidebarVisible())
	}
	a.handleKey(pressLeader())
	a.handleKey(press('b'))
	if a.sidebarOpen || a.sidebarMode != "hide" || a.sidebarVisible() {
		t.Fatalf("after the 2nd toggle: open=%v mode=%q visible=%v, want hide+closed+hidden",
			a.sidebarOpen, a.sidebarMode, a.sidebarVisible())
	}

	// a wide terminal (140 > 120): auto alone shows; the toggle hides.
	b := testApp()
	b.route = routeSession
	b.size = tea.WindowSizeMsg{Width: 140, Height: 30}
	if !b.sidebarVisible() {
		t.Fatal("wide + auto must show (the upstream wide() rule)")
	}
	b.toggleSidebar()
	if b.sidebarMode != "hide" || b.sidebarOpen || b.sidebarVisible() {
		t.Fatalf("wide toggle: mode=%q open=%v visible=%v, want hide+closed+hidden",
			b.sidebarMode, b.sidebarOpen, b.sidebarVisible())
	}

	// the home route: the dispatch is a no-op (the route guard — the state
	// is untouched; sidebarVisible() itself is route-INDEPENDENT arithmetic,
	// so it is not asserted here — the sidebar simply does not render on
	// home, sessionChrome is session-route-only).
	c := testApp()
	c.size = tea.WindowSizeMsg{Width: 140, Height: 30}
	c.handleKey(pressLeader())
	c.handleKey(press('b'))
	if c.sidebarOpen || c.sidebarMode != "auto" {
		t.Fatalf("home toggle: open=%v mode=%q, want the untouched auto state (the no-op)",
			c.sidebarOpen, c.sidebarMode)
	}
}

// TestSidebarTogglePersists pins the KV round-trip (the S6.3 theme-KV
// seam): the toggle writes sidebar_mode over the engine KV; a second app
// over the SAME KV file (the TestTipsTogglePersists idiom) reloads the
// "hide" mode in NewApp's loadSidebarMode.
func TestSidebarTogglePersists(t *testing.T) {
	a, e := themeApp(t)
	a.route = routeSession
	a.size = tea.WindowSizeMsg{Width: 140, Height: 30}
	a.dispatchCommand("sidebar_toggle")
	if a.sidebarMode != "hide" {
		t.Fatalf("toggle must persist the hide mode (got %q)", a.sidebarMode)
	}
	_ = e.Close() // drains the writer + final flush (idempotent; themeApp's cleanup re-closes)
	dir := filepath.Dir(kvPathOf(e))
	e2, err := theme.New(theme.EngineOptions{
		KVPath:        filepath.Join(dir, "kv.json"),
		GlobalYoloDir: dir,
		CWD:           dir,
		Palette:       func(context.Context) (theme.TerminalColors, bool) { return theme.TerminalColors{}, false },
	})
	if err != nil {
		t.Fatalf("theme.New (second): %v", err)
	}
	t.Cleanup(func() { _ = e2.Close() })
	if err := e2.Resolve(context.Background()); err != nil {
		t.Fatalf("theme.Resolve (second): %v", err)
	}
	b := &recApp{App: NewApp(client.New("http://127.0.0.1:9", ""), store.State{}, "", e2)}
	b.emitSink = func(cmds ...tea.Cmd) { b.Cmds = append(b.Cmds, cmds...) }
	if b.sidebarMode != "hide" {
		t.Fatalf("the sidebar mode must persist across restart (got %q, want hide)", b.sidebarMode)
	}
}

// TestSidebarLayout pins the sessionChrome composition: the sidebar is the
// right sidebarWidth columns of the viewport lines (the title / divider /
// help lines stay full width), the viewport wraps at w-42 (padded to the
// width by the bubbles viewport), and the panel carries the session title
// + the todo block.
func TestSidebarLayout(t *testing.T) {
	a := testApp()
	a.route = routeSession
	a.size = tea.WindowSizeMsg{Width: 140, Height: 30}
	a.store.Current = &protocol.Session{ID: "ses_1", Title: "my session"}
	a.store.Messages = []protocol.MessageWithParts{
		{Info: protocol.Message{ID: "m1", Role: "user"}, Parts: []protocol.Part{{Type: "text", Text: "plan it"}}},
		{Info: protocol.Message{ID: "m2", Role: "assistant"}, Parts: []protocol.Part{
			todowritePart(map[string]any{"content": "one", "status": "in_progress"}),
		}},
	}
	vh := 8
	helpRows := len(strings.Split(wrapLine(sessionHelp, 140), "\n"))
	got := a.sessionChrome(140, vh)
	rows := strings.Split(got, "\n")
	if len(rows) != 1+vh+1+helpRows {
		t.Fatalf("line count = %d, want %d (title + %d viewport + divider + help)",
			len(rows), 1+vh+1+helpRows, vh)
	}
	// every viewport row is exactly 140 columns (the w-42 left viewport,
	// padded to its width, + the 42-column panel).
	for i := 0; i < vh; i++ {
		if w := runeWidth(stripANSI(rows[1+i])); w != 140 {
			t.Fatalf("viewport row %d width = %d, want 140 (the w-42 left + 42 panel)", i, w)
		}
	}
	// the Todo header + the [•] item land in the viewport rows (the
	// right 42 columns; the 1-todo fixture gets the S7.1 bare header —
	// the ▼ glyph is the len > 2 rule only, deviation 250).
	found := 0
	for i := 0; i < vh; i++ {
		l := stripANSI(rows[1+i])
		if strings.Contains(l, "Todo") {
			found++
		}
		if strings.Contains(l, "[•] one") {
			found++
		}
	}
	if found != 2 {
		t.Fatalf("sidebar content rows: found=%d, want the Todo header + the [•] item\n%s", found, got)
	}
	// the session title lands in the panel (right of the transcript).
	if !strings.Contains(stripANSI(got), "my session") {
		t.Fatalf("the session title must render in the panel\n%s", got)
	}
	// the hidden case (the toggle off at the narrow width): the full-width
	// viewport, no panel.
	b := testApp()
	b.route = routeSession
	b.size = tea.WindowSizeMsg{Width: 100, Height: 30}
	b.store.Current = &protocol.Session{ID: "ses_1", Title: "my session"}
	rowsB := strings.Split(b.sessionChrome(100, 4), "\n")
	for i := 0; i < 4; i++ {
		if w := runeWidth(stripANSI(rowsB[1+i])); w != 100 {
			t.Fatalf("hidden-sidebar viewport row %d width = %d, want 100 (no panel)", i, w)
		}
	}
}

// TestSidebarTeatestPresence drives a real turn (the scripted fake driver
// emits a todowrite tool part) at the wide terminal size (140 > 120 → the
// auto-show): the ▼ Todo header + the 3 status-glyph items render. The
// merged WaitFor honors the buffer-drain rule (one condition, not two).
func TestSidebarTeatestPresence(t *testing.T) {
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{{
			Kind:   "tool",
			Name:   "todowrite",
			CallID: "call_todo",
			Args:   json.RawMessage(`{"todos":[{"content":"file the beads","status":"in_progress"},{"content":"run the gate","status":"pending"},{"content":"design the sidebar","status":"completed"}]}`),
			Finish: "tool_calls",
		}}},
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "all done"}}},
	)
	ts := testutil.BootWithDriver(t, drv)
	c := client.New(ts.URL, ts.Dir)
	a := newRecApp(c, store.State{}, "")
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(140, 24))

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	tm.Send(press('n'))
	teatest.WaitFor(t, tm.Output(), hasLine("esc abort/back"), teatest.WithDuration(5*time.Second))
	suiteType(tm, "plan it")
	tm.Send(press(tea.KeyEnter))

	var full string
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		full = stripANSI(string(b))
		return strings.Contains(full, "all done") &&
			strings.Contains(full, "▼ Todo") &&
			strings.Contains(full, "[•] file the beads") &&
			strings.Contains(full, "[ ] run the gate") &&
			strings.Contains(full, "[✓] design the sidebar")
	}, teatest.WithDuration(10*time.Second))

	tm.Send(ctrlCKey) // quit confirm
	tm.Send(press('y'))
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
