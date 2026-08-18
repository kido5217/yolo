package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/kido5217/yolo/internal/tui/client"
)

func TestClientScopingAndErrors(t *testing.T) {
	var gotDir, gotRoute string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDir = r.Header.Get("x-yolo-directory")
		gotRoute = r.Method + " " + r.URL.Path
		switch {
		case r.URL.Path == "/session" && r.Method == "GET":
			_, _ = w.Write([]byte(`[{"id":"ses_1","title":"T"}]`))
		case r.URL.Path == "/session/ses_x/message" && r.Method == "POST":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":{"message":"busy"}}`))
		case r.URL.Path == "/session/ses_missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"not found"}}`))
		case r.URL.Path == "/session/ses_bad":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"bad"}}`))
		}
	}))
	t.Cleanup(srv.Close)
	ctx := context.Background()
	c := client.New(srv.URL, "/abs/dir")

	got, err := c.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	// The wire carries the URL-escaped directory (upstream: encodeURIComponent;
	// the server PathUnescapes it), so slashes are encoded too.
	if want := url.PathEscape("/abs/dir"); gotDir != want {
		t.Fatalf("dir header = %q, want %s", gotDir, want)
	}
	if _, err := url.PathUnescape(gotDir); err != nil || gotDir == "/abs/dir" {
		t.Fatalf("dir header not escaped: %q", gotDir)
	}
	if gotRoute != "GET /session" {
		t.Fatalf("route = %q", gotRoute)
	}
	if len(got) != 1 || got[0].ID != "ses_1" || got[0].Title != "T" {
		t.Fatalf("sessions = %+v", got)
	}

	gotRoute = ""
	if _, err := c.SendMessage(ctx, "ses_x", "hi"); !errors.Is(err, client.ErrBusy) {
		t.Fatalf("SendMessage err = %v, want ErrBusy", err)
	}
	if gotRoute != "POST /session/ses_x/message" {
		t.Fatalf("send route = %q", gotRoute)
	}
	if _, err := c.GetSession(ctx, "ses_missing"); !errors.Is(err, client.ErrNotFound) {
		t.Fatalf("GetSession err = %v, want ErrNotFound", err)
	}
	if _, err := c.GetSession(ctx, "ses_bad"); !errors.Is(err, client.ErrBadRequest) {
		t.Fatalf("GetSession err = %v, want ErrBadRequest", err)
	}
}
