package target

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyHTTPHealthRejectsNonSuccessStatus(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusBadGateway)
	}))
	defer upstream.Close()

	target, err := NewProxyHTTP(upstream.URL)
	if err != nil {
		t.Fatalf("NewProxyHTTP() error = %v", err)
	}

	err = target.Health(context.Background())
	if err == nil {
		t.Fatal("expected health error")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected upstream status in error, got %q", err)
	}
}

func TestProxyHTTPHealthAcceptsSuccessStatus(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	target, err := NewProxyHTTP(upstream.URL)
	if err != nil {
		t.Fatalf("NewProxyHTTP() error = %v", err)
	}

	if err := target.Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
}
