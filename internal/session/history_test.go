package session

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
)

// TestMapHistoryPinsLockedMapping pins the pure LOCKED mapping
// (deviation 77: 1:1 history mirror, no re-append) without the engine.
func TestMapHistoryPinsLockedMapping(t *testing.T) {
	now := time.Now().UnixMilli()
	mk := func(role, agent, content string) protocol.MessageWithParts {
		return protocol.MessageWithParts{
			Info: protocol.Message{
				ID: protocol.NewID("msg"), Role: role, Agent: agent,
				Time: protocol.MessageTime{Created: now},
			},
			Parts: []protocol.Part{{ID: protocol.NewID("prt"), Type: "text", Text: content}},
		}
	}

	toolRound := protocol.MessageWithParts{
		Info: protocol.Message{
			ID: protocol.NewID("msg"), Role: "assistant", Agent: "build",
			Time: protocol.MessageTime{Created: now},
		},
		Parts: []protocol.Part{
			{ID: protocol.NewID("prt"), Type: "text", Text: "checking"},
			{
				ID: "call_1", Type: "tool", Tool: "bash",
				State: &protocol.ToolState{Status: "completed", Input: map[string]any{"command": "ls"}, Output: "out"},
			},
		},
	}

	cases := []struct {
		name string
		hist []protocol.MessageWithParts
		sys  []string
		want []string // role sequence
	}{
		{
			"tool round ends with tool result, 1:1 mirror",
			[]protocol.MessageWithParts{mk("user", "build", "do it"), toolRound, mk("user", "build", "then")},
			[]string{"sys"},
			[]string{"system", "user", "assistant", "tool", "user"},
		},
		{
			"each system entry leads as a separate system message",
			[]protocol.MessageWithParts{mk("user", "build", "hi")},
			[]string{"a", "b"},
			[]string{"system", "system", "user"},
		},
		{
			"empty assistant skipped",
			[]protocol.MessageWithParts{mk("user", "build", "do it"), mk("assistant", "build", "")},
			[]string{"sys"},
			[]string{"system", "user"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mapHistory(c.hist, "build", c.sys)
			roles := make([]string, len(got))
			for i, m := range got {
				roles[i] = string(m.Role)
			}
			if len(roles) != len(c.want) {
				t.Fatalf("roles = %v, want %v", roles, c.want)
			}
			for i := range c.want {
				if roles[i] != c.want[i] {
					t.Fatalf("roles = %v, want %v", roles, c.want)
				}
			}
		})
	}

	t.Run("tool result content and args", func(t *testing.T) {
		got := mapHistory(
			[]protocol.MessageWithParts{mk("user", "build", "do it"), toolRound},
			"build", []string{"sys"})
		if len(got) != 4 {
			t.Fatalf("len(got) = %d, want 4", len(got))
		}
		if got[0].Content != "sys" {
			t.Fatalf("system content = %q, want %q", got[0].Content, "sys")
		}
		asst := got[2]
		if asst.Content != "checking" {
			t.Fatalf("assistant content = %q, want %q", asst.Content, "checking")
		}
		if len(asst.ToolCalls) != 1 {
			t.Fatalf("tool calls = %d, want 1", len(asst.ToolCalls))
		}
		tc := asst.ToolCalls[0]
		if tc.ID != "call_1" || tc.Name != "bash" || string(tc.Args) != `{"command":"ls"}` {
			t.Fatalf("tool call = %+v, want call_1/bash/args {\"command\":\"ls\"}", tc)
		}
		if got[3].Content != "out" || got[3].ToolCallID != "call_1" {
			t.Fatalf("tool message = %+v, want call_1/out", got[3])
		}
	})

	t.Run("plan reminders attach to last user", func(t *testing.T) {
		hist := []protocol.MessageWithParts{
			mk("assistant", "plan", "planned"),
			mk("user", "build", "go"),
		}
		got := mapHistory(hist, "build", []string{"sys"})
		last := got[len(got)-1]
		want := "go\n\n" + buildSwitchMsg
		if string(last.Role) != "user" || last.Content != want {
			t.Fatalf("last = %+v, want user %q", last, want)
		}
	})
}

// TestUserContentBlocks pins the spec §5.2 rule: block format, the
// placeholder, the blank-line joins, the no-files zero-change guarantee,
// and the malformed-URL degradation.
func TestUserContentBlocks(t *testing.T) {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	mk := func(filename, mime, url string) protocol.Part {
		return protocol.Part{Type: protocol.PartTypeFile, Filename: filename, MIME: mime, URL: url}
	}
	text := func(s string) protocol.Part { return protocol.Part{Type: "text", Text: s} }
	cases := []struct {
		name  string
		parts []protocol.Part
		want  string
	}{
		{"no files is joinTextParts", []protocol.Part{text("a"), text("b")}, "a\nb"},
		{"single file, no trailing newline", []protocol.Part{
			mk("a.txt", "text/plain", "data:text/plain;base64,"+b64("hello")),
			text("What do these say?"),
		}, "--- BEGIN FILE a.txt ---\nhello\n--- END FILE a.txt ---\n\nWhat do these say?"},
		{"the spec §5.2 full example", []protocol.Part{
			mk("a.txt", "text/plain", "data:text/plain;base64,"+b64("hello")),
			mk("b.txt", "text/plain", "data:text/plain;base64,"+b64("world\n")),
			mk("bin.dat", "application/octet-stream", "data:application/octet-stream;base64,"+b64("\x01\x02")),
			text("What do these say?"),
		}, "--- BEGIN FILE a.txt ---\nhello\n--- END FILE a.txt ---\n\n" +
			"--- BEGIN FILE b.txt ---\nworld\n--- END FILE b.txt ---\n\n" +
			"[Attached application/octet-stream: bin.dat]\n\nWhat do these say?"},
		{"files only, no text", []protocol.Part{
			mk("a.txt", "text/plain", "data:text/plain;base64,"+b64("hello")),
		}, "--- BEGIN FILE a.txt ---\nhello\n--- END FILE a.txt ---"},
		{"malformed data url degrades to the placeholder", []protocol.Part{
			mk("a.txt", "text/plain", "data:text/plain;base64,!!!"),
		}, "[Attached text/plain: a.txt]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := userContent(c.parts); got != c.want {
				t.Fatalf("userContent =\n%q\nwant\n%q", got, c.want)
			}
		})
	}
}
