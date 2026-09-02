package tui

import (
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/tui/store"
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
