package storage_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

func openDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSessionCRUDAndListOrder(t *testing.T) {
	db := openDB(t)
	mk := storage.SessionRow{ProjectDir: "/w", Title: "t", Model: "kido/Qwen3.8-27B", Agent: "build"}
	for i, id := range []string{"ses_aaa", "ses_bbb", "ses_ccc"} {
		r := mk
		r.ID = id
		r.TimeCreated = int64(100 + i)
		r.TimeUpdated = int64(100 + i)
		if err := db.CreateSession(r); err != nil {
			t.Fatal(err)
		}
	}
	// another directory is isolated
	other := mk
	other.ID, other.ProjectDir = "ses_other", "/other"
	other.TimeCreated, other.TimeUpdated = 999, 999
	if err := db.CreateSession(other); err != nil {
		t.Fatal(err)
	}
	got, err := db.ListSessions("/w", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (scoping broken)", len(got))
	}
	if got[0].ID != "ses_ccc" {
		t.Fatalf("first = %s, want newest-first", got[0].ID)
	}
	if _, err := db.GetSession("ses_missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCascadeDelete(t *testing.T) {
	db := openDB(t)
	if err := db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(storage.MessageRow{ID: "msg_1", SessionID: "ses_1", Role: "user", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPart(storage.PartRow{ID: "prt_1", MessageID: "msg_1", SessionID: "ses_1", Type: "text", StateJSON: `{"text":"hi"}`, TimeCreated: 3}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteSession("ses_1"); err != nil {
		t.Fatal(err)
	}
	msgs, err := db.ListMessages("ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("cascade failed: %d messages", len(msgs))
	}
}

func TestMessageAgentRoundTrip(t *testing.T) {
	db := openDB(t)
	if err := db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(storage.MessageRow{ID: "msg_1", SessionID: "ses_1", Role: "user", Agent: "plan", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(storage.MessageRow{ID: "msg_2", SessionID: "ses_1", Role: "assistant", TimeCreated: 3}); err != nil {
		t.Fatal(err)
	}
	msgs, err := db.ListMessages("ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d", len(msgs))
	}
	if msgs[0].Agent != "plan" {
		t.Fatalf("agent = %q, want plan", msgs[0].Agent)
	}
	if msgs[1].Agent != "build" {
		t.Fatalf("default agent = %q, want build", msgs[1].Agent)
	}
}

func TestTextAndToolPartRoundTrip(t *testing.T) {
	db := openDB(t)
	if err := db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(storage.MessageRow{ID: "msg_1", SessionID: "ses_1", Role: "assistant", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	text := protocol.Part{ID: "prt_txt", MessageID: "msg_1", SessionID: "ses_1", Type: "text", Text: "hello", Time: protocol.PartTime{Start: 5, End: 9}}
	if err := db.UpsertPart(storage.ProtocolToPart(text)); err != nil {
		t.Fatal(err)
	}
	row, err := db.GetPart("prt_txt")
	if err != nil {
		t.Fatal(err)
	}
	back, err := storage.PartToProtocol(row)
	if err != nil {
		t.Fatal(err)
	}
	if back.Text != "hello" || back.Time.End != 9 {
		t.Fatalf("round trip: %+v", back)
	}
	tool := protocol.Part{ID: "prt_tool", MessageID: "msg_1", SessionID: "ses_1", Type: "tool", CallID: "call_1", Tool: "bash", State: &protocol.ToolState{Status: "completed", Input: map[string]any{"command": "ls"}, Output: "ok", Time: protocol.PartTime{Start: 1, End: 2}}}
	if err := db.UpsertPart(storage.ProtocolToPart(tool)); err != nil {
		t.Fatal(err)
	}
	prow, err := db.GetPart("prt_tool")
	if err != nil {
		t.Fatal(err)
	}
	pback, err := storage.PartToProtocol(prow)
	if err != nil {
		t.Fatal(err)
	}
	if pback.State == nil || pback.State.Output != "ok" {
		t.Fatalf("tool state lost: %+v", pback)
	}
	raw, _ := json.Marshal(prow.StateJSON)
	_ = raw
}

func TestSessionAggregateCostTokens(t *testing.T) {
	db := openDB(t)
	if err := db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Title: "x", Model: "kido/m", Agent: "build", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(storage.MessageRow{ID: "msg_u", SessionID: "ses_1", Role: "user", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(storage.MessageRow{ID: "msg_a", SessionID: "ses_1", Role: "assistant", Cost: 0.25,
		Tokens: protocol.Tokens{Input: 100, Output: 50, Reasoning: 5, Cache: protocol.CacheTokens{Read: 7, Write: 1}}, TimeCreated: 3}); err != nil {
		t.Fatal(err)
	}
	sess, err := db.Session("ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Cost != 0.25 || sess.Tokens.Input != 100 || sess.Tokens.Cache.Read != 7 {
		t.Fatalf("aggregate = %+v", sess)
	}
	if sess.Model == nil || sess.Model.ProviderID != "kido" || sess.Model.ID != "m" || sess.Directory != "/w" {
		t.Fatalf("wire mapping: %+v", sess)
	}
}

func TestAlwaysRules(t *testing.T) {
	db := openDB(t)
	if err := db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SavePermission(storage.PermissionRow{RequestID: "per_1", SessionID: "ses_1", Action: "bash", Resource: "*", Response: "always", AlwaysJSON: `["ls","whoami"]`, TimeCreated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SavePermission(storage.PermissionRow{RequestID: "per_2", SessionID: "ses_1", Action: "bash", Resource: "*", Response: "once", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	rules, err := db.AlwaysRules("ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("want 2 always rules, got %d: %+v", len(rules), rules)
	}
	for _, r := range rules {
		if r.Action != "allow" || r.Permission != "bash" {
			t.Fatalf("bad rule %+v", r)
		}
	}
}

func TestSchemaVersionTracked(t *testing.T) {
	db := openDB(t)
	v, err := db.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v < 1 {
		t.Fatalf("schema version = %d", v)
	}
}
