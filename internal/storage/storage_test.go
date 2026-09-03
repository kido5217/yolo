package storage_test

import (
	"context"
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
		if err := db.CreateSession(t.Context(), r); err != nil {
			t.Fatal(err)
		}
	}
	// another directory is isolated
	other := mk
	other.ID, other.ProjectDir = "ses_other", "/other"
	other.TimeCreated, other.TimeUpdated = 999, 999
	if err := db.CreateSession(t.Context(), other); err != nil {
		t.Fatal(err)
	}
	got, err := db.ListSessions(t.Context(), "/w", 50)
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
	if _, err := db.GetSession(t.Context(), "ses_missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCascadeDelete(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_1", SessionID: "ses_1", Role: "user", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPart(t.Context(), storage.PartRow{ID: "prt_1", MessageID: "msg_1", SessionID: "ses_1", Type: "text", StateJSON: `{"text":"hi"}`, TimeCreated: 3}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteSession(t.Context(), "ses_1"); err != nil {
		t.Fatal(err)
	}
	msgs, err := db.ListMessages(t.Context(), "ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("cascade failed: %d messages", len(msgs))
	}
	if _, err := db.GetPart(t.Context(), "prt_1"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("part not cascaded: %v", err)
	}
}

func TestMessageAgentRoundTrip(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_1", SessionID: "ses_1", Role: "user", Agent: "plan", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_2", SessionID: "ses_1", Role: "assistant", TimeCreated: 3}); err != nil {
		t.Fatal(err)
	}
	msgs, err := db.ListMessages(t.Context(), "ses_1")
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
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_1", SessionID: "ses_1", Role: "assistant", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	text := protocol.Part{ID: "prt_txt", MessageID: "msg_1", SessionID: "ses_1", Type: "text", Text: "hello", Time: protocol.PartTime{Start: 5, End: 9}}
	textRow, err := storage.ProtocolToPart(text)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPart(t.Context(), textRow); err != nil {
		t.Fatal(err)
	}
	row, err := db.GetPart(t.Context(), "prt_txt")
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
	if err := db.UpsertPart(t.Context(), toolRow); err != nil {
		t.Fatal(err)
	}
	prow, err := db.GetPart(t.Context(), "prt_tool")
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
	if _, err := db.GetPart(t.Context(), "prt_x"); err == nil {
		t.Error("GetPart on closed DB: want error, got nil")
	}
	if _, err := db.Exec(`SELECT 1`); err == nil {
		t.Error("Exec on closed DB: want error, got nil")
	}
	if _, err := db.Query(`SELECT 1`); err == nil {
		t.Error("Query on closed DB: want error, got nil")
	}
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "s", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err == nil {
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
		{"synthetic", protocol.Part{Type: "text", Text: "hi", IsSynthetic: &syn}, `{"synthetic":true,"text":"hi"}`},
		{"end and synthetic", protocol.Part{Type: "text", Text: "hi", Time: protocol.PartTime{End: 9}, IsSynthetic: &syn}, `{"end":9,"synthetic":true,"text":"hi"}`},
		{"synthetic false omitted", protocol.Part{Type: "text", Text: "hi", IsSynthetic: &noSyn}, `{"text":"hi"}`},
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

// TestMessageErrorRoundTrip: the turn-failure error (migration 3
// error_json) round-trips through create/list/get and SetMessageError; the
// unset error stays nil (NULL), and the zero-rows update paths map to
// ErrNotFound.
func TestMessageErrorRoundTrip(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	tc := int64(42)
	if err := db.CreateMessage(t.Context(), storage.MessageRow{
		ID: "msg_err", SessionID: "ses_1", Role: "assistant", TimeCreated: 2, TimeCompleted: &tc,
		Error: &protocol.MessageError{Type: "unknown", Message: "boom"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_ok", SessionID: "ses_1", Role: "user", TimeCreated: 3}); err != nil {
		t.Fatal(err)
	}
	msgs, err := db.ListMessages(t.Context(), "ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Error == nil || msgs[0].Error.Type != "unknown" || msgs[0].Error.Message != "boom" {
		t.Fatalf("msg_err Error = %+v, want unknown/boom", msgs[0].Error)
	}
	if msgs[0].TimeCompleted == nil || *msgs[0].TimeCompleted != 42 {
		t.Fatalf("msg_err TimeCompleted = %v, want 42 (the error column must not shift the row)", msgs[0].TimeCompleted)
	}
	if msgs[1].Error != nil {
		t.Fatalf("msg_ok Error = %+v, want nil (NULL round trip)", msgs[1].Error)
	}
	row, err := db.GetMessage(t.Context(), "msg_err")
	if err != nil {
		t.Fatal(err)
	}
	if row.Error == nil || row.Error.Type != "unknown" || row.Error.Message != "boom" || row.Role != "assistant" {
		t.Fatalf("GetMessage = %+v", row)
	}
	if err := db.SetMessageError(t.Context(), "msg_ok", protocol.MessageError{Type: "aborted", Message: "aborted by the user"}); err != nil {
		t.Fatal(err)
	}
	back, err := db.GetMessage(t.Context(), "msg_ok")
	if err != nil {
		t.Fatal(err)
	}
	if back.Error == nil || back.Error.Type != "aborted" || back.Error.Message != "aborted by the user" {
		t.Fatalf("SetMessageError round trip = %+v", back.Error)
	}
	if back.Role != "user" || back.TimeCreated != 3 {
		t.Fatalf("SetMessageError must not clobber the row: %+v", back)
	}
	if _, err := db.GetMessage(t.Context(), "msg_missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("GetMessage missing id: want ErrNotFound, got %v", err)
	}
	if err := db.SetMessageError(t.Context(), "msg_missing", protocol.MessageError{Type: "unknown", Message: "x"}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("SetMessageError missing id: want ErrNotFound, got %v", err)
	}
}

// TestMessageErrorColumnUpgradeFromV2: an existing pre-v3 database (meta at
// schema_version 2, no error_json column) upgrades in place to v3 with the
// column present and round-tripping.
func TestMessageErrorColumnUpgradeFromV2(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "yolo.db")
	db, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// Downgrade to the pre-v3 shape: drop the column and pin the meta at 2,
	// so the reopen runs the real ALTER TABLE upgrade of an existing DB.
	if _, err := db.Exec(`ALTER TABLE message DROP COLUMN error_json`); err != nil {
		t.Skipf("sqlite without DROP COLUMN support: %v", err)
	}
	if _, err := db.Exec(`UPDATE meta SET value='2' WHERE key='schema_version'`); err != nil {
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
	if v != 3 {
		t.Fatalf("schema version = %d, want 3 (in-place upgrade)", v)
	}
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_1", SessionID: "ses_1", Role: "assistant", TimeCreated: 2,
		Error: &protocol.MessageError{Type: "unknown", Message: "upgraded"}}); err != nil {
		t.Fatalf("upgraded column unusable: %v", err)
	}
	row, err := db.GetMessage(t.Context(), "msg_1")
	if err != nil {
		t.Fatal(err)
	}
	if row.Error == nil || row.Error.Message != "upgraded" {
		t.Fatalf("upgraded round trip = %+v", row.Error)
	}
}

func TestNullColumnRoundTrips(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	tc := int64(99)
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_done", SessionID: "ses_1", Role: "assistant", TimeCreated: 1, TimeCompleted: &tc}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_open", SessionID: "ses_1", Role: "assistant", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPart(t.Context(), storage.PartRow{ID: "prt_notool", MessageID: "msg_open", SessionID: "ses_1", Type: "text", StateJSON: `{"text":"x"}`, TimeCreated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertPart(t.Context(), storage.PartRow{ID: "prt_tool", MessageID: "msg_open", SessionID: "ses_1", Type: "tool", Tool: "bash", StateJSON: `{"status":"completed"}`, TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.SavePermission(t.Context(), storage.PermissionRow{RequestID: "per_new", SessionID: "ses_1", Action: "bash", Resource: "*", TimeCreated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SavePermission(t.Context(), storage.PermissionRow{RequestID: "per_done", SessionID: "ses_1", Action: "bash", Resource: "*", Response: "once", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	t.Run("message time_completed", func(t *testing.T) {
		msgs, err := db.ListMessages(t.Context(), "ses_1")
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
		row, err := db.GetPart(t.Context(), "prt_notool")
		if err != nil {
			t.Fatal(err)
		}
		if row.Tool != "" {
			t.Fatalf("prt_notool Tool = %q, want \"\" (NULL round trip)", row.Tool)
		}
		row, err = db.GetPart(t.Context(), "prt_tool")
		if err != nil {
			t.Fatal(err)
		}
		if row.Tool != "bash" {
			t.Fatalf("prt_tool Tool = %q, want bash", row.Tool)
		}
	})
	t.Run("permission response", func(t *testing.T) {
		pending, err := db.ListPermissions(t.Context(), "ses_1", true)
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) != 1 || pending[0].RequestID != "per_new" {
			t.Fatalf("pending = %+v, want only per_new", pending)
		}
		if pending[0].Response != "" || pending[0].AlwaysJSON != "" {
			t.Fatalf("per_new (Response, AlwaysJSON) = (%q, %q), want both empty", pending[0].Response, pending[0].AlwaysJSON)
		}
		all, err := db.ListPermissions(t.Context(), "ses_1", false)
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
		if _, err := db.GetPart(t.Context(), "prt_missing"); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
	t.Run("missing session", func(t *testing.T) {
		if err := db.UpdateSession(t.Context(), "ses_missing", storage.SessionRow{Title: "t"}); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
	t.Run("missing message", func(t *testing.T) {
		if err := db.UpdateMessage(t.Context(), storage.MessageRow{ID: "msg_missing"}); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
	t.Run("missing permission request", func(t *testing.T) {
		if err := db.ReplyPermission(t.Context(), "per_missing", "once"); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

func TestSessionAggregateCostTokens(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Title: "x", Model: "kido/m", Agent: "build", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_u", SessionID: "ses_1", Role: "user", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_a", SessionID: "ses_1", Role: "assistant", Cost: 0.25,
		Tokens: protocol.Tokens{Input: 100, Output: 50, Reasoning: 5, Cache: protocol.CacheTokens{Read: 7, Write: 1}}, TimeCreated: 3}); err != nil {
		t.Fatal(err)
	}
	sess, err := db.Session(t.Context(), "ses_1")
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
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SavePermission(t.Context(), storage.PermissionRow{RequestID: "per_1", SessionID: "ses_1", Action: "bash", Resource: "*", Response: "always", AlwaysJSON: `["ls","whoami"]`, TimeCreated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SavePermission(t.Context(), storage.PermissionRow{RequestID: "per_2", SessionID: "ses_1", Action: "bash", Resource: "*", Response: "once", TimeCreated: 2}); err != nil {
		t.Fatal(err)
	}
	rules, err := db.AlwaysRules(t.Context(), "ses_1")
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
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_1", ProjectDir: "/w", Title: "t", TimeCreated: 1, TimeUpdated: 1}); err != nil {
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
	if v != 3 {
		t.Fatalf("schema version = %d, want 3", v)
	}
	row, err := db.GetSession(t.Context(), "ses_1")
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

// TestSameMillisecondTiebreakRowid: same-time_created rows come back in
// insertion (rowid) order — parts, messages, permissions (safety-4 +
// database-6). Pre-fix the order is query-plan-dependent; the test pins
// insertion order.
func TestSameMillisecondTiebreakRowid(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	const ses, msg = "ses_t", "msg_a"
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: ses, ProjectDir: "/p", Model: "kido/q", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_a", SessionID: ses, Role: "user", TimeCreated: 7}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: "msg_b", SessionID: ses, Role: "assistant", TimeCreated: 7}); err != nil {
		t.Fatal(err)
	}
	must := func(id, text string) {
		t.Helper()
		if err := db.UpsertPart(t.Context(), storage.PartRow{ID: id, MessageID: msg, SessionID: ses, Type: "text", StateJSON: `{"text":"` + text + `"}`, TimeCreated: 9}); err != nil {
			t.Fatal(err)
		}
	}
	must("prt_a", "a")
	must("prt_b", "b")
	if err := db.SavePermission(t.Context(), storage.PermissionRow{RequestID: "perm_a", SessionID: ses, Action: "bash", TimeCreated: 11}); err != nil {
		t.Fatal(err)
	}
	if err := db.SavePermission(t.Context(), storage.PermissionRow{RequestID: "perm_b", SessionID: ses, Action: "bash", TimeCreated: 11}); err != nil {
		t.Fatal(err)
	}

	partRows, err := db.ListParts(t.Context(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := partIDs(partRows), []string{"prt_a", "prt_b"}; !slices.Equal(got, want) {
		t.Fatalf("ListParts = %v, want %v (rowid tiebreak)", got, want)
	}
	msgRows, err := db.ListMessages(t.Context(), ses)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := messageIDs(msgRows), []string{"msg_a", "msg_b"}; !slices.Equal(got, want) {
		t.Fatalf("ListMessages = %v, want %v (rowid tiebreak)", got, want)
	}
	permRows, err := db.ListPermissions(t.Context(), ses, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := permissionIDs(permRows), []string{"perm_a", "perm_b"}; !slices.Equal(got, want) {
		t.Fatalf("ListPermissions = %v, want %v (rowid tiebreak)", got, want)
	}
}

func partIDs(rows []storage.PartRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

func messageIDs(rows []storage.MessageRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

func permissionIDs(rows []storage.PermissionRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.RequestID
	}
	return out
}

// TestListPartsByMessageIDs pins the batched part fetch (the N+1 fix): one
// parameterized IN query returns every requested message's parts grouped by
// message_id, each message's parts in ListParts order (time_created ASC,
// rowid ASC); a message with no parts and an id absent from the table
// contribute no rows, and an empty input returns an empty slice.
func TestListPartsByMessageIDs(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	const ses = "ses_batch"
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: ses, ProjectDir: "/p", Model: "kido/q", TimeCreated: 1, TimeUpdated: 1}); err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"msg_x", "msg_a", "msg_b"} {
		if err := db.CreateMessage(t.Context(), storage.MessageRow{ID: id, SessionID: ses, Role: "user", TimeCreated: int64(10 + i)}); err != nil {
			t.Fatal(err)
		}
	}
	seed := func(id, msg string, tc int64) {
		t.Helper()
		if err := db.UpsertPart(t.Context(), storage.PartRow{ID: id, MessageID: msg, SessionID: ses, Type: "text", StateJSON: `{"text":"` + id + `"}`, TimeCreated: tc}); err != nil {
			t.Fatal(err)
		}
	}
	// msg_a: a same-time_created pair (the rowid tiebreak decides), msg_b:
	// two parts inserted in reverse time_created order (the query must
	// re-sort them), msg_x: no parts at all.
	seed("prt_a1", "msg_a", 5)
	seed("prt_a2", "msg_a", 5)
	seed("prt_b1", "msg_b", 9)
	seed("prt_b2", "msg_b", 7)

	grouped := func(t *testing.T, ids []string) map[string][]string {
		t.Helper()
		rows, err := db.ListPartsByMessageIDs(t.Context(), ids)
		if err != nil {
			t.Fatalf("ListPartsByMessageIDs: %v", err)
		}
		got := map[string][]string{}
		for _, r := range rows {
			got[r.MessageID] = append(got[r.MessageID], r.ID)
		}
		return got
	}

	t.Run("multiple messages grouped with per-message order", func(t *testing.T) {
		// Input order is deliberately not alphabetical: the query's
		// message_id, time_created, rowid order groups and orders the
		// output regardless of the input order.
		got := grouped(t, []string{"msg_b", "msg_missing", "msg_a", "msg_x"})
		want := map[string][]string{
			"msg_a": {"prt_a1", "prt_a2"},
			"msg_b": {"prt_b2", "prt_b1"},
		}
		if len(got) != len(want) {
			t.Fatalf("grouped = %v, want %v", got, want)
		}
		for msg, wantIDs := range want {
			if !slices.Equal(got[msg], wantIDs) {
				t.Fatalf("parts[%s] = %v, want %v", msg, got[msg], wantIDs)
			}
		}
	})

	t.Run("message without parts and unknown id contribute nothing", func(t *testing.T) {
		got := grouped(t, []string{"msg_x", "msg_missing"})
		if len(got) != 0 {
			t.Fatalf("grouped = %v, want no messages", got)
		}
	})

	t.Run("empty input returns empty slice without error", func(t *testing.T) {
		rows, err := db.ListPartsByMessageIDs(t.Context(), nil)
		if err != nil {
			t.Fatalf("empty input: %v", err)
		}
		if rows == nil || len(rows) != 0 {
			t.Fatalf("rows = %v, want an empty non-nil slice", rows)
		}
	})
}

// TestUpdateNotFoundPaths: a zero-rows update maps to ErrNotFound (the
// surviving path after Task J starts returning driver RowsAffected errors
// as-is instead of masking them as not-found).
func TestUpdateNotFoundPaths(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	if err := db.UpdateSession(t.Context(), "ses_nope", storage.SessionRow{Title: "x"}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("UpdateSession missing id: err = %v, want ErrNotFound", err)
	}
	if err := db.UpdateMessage(t.Context(), storage.MessageRow{ID: "msg_nope"}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("UpdateMessage missing id: err = %v, want ErrNotFound", err)
	}
	if err := db.ReplyPermission(t.Context(), "perm_nope", "once"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("ReplyPermission missing id: err = %v, want ErrNotFound", err)
	}
}

// TestCancelledCtxReachesDriver: a cancelled ctx fails the DAO call with the
// ctx error (context propagation — database-3).
func TestCancelledCtxReachesDriver(t *testing.T) {
	t.Parallel()
	db := openDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := db.GetSession(ctx, "ses_nope"); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetSession(cancelled) err = %v, want context.Canceled", err)
	}
}
