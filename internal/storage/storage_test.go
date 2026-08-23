package storage_test

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"sync"
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

func TestOpenAppliesPragmasToEveryConnection(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "yolo.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	// A transaction stays bound to its pool connection until rollback, so
	// concurrent txs hold one distinct connection each; every one of them
	// must carry the per-connection PRAGMAs.
	const n = 4
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, err := db.Begin()
			if err != nil {
				errs <- err
				return
			}
			defer tx.Rollback()
			var fk, busy int
			if err := tx.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
				errs <- err
				return
			}
			if err := tx.QueryRow(`PRAGMA busy_timeout`).Scan(&busy); err != nil {
				errs <- err
				return
			}
			if fk != 1 {
				errs <- fmt.Errorf("connection foreign_keys = %d, want 1", fk)
				return
			}
			if busy != 5000 {
				errs <- fmt.Errorf("connection busy_timeout = %d, want 5000", busy)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestOpenBoundsConnectionPool(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	// Single-process SQLite store: one shared connection keeps at most
	// one writer and makes the per-connection PRAGMAs total.
	if got := db.Stats().MaxOpenConnections; got != 1 {
		t.Errorf("MaxOpenConnections = %d, want 1", got)
	}
}

func TestSessionCRUDAndListOrder(t *testing.T) {
	t.Parallel()
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
	ids := make([]string, 0, len(got))
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	if want := []string{"ses_ccc", "ses_bbb", "ses_aaa"}; !slices.Equal(ids, want) {
		t.Fatalf("order = %v, want %v (newest-first)", ids, want)
	}
	if _, err := db.GetSession("ses_missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCascadeDelete(t *testing.T) {
	t.Parallel()
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
	if _, err := db.GetPart("prt_1"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("part not cascaded: %v", err)
	}
}

func TestMessageAgentRoundTrip(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	db := openDB(t)
	if err := db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(storage.MessageRow{ID: "msg_1", SessionID: "ses_1", Role: "assistant", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	text := protocol.Part{ID: "prt_txt", MessageID: "msg_1", SessionID: "ses_1", Type: "text", Text: "hello", Time: protocol.PartTime{Start: 5, End: 9}}
	textRow, err := storage.ProtocolToPart(text)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPart(textRow); err != nil {
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
	if back.Text != "hello" || back.Time.Start != 5 || back.Time.End != 9 {
		t.Fatalf("round trip: %+v (Time.Start must survive via TimeCreated)", back)
	}
	tool := protocol.Part{ID: "prt_tool", MessageID: "msg_1", SessionID: "ses_1", Type: "tool", CallID: "call_1", Tool: "bash", State: &protocol.ToolState{Status: "completed", Input: map[string]any{"command": "ls"}, Output: "ok", Time: protocol.PartTime{Start: 1, End: 2}}}
	toolRow, err := storage.ProtocolToPart(tool)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPart(toolRow); err != nil {
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
}

func TestClosedDBReturnsErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yolo.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetPart("prt_x"); err == nil {
		t.Error("GetPart on closed DB: want error, got nil")
	}
	if _, err := db.Exec(`SELECT 1`); err == nil {
		t.Error("Exec on closed DB: want error, got nil")
	}
	if _, err := db.Query(`SELECT 1`); err == nil {
		t.Error("Query on closed DB: want error, got nil")
	}
	if err := db.CreateSession(storage.SessionRow{ID: "s", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err == nil {
		t.Error("CreateSession on closed DB: want error, got nil")
	}
}

func TestProtocolToPartTextStateJSONBytes(t *testing.T) {
	// The text branch of ProtocolToPart is on the streaming hot path; its
	// output is a round-trip contract with PartToProtocol. The encoder may
	// be optimized but must stay byte-identical: sorted keys (end,
	// synthetic, text), compact separators, HTML string escaping.
	syn, noSyn := true, false
	cases := []struct {
		name string
		p    protocol.Part
		want string
	}{
		{"text only", protocol.Part{Type: "text", Text: "hi"}, `{"text":"hi"}`},
		{"end", protocol.Part{Type: "text", Text: "hi", Time: protocol.PartTime{End: 9}}, `{"end":9,"text":"hi"}`},
		{"synthetic", protocol.Part{Type: "text", Text: "hi", Synthetic: &syn}, `{"synthetic":true,"text":"hi"}`},
		{"end and synthetic", protocol.Part{Type: "text", Text: "hi", Time: protocol.PartTime{End: 9}, Synthetic: &syn}, `{"end":9,"synthetic":true,"text":"hi"}`},
		{"synthetic false omitted", protocol.Part{Type: "text", Text: "hi", Synthetic: &noSyn}, `{"text":"hi"}`},
		{"html escaping", protocol.Part{Type: "text", Text: `<a>&"q"`}, `{"text":"\u003ca\u003e\u0026\"q\""}`},
		{"empty text", protocol.Part{Type: "text"}, `{"text":""}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			row, err := storage.ProtocolToPart(c.p)
			if err != nil {
				t.Fatal(err)
			}
			if got := row.StateJSON; got != c.want {
				t.Errorf("StateJSON = %s, want %s", got, c.want)
			}
		})
	}
}

func TestNullColumnRoundTrips(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	if err := db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	tc := int64(99)
	if err := db.CreateMessage(storage.MessageRow{ID: "msg_done", SessionID: "ses_1", Role: "assistant", TimeCreated: 1, TimeCompleted: &tc}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(storage.MessageRow{ID: "msg_open", SessionID: "ses_1", Role: "assistant", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPart(storage.PartRow{ID: "prt_notool", MessageID: "msg_open", SessionID: "ses_1", Type: "text", StateJSON: `{"text":"x"}`, TimeCreated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPart(storage.PartRow{ID: "prt_tool", MessageID: "msg_open", SessionID: "ses_1", Type: "tool", Tool: "bash", StateJSON: `{"status":"completed"}`, TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.SavePermission(storage.PermissionRow{RequestID: "per_new", SessionID: "ses_1", Action: "bash", Resource: "*", TimeCreated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SavePermission(storage.PermissionRow{RequestID: "per_done", SessionID: "ses_1", Action: "bash", Resource: "*", Response: "once", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	t.Run("message time_completed", func(t *testing.T) {
		msgs, err := db.ListMessages("ses_1")
		if err != nil {
			t.Fatal(err)
		}
		if len(msgs) != 2 {
			t.Fatalf("len = %d, want 2", len(msgs))
		}
		if msgs[0].TimeCompleted == nil || *msgs[0].TimeCompleted != 99 {
			t.Fatalf("msg_done TimeCompleted = %v, want 99", msgs[0].TimeCompleted)
		}
		if msgs[1].TimeCompleted != nil {
			t.Fatalf("msg_open TimeCompleted = %v, want nil", msgs[1].TimeCompleted)
		}
	})
	t.Run("part tool", func(t *testing.T) {
		row, err := db.GetPart("prt_notool")
		if err != nil {
			t.Fatal(err)
		}
		if row.Tool != "" {
			t.Fatalf("prt_notool Tool = %q, want \"\" (NULL round trip)", row.Tool)
		}
		row, err = db.GetPart("prt_tool")
		if err != nil {
			t.Fatal(err)
		}
		if row.Tool != "bash" {
			t.Fatalf("prt_tool Tool = %q, want bash", row.Tool)
		}
	})
	t.Run("permission response", func(t *testing.T) {
		pending, err := db.ListPermissions("ses_1", true)
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) != 1 || pending[0].RequestID != "per_new" {
			t.Fatalf("pending = %+v, want only per_new", pending)
		}
		if pending[0].Response != "" || pending[0].AlwaysJSON != "" {
			t.Fatalf("per_new (Response, AlwaysJSON) = (%q, %q), want both empty", pending[0].Response, pending[0].AlwaysJSON)
		}
		all, err := db.ListPermissions("ses_1", false)
		if err != nil {
			t.Fatal(err)
		}
		if len(all) != 2 || all[1].Response != "once" {
			t.Fatalf("all = %+v, want 2 rows with per_done responded", all)
		}
	})
}

func TestErrNotFoundForMissingRows(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	t.Run("missing part", func(t *testing.T) {
		if _, err := db.GetPart("prt_missing"); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
	t.Run("missing session", func(t *testing.T) {
		if err := db.UpdateSession("ses_missing", storage.SessionRow{Title: "t"}); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
	t.Run("missing message", func(t *testing.T) {
		if err := db.UpdateMessage(storage.MessageRow{ID: "msg_missing"}); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
	t.Run("missing permission request", func(t *testing.T) {
		if err := db.ReplyPermission("per_missing", "once"); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

func TestSessionAggregateCostTokens(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	db := openDB(t)
	v, err := db.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v < 1 {
		t.Fatalf("schema version = %d", v)
	}
}

func TestReopenAlreadyMigratedDatabase(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "yolo.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CreateSession(storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Title: "t", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	v, err := db.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Fatalf("schema version = %d, want 2", v)
	}
	row, err := db.GetSession("ses_1")
	if err != nil {
		t.Fatalf("data lost on reopen: %v", err)
	}
	if row.Title != "t" {
		t.Fatalf("title = %q, want t", row.Title)
	}
}

// TestProtocolToPartSurfacesMarshalError: an unmarshalable tool state (NaN in
// input) must fail at write time, not persist StateJSON="" and 500 every
// later read (safety-3 + error-2).
func TestProtocolToPartSurfacesMarshalError(t *testing.T) {
	_, err := storage.ProtocolToPart(protocol.Part{
		ID: "prt_1", MessageID: "msg_1", SessionID: "ses_1", Type: "tool", Tool: "bash",
		State: &protocol.ToolState{Status: "running", Input: map[string]any{"n": math.NaN()}},
	})
	if err == nil {
		t.Fatal("ProtocolToPart accepted an unmarshalable tool state (NaN) — the error must surface")
	}
}
