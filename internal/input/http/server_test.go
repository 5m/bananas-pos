package httpinput

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bananas-pos/internal/job"
)

type stubTarget struct{}

func (stubTarget) Name() string { return "stub" }

func (stubTarget) Send(context.Context, job.PrintJob) error { return nil }

func (stubTarget) Health(context.Context) error { return nil }

func TestHealthIncludesStationAndTCPPort(t *testing.T) {
	server := NewServer(":0", stubTarget{}, HealthInfo{Station: "Kitchen", TCPPort: "9100"})

	req := httptest.NewRequest(http.MethodGet, "/_/health", nil)
	rec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body["station"] != "Kitchen" {
		t.Fatalf("expected station Kitchen, got %#v", body["station"])
	}
	if body["tcp_port"] != "9100" {
		t.Fatalf("expected tcp_port 9100, got %#v", body["tcp_port"])
	}
}

func TestCORSPreflight(t *testing.T) {
	server := NewServer(":0", stubTarget{}, HealthInfo{})

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
	server := NewServer(":0", stubTarget{}, HealthInfo{})

	req := httptest.NewRequest(http.MethodPost, "/zpl", strings.NewReader("^XA^XZ"))
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

func TestRejectsEmptyPrintPayload(t *testing.T) {
	server := NewServer(":0", stubTarget{}, HealthInfo{})

	req := httptest.NewRequest(http.MethodPost, "/zpl", http.NoBody)
	rec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "print payload is empty") {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestPrintSplitsMultipleLabels(t *testing.T) {
	target := &captureTarget{}
	server := NewServer(":0", target, HealthInfo{})

	req := httptest.NewRequest(
		http.MethodPost,
		"/zpl",
		strings.NewReader("^XA^FO0,0^FDONE^FS^XZ^XA^FO0,0^FDTWO^FS^XZ"),
	)
	rec := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if len(target.jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(target.jobs))
	}
}

type captureTarget struct {
	jobs []job.PrintJob
}

func (t *captureTarget) Name() string { return "capture" }

func (t *captureTarget) Send(_ context.Context, printJob job.PrintJob) error {
	printJob.Raw = append([]byte(nil), printJob.Raw...)
	t.jobs = append(t.jobs, printJob)
	return nil
}

func (t *captureTarget) Health(context.Context) error { return nil }
