package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
)

func permProps() protocol.PermissionAskedProps {
	return protocol.PermissionAskedProps{
		ID:         "perm_1",
		SessionID:  "ses_1",
		Permission: "bash",
		Patterns:   []string{"ls *"},
		Always:     []string{"ls", "dir/*"},
		Tool:       &protocol.PermissionToolRef{MessageID: "msg_1", CallID: "call_abcdef"},
	}
}

func permApp() *recApp {
	a := testApp()
	a.store.Pending = []protocol.PermissionAskedProps{permProps()}
	a.route = routeSession
	a.curSessionID = "ses_1"
	return a
}

func TestPermissionRender(t *testing.T) {
	t.Run("bash ask with no part input", func(t *testing.T) {
		a := permApp()
		got := stripANSI(a.permissionView(80))
		want := "△ Permission required\n" +
			"  # Shell command\n" +
			"  patterns: ls *\n" +
			"  Always: ls, dir/*\n" +
			"Allow once  Allow always  Reject"
		if got != want {
			t.Errorf("permissionView mismatch:\ngot:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("part input renders the body line", func(t *testing.T) {
		a := permApp()
		a.store.Messages = []protocol.MessageWithParts{{
			Info: protocol.Message{ID: "msg_1"},
			Parts: []protocol.Part{{
				ID: "prt_1", MessageID: "msg_1", CallID: "call_abcdef", Type: "tool",
				State: &protocol.ToolState{Input: map[string]any{"command": "echo hi"}},
			}},
		}}
		got := stripANSI(a.permissionView(80))
		if !strings.Contains(got, "  $ echo hi") {
			t.Errorf("body line missing:\n%q", got)
		}
	})

	t.Run("edit ask formats the path title", func(t *testing.T) {
		a := permApp()
		a.store.Pending[0].Permission = "edit"
		a.store.Pending[0].Patterns = nil
		a.store.Pending[0].Always = nil
		a.store.Pending[0].Tool = &protocol.PermissionToolRef{CallID: "c1"}
		a.store.Messages = []protocol.MessageWithParts{{
			Parts: []protocol.Part{{
				CallID: "c1", Type: "tool",
				State: &protocol.ToolState{Input: map[string]any{"filePath": "/tmp/x.go"}},
			}},
		}}
		got := stripANSI(a.permissionView(80))
		if !strings.Contains(got, "  → Edit /tmp/x.go") {
			t.Errorf("edit title missing:\n%q", got)
		}
		if strings.Contains(got, "Always:") || strings.Contains(got, "patterns:") {
			t.Errorf("empty lines must be omitted:\n%q", got)
		}
	})

	t.Run("empty always omits the line", func(t *testing.T) {
		a := permApp()
		a.store.Pending[0].Always = nil
		if got := stripANSI(a.permissionView(80)); strings.Contains(got, "Always:") {
			t.Errorf("Always line must be omitted when empty:\n%q", got)
		}
	})

	t.Run("empty pending renders nothing", func(t *testing.T) {
		a := permApp()
		a.store.Pending = nil
		if got := a.permissionView(80); got != "" {
			t.Errorf("expected empty view, got %q", got)
		}
	})
}

func TestPermissionPillKeys(t *testing.T) {
	if got, ok := permReplyFor(press('1'), 0); !ok || got != "once" {
		t.Fatalf("1 → %q/%v, want once", got, ok)
	}
	if got, ok := permReplyFor(press('2'), 0); !ok || got != "always" {
		t.Fatalf("2 → %q/%v, want always", got, ok)
	}
	if got, ok := permReplyFor(press('3'), 0); !ok || got != "reject" {
		t.Fatalf("3 → %q/%v, want reject", got, ok)
	}
	if got, ok := permReplyFor(press(tea.KeyEscape), 1); !ok || got != "reject" {
		t.Fatalf("esc → %q/%v, want reject (yolo pin)", got, ok)
	}
	if got, ok := permReplyFor(enterKey, 1); !ok || got != "always" {
		t.Fatalf("enter on sel 1 → %q/%v, want always", got, ok)
	}
	if _, ok := permReplyFor(press('x'), 0); ok {
		t.Fatalf("x must not reply")
	}
}

func TestPermissionPillMove(t *testing.T) {
	p := &permDlg{}
	p.moveSel(1)
	if p.sel != 1 {
		t.Fatalf("right: sel=%d, want 1", p.sel)
	}
	p.moveSel(1)
	if p.sel != 2 {
		t.Fatalf("right: sel=%d, want 2", p.sel)
	}
	p.moveSel(1)
	if p.sel != 0 {
		t.Fatalf("wrap: sel=%d, want 0", p.sel)
	}
	p.moveSel(-1)
	if p.sel != 2 {
		t.Fatalf("wrap back: sel=%d, want 2", p.sel)
	}
}

func TestPermissionStackSync(t *testing.T) {
	a := permApp()
	a.syncPermDialog()
	top, ok := a.dlg.top()
	if len(a.dlg.items) != 1 || !ok || top.kind != dlgPerm || !top.modal {
		t.Fatalf("sync must push the perm modal: %+v", a.dlg.items)
	}
	a.syncPermDialog() // idempotent
	if len(a.dlg.items) != 1 {
		t.Fatalf("sync must be idempotent: %+v", a.dlg.items)
	}
	a.store.Pending = nil
	a.syncPermDialog()
	if len(a.dlg.items) != 0 {
		t.Fatalf("drained queue must pop the perm modal: %+v", a.dlg.items)
	}
}

func TestPermissionOverlayAbovePrompt(t *testing.T) {
	a := permApp()
	v := stripANSI(a.view())
	p := stripANSI(a.prompt.view())
	pidx := strings.Index(v, "Permission required")
	sidx := strings.Index(v, p)
	if pidx < 0 || sidx < 0 {
		t.Fatalf("view missing permission overlay or prompt line:\n%q", v)
	}
	if pidx > sidx {
		t.Errorf("overlay must render above the prompt:\n%q", v)
	}
}

func TestPermissionReplyApply(t *testing.T) {
	t.Run("success drops the asked request", func(t *testing.T) {
		a := permApp()
		a.applyPermReply(permReplyMsg{id: "perm_1", reply: "once"})
		if len(a.store.Pending) != 0 {
			t.Fatalf("pending = %+v, want empty", a.store.Pending)
		}
	})

	t.Run("failure toasts and keeps the dialog", func(t *testing.T) {
		a := permApp()
		a.applyPermReply(permReplyMsg{id: "perm_1", reply: "once", err: errors.New("boom")})
		if !hasToast(a, "boom") {
			t.Fatalf("toasts = %+v, want boom", a.toasts)
		}
		if len(a.store.Pending) != 1 {
			t.Fatalf("pending = %+v, want the original ask", a.store.Pending)
		}
	})
}

// permKey builds a synthetic keypress for teatest (teatest v2 Send takes tea.Msg
// directly, bypassing the terminal string parser).
func permKey(r rune) tea.KeyPressMsg {
	switch r {
	case tea.KeyEnter, tea.KeyEscape:
		return tea.KeyPressMsg{Code: r}
	}
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

// permHarness boots the full stack with a scripted driver (turn 1: text + a
// bash tool call, turn 2: final text) and a config that asks for bash, then
// drives the app on the new session at 80x16 (the 13-row viewport holds the
// whole 9-line end transcript — user, divider, listing text, the completed
// tool row, its 3-line inline output preview and "done" — so the completed
// tool row is visible at the end).
func permHarness(t *testing.T) (*testutil.TestServer, *client.Service, *teatest.TestModel, string) {
	t.Helper()
	drv := fake.New(
		fake.Turn{Parts: []llm.Part{
			{Kind: "text", Text: "listing"},
			{Kind: "tool", Name: "bash", CallID: "call_1", Args: json.RawMessage(`{"command":"ls -la"}`), Finish: "tool_calls"},
		}},
		fake.Turn{Parts: []llm.Part{{Kind: "text", Text: "done", Finish: "stop"}}},
	)
	cfg := &protocol.Config{Permission: map[string]any{"bash": "ask"}}
	ts := testutil.BootWithDriverConfig(t, drv, cfg)
	c := client.New(ts.URL, ts.Dir)
	ctx := context.Background()
	ses, err := c.CreateSession(ctx, "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	a := newRecApp(c, store.State{}, ses.ID)
	t.Cleanup(a.Close)
	tm := teatest.NewTestModel(t, a, teatest.WithInitialTermSize(80, 16))
	return ts, c, tm, ses.ID
}

// hasPermDialog matches the real permission flow under the S2.8 restyle
// (header + the info() port + the pills).
func hasPermDialog(b []byte) bool {
	s := stripANSI(string(b))
	return strings.Contains(s, "Permission required") &&
		strings.Contains(s, "Shell command") &&
		strings.Contains(s, "patterns: ls *") &&
		strings.Contains(s, "Allow once") &&
		strings.Contains(s, "Reject")
}

func hasLine(s string) func([]byte) bool {
	return func(b []byte) bool { return bytes.Contains([]byte(stripANSI(string(b))), []byte(s)) }
}

// waitPending polls GET /permission until the parked count settles (the park
// and the reply resolution happen on engine/service goroutines).
func waitPending(t *testing.T, ts *testutil.TestServer, want int) []protocol.PermissionAskedProps {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var out []protocol.PermissionAskedProps
	for time.Now().Before(deadline) {
		resp, b := testutil.Req(t, ts, "GET", "/permission", ts.Dir, "")
		if resp.StatusCode != 200 {
			t.Fatalf("GET /permission: %d %s", resp.StatusCode, b)
		}
		out = nil
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("decode /permission: %v", err)
		}
		if len(out) == want {
			return out
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pending asks = %d, want %d", len(out), want)
	return nil
}

// TestPermissionDialogKeyReply is the teatest scenario: a real server with a
// scripted bash ask; the dialog must render, key 1 replies once, the engine
// resumes and completes the tool.
func TestPermissionDialogKeyReply(t *testing.T) {
	ts, c, tm, sesID := permHarness(t)
	ctx := context.Background()

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	if _, err := c.SendMessage(ctx, sesID, "ls"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	teatest.WaitFor(t, tm.Output(), hasPermDialog, teatest.WithDuration(5*time.Second))
	waitPending(t, ts, 1)

	tm.Send(permKey('1'))

	// Zero-engine run: the running->completed transition rewrites the
	// WHOLE row (`~ Writing command...` -> `$ ls -la`), so the full
	// completed line lands in the drain — pin it + the final text
	// (deviation 144 pinned the pre-S1.7 icon-cell form).
	teatest.WaitFor(t, tm.Output(), hasLines("$ ls -la", "done"), teatest.WithDuration(5*time.Second))
	if got := waitPending(t, ts, 0); len(got) != 0 {
		t.Fatalf("pending = %+v, want empty after reply", got)
	}

	var found bool
	for _, m := range ts.LastMessages(t, sesID) {
		for _, p := range m.Parts {
			if p.Type == "tool" && p.Tool == "bash" && p.State != nil && p.State.Status == "completed" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("bash part not completed server-side")
	}

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestPermissionDialogHTTPReply is the second scenario: the reply comes over
// HTTP directly (not a key); the permission.replied event must drop the
// dialog through the same store path.
func TestPermissionDialogHTTPReply(t *testing.T) {
	ts, c, tm, sesID := permHarness(t)
	ctx := context.Background()

	teatest.WaitFor(t, tm.Output(), hasLine("New session"), teatest.WithDuration(5*time.Second))
	if _, err := c.SendMessage(ctx, sesID, "ls"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	teatest.WaitFor(t, tm.Output(), hasPermDialog, teatest.WithDuration(5*time.Second))
	pend := waitPending(t, ts, 1)
	if len(pend) != 1 {
		t.Fatalf("pending = %+v, want 1 ask", pend)
	}
	if err := c.ReplyPermission(ctx, pend[0].ID, "once"); err != nil {
		t.Fatalf("ReplyPermission: %v", err)
	}

	// Zero-engine run: the running->completed transition rewrites the
	// WHOLE row (`~ Writing command...` -> `$ ls -la`), so the full
	// completed line lands in the drain — pin it + the final text
	// (deviation 144 pinned the pre-S1.7 icon-cell form).
	teatest.WaitFor(t, tm.Output(), hasLines("$ ls -la", "done"), teatest.WithDuration(5*time.Second))
	if got := waitPending(t, ts, 0); len(got) != 0 {
		t.Fatalf("pending = %+v, want empty after the replied event", got)
	}

	_ = tm.Quit()
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

// TestPermissionKeyReplyWiring executes each reply cmd and pins the wire
// body per key (testing-1): 1→once, 2→always, 3→reject, esc→reject.
// TestPermissionKeyGate only counts cmds — a '1'→always mixup passed the
// whole suite before this pin.
func TestPermissionKeyReplyWiring(t *testing.T) {
	for _, tc := range []struct {
		key  rune
		want string
	}{
		{'1', "once"},
		{'2', "always"},
		{'3', "reject"},
		{tea.KeyEscape, "reject"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			var mu sync.Mutex
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/event":
					w.Header().Set("Content-Type", "text/event-stream")
					fl, _ := w.(http.Flusher)
					fl.Flush()
					<-r.Context().Done()
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reply"):
					var body struct {
						Response string `json:"response"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					mu.Lock()
					got = body.Response
					mu.Unlock()
					w.WriteHeader(http.StatusNoContent)
				default:
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			t.Cleanup(srv.Close)

			c := client.New(srv.URL, "")
			a := newRecApp(c, store.State{
				Pending: []protocol.PermissionAskedProps{permProps()},
			}, "ses_1")
			t.Cleanup(a.Close)

			cmds := a.handleKey(press(tc.key))
			if len(cmds) != 1 {
				t.Fatalf("key %q emitted %d cmds, want 1", tc.key, len(cmds))
			}
			cmds[0]() // runs the reply POST synchronously
			mu.Lock()
			defer mu.Unlock()
			if got != tc.want {
				t.Fatalf("key %q replied %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}
