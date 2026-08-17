package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kido5217/yolo/internal/server"
)

func TestHealth(t *testing.T) {
	s := server.New("/tmp/work")
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/global/health", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	want := `{"status":"ok"}`
	if strings.TrimSpace(string(body)) != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
}

func TestHealthWithDirectoryHeader(t *testing.T) {
	s := server.New("/tmp/work")
	defer s.Close()
	req := httptest.NewRequest(http.MethodGet, "/global/health", nil)
	req.Header.Set("x-yolo-directory", "%2Ftmp%2Fother")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
