package httpinput

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"bananas-pos/internal/job"
)

type stubTarget struct{}

func (stubTarget) Name() string { return "stub" }

func (stubTarget) Send(context.Context, job.PrintJob) error { return nil }

func (stubTarget) Health(context.Context) error { return nil }

func TestCORSPreflight(t *testing.T) {
	server := NewServer(":0", stubTarget{})

	req := httptest.NewRequest(http.MethodOptions, "/zpl", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, X-Test")

	rec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("unexpected allow methods %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, X-Test" {
		t.Fatalf("unexpected allow headers %q", got)
	}
}

func TestCORSHeadersOnPrintResponse(t *testing.T) {
	server := NewServer(":0", stubTarget{})

	req := httptest.NewRequest(http.MethodPost, "/zpl", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	rec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard origin, got %q", got)
	}
}
