package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kido5217/yolo/internal/protocol"
	"github.com/kido5217/yolo/internal/server/testutil"
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
	t.Parallel()
	for _, e := range []error{client.ErrNotFound, client.ErrBusy, client.ErrBadRequest} {
		if !strings.HasPrefix(e.Error(), "client: ") {
			t.Fatalf("sentinel %q lacks the \"client: \" prefix", e.Error())
		}
	}
}

// waitShellPart polls the wire message list until the shell's bash part
// reaches a terminal status (the Task-8 wait idiom over the wire: the
// client test package may not reach into storage). The exec goroutine
// finalizes out-of-band, so a bounded wait keeps the legs deterministic.
func waitShellPart(t *testing.T, s *testutil.TestServer, sessionID, dir, partID string) protocol.Part {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, b := testutil.Req(t, s, "GET", "/session/"+sessionID+"/message", dir, "")
		if resp.StatusCode == 200 {
			var msgs []protocol.MessageWithParts
			if err := json.Unmarshal(b, &msgs); err == nil {
				for i := range msgs {
					for j := range msgs[i].Parts {
						p := msgs[i].Parts[j]
						if p.ID == partID && p.State != nil &&
							(p.State.Status == "completed" || p.State.Status == "error") {
							return p
						}
					}
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("part %s did not reach a terminal status within 5s", partID)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestShellRoundTrip pins the client's Service.Shell against the full
// harness (testutil.Boot): the 202 id mapping (message + part ids) and
// the error envelope on 404 (client.ErrNotFound carrying the server
// message).
func TestShellRoundTrip(t *testing.T) {
	t.Parallel()
	s := testutil.Boot(t)
	d := t.TempDir()
	resp, b := testutil.Req(t, s, "POST", "/session", d, `{}`)
	if resp.StatusCode != 201 {
		t.Fatalf("create session: %d %s", resp.StatusCode, b)
	}
	var ses struct{ ID string }
	if err := json.Unmarshal(b, &ses); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, b)
	}
	// the shell run lazily spawns the session's persistent shell; close
	// it (the engine delete path) so its readLoop goroutine does not
	// outlive the test
	t.Cleanup(func() { s.Eng.Close(ses.ID) })

	c := client.New(s.URL, d)
	msgID, partID, err := c.Shell(t.Context(), ses.ID, "echo client-shell")
	if err != nil {
		t.Fatalf("Shell: %v", err)
	}
	if msgID == "" || partID == "" {
		t.Fatalf("ids = %q %q; want both present", msgID, partID)
	}
	// The part id maps onto the engine's bash part row: it finalizes
	// completed with the command output.
	part := waitShellPart(t, s, ses.ID, d, partID)
	if part.State.Status != "completed" {
		t.Fatalf("status = %q; want completed", part.State.Status)
	}
	if !strings.Contains(part.State.Output, "client-shell") {
		t.Fatalf("Output = %q; want it to contain client-shell", part.State.Output)
	}
	// The returned message id maps onto the persisted user message row
	// (the bash part itself lives under the assistant message).
	resp, b = testutil.Req(t, s, "GET", "/session/"+ses.ID+"/message", d, "")
	if resp.StatusCode != 200 {
		t.Fatalf("list messages: %d %s", resp.StatusCode, b)
	}
	var msgs []protocol.MessageWithParts
	if err := json.Unmarshal(b, &msgs); err != nil {
		t.Fatalf("decode: %v (%s)", err, b)
	}
	var user *protocol.MessageWithParts
	for i := range msgs {
		if msgs[i].Info.ID == msgID {
			user = &msgs[i]
		}
	}
	if user == nil || user.Info.Role != "user" {
		t.Fatalf("message %s not present as a user row: %s", msgID, b)
	}

	// 404 → client.ErrNotFound (the envelope message is carried).
	_, _, err = c.Shell(t.Context(), "ses_missing", "echo hi")
	if !errors.Is(err, client.ErrNotFound) {
		t.Fatalf("Shell err = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "session not found") {
		t.Fatalf("err = %q; want the server envelope message", err.Error())
	}
}
