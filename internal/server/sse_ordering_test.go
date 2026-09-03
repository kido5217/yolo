package server_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/server/testutil"
)

// sseMsgField pulls a string field out of a message.updated frame's info object.
func sseMsgField(t *testing.T, f testutil.SSEFrame, field string) string {
	t.Helper()
	info, ok := f.Properties["info"].(map[string]any)
	if !ok {
		return ""
	}
	s, _ := info[field].(string)
	return s
}

// ssePart returns the part object of a message.part.updated frame.
func ssePart(t *testing.T, f testutil.SSEFrame) map[string]any {
	t.Helper()
	p, _ := f.Properties["part"].(map[string]any)
	return p
}

func sseTypes(frames []testutil.SSEFrame) []string {
	out := make([]string, 0, len(frames))
	for _, f := range frames {
		out = append(out, f.Type)
	}
	return out
}

// TestSSEOrdering asserts the EXACT faithful frame order for one text turn
// (fake driver). The user message/part are published synchronously in Send
// BEFORE the turn goroutine emits busy — matching upstream v1.18.18, and
// deviating from the plan's pinned "busy first" order (see PROGRESS.md).
func TestSSEOrdering(t *testing.T) {
	t.Parallel()
	s := testutil.Boot(t)
	d := t.TempDir()
	id := mkSession(t, s, d, "Sse") // explicit title: no title-generation side request

	res := testutil.SSEConnect(t, s, d)
	s.WaitSubscribe(t, 1) // subscription live before we publish the turn
	resp, b := testutil.Req(t, s, "POST", "/session/"+id+"/message", d, `{"text":"hello"}`)
	if resp.StatusCode != 202 {
		t.Fatalf("send: %d %s", resp.StatusCode, b)
	}
	var out struct {
		MessageID string `json:"message_id"`
	}
	_ = json.Unmarshal(b, &out)
	userMsgID := out.MessageID

	var frames []testutil.SSEFrame
	for i := 0; i < 200; i++ {
		f := res.Frame(t)
		frames = append(frames, f)
		if f.Type == "session.status" && f.String("status") == "idle" {
			break
		}
	}
	if len(frames) == 0 {
		t.Fatal("no SSE frames received")
	}
	types := sseTypes(frames)

	// frames 0..2 are fixed by index:
	f0 := frames[0]
	if f0.Type != "message.updated" || sseMsgField(t, f0, "role") != "user" || sseMsgField(t, f0, "id") != userMsgID {
		t.Fatalf("frame 0 = type %s info %v, want user message %s (got %v)", f0.Type, f0.Properties["info"], userMsgID, types)
	}
	f1 := frames[1]
	if f1.Type != "message.part.updated" {
		t.Fatalf("frame 1 = %s, want message.part.updated (got %v)", f1.Type, types)
	}
	if p := ssePart(t, f1); p["messageID"] != userMsgID || p["type"] != "text" {
		t.Fatalf("frame 1 part = %v, want user text part %s", p, userMsgID)
	}
	f2 := frames[2]
	if f2.Type != "session.status" || f2.String("status") != "busy" {
		t.Fatalf("frame 2 = %s %v, want session.status busy (got %v)", f2.Type, f2.Properties["status"], types)
	}

	// The rest, by relative order:
	//   assistant message (round start) < first assistant part < last delta
	//   < final assistant part < last assistant message < idle (last frame)
	var ai, pi, di, mi, lastPart int
	ai, pi, di, mi, lastPart = -1, -1, -1, -1, -1
	for i, f := range frames {
		switch f.Type {
		case "message.updated":
			if sseMsgField(t, f, "role") == "assistant" {
				if ai == -1 {
					ai = i
				}
				mi = i // last assistant message.updated
			}
		case "message.part.updated":
			if p := ssePart(t, f); p["messageID"] != userMsgID {
				if pi == -1 {
					pi = i
				}
				lastPart = i // last assistant part.updated (the final frame)
			}
		case "message.part.delta":
			di = i
		}
	}
	for name, idx := range map[string]int{"assistantMsg": ai, "assistantPart": pi, "delta": di, "assistantFinal": mi, "finalPart": lastPart} {
		if idx < 0 {
			t.Fatalf("missing frame %s in %v", name, types)
		}
	}
	if ai <= 2 || pi <= ai || di <= pi || lastPart <= di || mi <= lastPart {
		t.Fatalf("ordering violation: busy(2) < assistantMsg(%d) < assistantPart(%d) < lastDelta(%d) < finalPart(%d) < assistantFinal(%d); got %v",
			ai, pi, di, lastPart, mi, types)
	}
	// the final assistant part carries the full text with an end timestamp
	lp := ssePart(t, frames[lastPart])
	if lp["type"] != "text" || strings.TrimSpace(lpText(lp)) == "" {
		t.Fatalf("final assistant part = %v, want non-empty text", lp)
	}
	if tm, ok := lp["time"].(map[string]any); !ok || tm["end"] == nil {
		t.Fatalf("final assistant part = %v, want time.end set", lp)
	}
	// idle is the last frame
	last := frames[len(frames)-1]
	if last.Type != "session.status" || last.String("status") != "idle" {
		t.Fatalf("last frame = %s %v, want session.status idle", last.Type, last.Properties["status"])
	}
}

func lpText(p map[string]any) string {
	s, _ := p["text"].(string)
	return s
}
