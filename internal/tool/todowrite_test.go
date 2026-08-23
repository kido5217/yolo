package tool

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/storage"
)

func todoEnv(t *testing.T) (*storage.DB, *Env) {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "yolo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.CreateSession(t.Context(), storage.SessionRow{ID: "ses_t", ProjectDir: "/w", Agent: "build", Model: "k"}); err != nil {
		t.Fatal(err)
	}
	d := t.TempDir()
	return db, &Env{Dir: d, Limits: Limits{2000, 50 * 1024}, Storage: db, SessionID: "ses_t"}
}

func TestTodoWritePersistsAndTitles(t *testing.T) {
	t.Parallel()
	t.Run("persist, title and round trip", func(t *testing.T) {
		t.Parallel()
		db, env := todoEnv(t)
		raw, _ := json.Marshal(map[string]any{"todos": []map[string]any{
			{"content": "a", "status": "completed", "priority": "high"},
			{"content": "b", "status": "in_progress"},
			{"content": "c", "status": "pending"},
		}})
		out, err := Registry()["todowrite"].Run(context.Background(), raw, env)
		if err != nil {
			t.Fatal(err)
		}
		if out.Title != "2 todos" {
			t.Fatalf("title = %q", out.Title)
		}
		// output JSON shape: 2-space indent, round-trips to the same todos
		// (key order is model-controlled, so compare fields, not bytes)
		if !strings.Contains(out.Text, "\n  {\n    \"content\"") {
			t.Fatalf("indent/shape = %q", out.Text)
		}
		var rt []protocol.Todo
		if err := json.Unmarshal([]byte(out.Text), &rt); err != nil {
			t.Fatalf("unmarshal out.Text: %v", err)
		}
		if len(rt) != 3 || rt[0].Status != "completed" || rt[0].Priority != "high" || rt[1].Priority != "medium" || rt[2].Priority != "medium" {
			t.Fatalf("round trip = %+v", rt)
		}
		back, err := db.GetTodos(t.Context(), "ses_t")
		if err != nil || len(back) != 3 {
			t.Fatalf("get = %v %v", back, err)
		}
		if back[1].Status != "in_progress" || back[1].Priority != "medium" {
			t.Fatalf("row = %+v", back[1])
		}
	})
	t.Run("update replaces", func(t *testing.T) {
		t.Parallel()
		db, env := todoEnv(t)
		raw, _ := json.Marshal(map[string]any{"todos": []map[string]any{{"content": "z", "status": "pending"}}})
		if _, err := Registry()["todowrite"].Run(context.Background(), raw, env); err != nil {
			t.Fatal(err)
		}
		back, err := db.GetTodos(t.Context(), "ses_t")
		if err != nil || len(back) != 1 || back[0].Content != "z" {
			t.Fatalf("replace failed: %v %v", back, err)
		}
	})
	t.Run("invalid status rejected", func(t *testing.T) {
		t.Parallel()
		_, env := todoEnv(t)
		raw, _ := json.Marshal(map[string]any{"todos": []map[string]any{{"content": "x", "status": "bogus"}}})
		if _, err := Registry()["todowrite"].Run(context.Background(), raw, env); err == nil || !strings.Contains(err.Error(), "invalid status") {
			t.Fatalf("validation err = %v", err)
		}
	})
}
