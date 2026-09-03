package session

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kido5217/yolo/internal/bus"
	"github.com/kido5217/yolo/internal/permission"
	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/provider"
	"github.com/kido5217/yolo/internal/storage"
	"github.com/kido5217/yolo/internal/tool"
)

// benchText repeats a representative prose page until it reaches at least
// want bytes, then trims to a rune boundary (same fixture shape as the
// storage bench; packages are independent so it is re-declared here).
func benchText(want int) string {
	page := "stream line: build finished, \"tests\" passed, 42 items — ok\n"
	var sb strings.Builder
	for sb.Len() < want {
		sb.WriteString(page)
	}
	s := sb.String()
	if len(s) > want {
		cut := want
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut]
	}
	return s
}

// BenchmarkCallKeyHash measures the doom-window identity hash (engine.go
// callKeyHash): unmarshal + canonical sorted-key re-marshal + sha256 per
// tool call, over the flat and nested args shapes the engine sees
// (~32KB matches the perf-wave evidence size).
func BenchmarkCallKeyHash(b *testing.B) {
	flat := fmt.Sprintf(`{"command":%q,"timeout":120}`, benchText(1<<10))
	nested := make([]byte, 0, 32<<10)
	nested = append(nested, '{')
	for i := 0; i < 500; i++ {
		format := `"key_%d":{"a":"value %d","n":%d,"s":"%s"},`
		nested = append(nested, fmt.Sprintf(format, i, i, i, strings.Repeat("x", 24))...)
	}
	nested = append(nested, `"last":true}`...)
	cases := []struct {
		name string
		raw  json.RawMessage
	}{
		{"flat/1KB", json.RawMessage(flat)},
		{"nested/32KB", json.RawMessage(nested)},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			var sink string
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sink = callKeyHash(c.raw)
			}
			_ = sink
		})
	}
}

// BenchmarkMessagesFor measures the per-round request-history mapping
// (engine.go messagesFor): ListMessages + ListParts per message, the
// PartToProtocol decode of every part and the system-prompt build, over the
// perf-wave evidence size of 100 messages / 300 parts (50 user x 3 text,
// 50 assistant x text + 2 tool parts, first 20 assistants carrying a
// 32KB-input / 64KB-output tool part). The DB and project dir are
// per-run temp; one warm call primes the git-repo detection cache before
// the timed region.
func BenchmarkMessagesFor(b *testing.B) {
	const (
		sessionID = "ses_bench"
		nMsgs     = 100
	)
	b.ReportAllocs()
	db, err := storage.Open(filepath.Join(b.TempDir(), "yolo.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { db.Close() })
	reg := provider.NewStaticForTest()
	eng, err := New(Deps{
		DB:    db,
		Bus:   bus.New(),
		Prov:  reg,
		Perm:  permission.New(db, bus.New(), nil, b.TempDir()),
		Tools: tool.Registry(),
	})
	if err != nil {
		b.Fatal(err)
	}
	projectDir := b.TempDir()
	srow := storage.SessionRow{
		ID: sessionID, ProjectDir: projectDir, Model: "kido/q",
		Agent: "build", TimeCreated: 1, TimeUpdated: 1,
	}
	if err := db.CreateSession(b.Context(), srow); err != nil {
		b.Fatal(err)
	}
	userText := benchText(256)
	asstText := benchText(2 << 10)
	smallOut := benchText(1 << 10)
	bigIn := benchText(32 << 10)
	bigOut := benchText(64 << 10)
	addPart := func(mid string, p protocol.Part) {
		row, err := storage.ProtocolToPart(p)
		if err != nil {
			b.Fatal(err)
		}
		if err := db.UpsertPart(b.Context(), row); err != nil {
			b.Fatal(err)
		}
	}
	for i := 0; i < nMsgs; i++ {
		mid := fmt.Sprintf("msg_bench_%03d", i)
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		mrow := storage.MessageRow{ID: mid, SessionID: sessionID, Role: role, Agent: "build", TimeCreated: int64(i) + 2}
		if err := db.CreateMessage(b.Context(), mrow); err != nil {
			b.Fatal(err)
		}
		if role == "user" {
			for j := 0; j < 3; j++ {
				textPart := protocol.Part{
					ID: fmt.Sprintf("prt_bench_%03d_%d", i, j), MessageID: mid, SessionID: sessionID,
					Type: "text", Text: userText, Time: protocol.PartTime{Start: int64(i)},
				}
				addPart(mid, textPart)
			}
			continue
		}
		asstPart := protocol.Part{
			ID: fmt.Sprintf("prt_bench_%03d_t", i), MessageID: mid, SessionID: sessionID,
			Type: "text", Text: asstText, Time: protocol.PartTime{Start: int64(i)},
		}
		addPart(mid, asstPart)
		aState := &protocol.ToolState{
			Status: "completed", Input: map[string]any{"command": "ls"}, Output: smallOut,
			Time: protocol.PartTime{Start: int64(i), End: int64(i) + 1},
		}
		addPart(mid, protocol.Part{
			ID: fmt.Sprintf("prt_bench_%03d_a", i), MessageID: mid, SessionID: sessionID,
			Type: "tool", Tool: "bash", State: aState,
		})
		if i < 40 {
			bState := &protocol.ToolState{
				Status: "completed", Input: map[string]any{"command": bigIn}, Output: bigOut,
				Time: protocol.PartTime{Start: int64(i), End: int64(i) + 1},
			}
			addPart(mid, protocol.Part{
				ID: fmt.Sprintf("prt_bench_%03d_b", i), MessageID: mid, SessionID: sessionID,
				Type: "tool", Tool: "bash", State: bState,
			})
		}
	}
	row, err := db.GetSession(b.Context(), sessionID)
	if err != nil {
		b.Fatal(err)
	}
	info, model, err := reg.Resolve("kido/q")
	if err != nil {
		b.Fatal(err)
	}
	t := newTurn(sessionID, row, info, model)
	if _, err := eng.messagesFor(t); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := eng.messagesFor(t); err != nil {
			b.Fatal(err)
		}
	}
}
