package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

	t.Run("scoping header escaped", func(t *testing.T) {
		if _, err := c.ListSessions(ctx); err != nil {
			t.Fatalf("ListSessions: %v", err)
		}
		// The wire carries the URL-escaped directory (upstream:
		// encodeURIComponent; the server PathUnescapes it), so slashes
		// are encoded too.
		if want := url.PathEscape("/abs/dir"); gotDir != want {
			t.Fatalf("dir header = %q, want %s", gotDir, want)
		}
		if _, err := url.PathUnescape(gotDir); err != nil || gotDir == "/abs/dir" {
			t.Fatalf("dir header not escaped: %q", gotDir)
		}
	})
	t.Run("list decode", func(t *testing.T) {
		got, err := c.ListSessions(ctx)
		if err != nil {
			t.Fatalf("ListSessions: %v", err)
		}
		if gotRoute != "GET /session" {
			t.Fatalf("route = %q", gotRoute)
		}
		if len(got) != 1 || got[0].ID != "ses_1" || got[0].Title != "T" {
			t.Fatalf("sessions = %+v", got)
		}
	})
	t.Run("409 is ErrBusy", func(t *testing.T) {
		if _, err := c.SendMessage(ctx, "ses_x", "hi"); !errors.Is(err, client.ErrBusy) {
			t.Fatalf("SendMessage err = %v, want ErrBusy", err)
		}
		if gotRoute != "POST /session/ses_x/message" {
			t.Fatalf("send route = %q", gotRoute)
		}
	})
	t.Run("404 is ErrNotFound", func(t *testing.T) {
		if _, err := c.GetSession(ctx, "ses_missing"); !errors.Is(err, client.ErrNotFound) {
			t.Fatalf("GetSession err = %v, want ErrNotFound", err)
		}
	})
	t.Run("400 is ErrBadRequest", func(t *testing.T) {
		if _, err := c.GetSession(ctx, "ses_bad"); !errors.Is(err, client.ErrBadRequest) {
			t.Fatalf("GetSession err = %v, want ErrBadRequest", err)
		}
	})
}

// TestSentinelPrefixes pins the "client: " package prefix on the sentinel
// errors (naming-3, deviation 110): the text is what the user sees in the
// status line, so origin must survive wrapping.
func TestSentinelPrefixes(t *testing.T) {
	for _, e := range []error{client.ErrNotFound, client.ErrBusy, client.ErrBadRequest} {
		if !strings.HasPrefix(e.Error(), "client: ") {
			t.Fatalf("sentinel %q lacks the \"client: \" prefix", e.Error())
		}
	}
}
