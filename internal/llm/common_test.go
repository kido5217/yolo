package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func ctx0(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func sseServer(t *testing.T, dir, fixture string, checks ...func(*http.Request)) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", dir, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, c := range checks {
			c(r)
		}
		fl, _ := w.(http.Flusher)
		_, _ = w.Write(data)
		fl.Flush()
	}))
}

func sseServerSplit(t *testing.T, dir, fixture string, checks ...func(*http.Request)) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", dir, fixture))
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, c := range checks {
			c(r)
		}
		fl, _ := w.(http.Flusher)
		// flush mid-frame (1 byte at a time) to exercise the incremental reader
		for _, b := range data {
			_, _ = w.Write([]byte{b})
		}
		fl.Flush()
	}))
}

func stream(t *testing.T, d Driver, req Request) PartStream {
	t.Helper()
	s, err := d.Stream(ctx0(t), req)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	return s
}

func collect(t *testing.T, s PartStream) []Part {
	t.Helper()
	// one ctx bounds the WHOLE drain: a stream that emits parts forever
	// without Finish fails fast here instead of getting a fresh 10 s
	// context per Next call.
	ctx := ctx0(t)
	var out []Part
	for {
		p, err := s.Next(ctx)
		if err != nil {
			break
		}
		out = append(out, p)
		if p.Finish != "" {
			break
		}
	}
	return out
}
