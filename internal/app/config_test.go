package app

import "testing"

func TestSettingsApplySeparatesPersistedPortsFromBaseConfig(t *testing.T) {
	base := Config{
		HTTPEnabled:  true,
		HTTPAddr:     "127.0.0.1:9180",
		TCPEnabled:   true,
		TCPAddr:      ":9100",
		TargetMode:   "system-print-queue",
		Transform:    "",
		ProxyHTTPURL: "http://localhost:9100",
		EmulatorDPMM: 8,
	}

	settings := settingsState{
		HTTPEnabled: false,
		HTTPPort:    "9280",
		TCPEnabled:  true,
		TCPPort:     "9200",
		TargetMode:  "http-proxy",
		Transform:   "ignored",
	}

	got := settings.apply(base)
	if got.HTTPEnabled {
		t.Fatal("expected HTTP to be disabled")
	}
	if got.HTTPAddr != "127.0.0.1:9280" {
		t.Fatalf("expected HTTP addr to preserve host, got %q", got.HTTPAddr)
	}
	if got.TCPAddr != ":9200" {
		t.Fatalf("expected TCP addr to update port, got %q", got.TCPAddr)
	}
	if got.TargetMode != "http-proxy" {
		t.Fatalf("expected target mode http-proxy, got %q", got.TargetMode)
	}
	if got.Transform != "" {
		t.Fatalf("expected transform to be cleared outside system-print-queue, got %q", got.Transform)
	}
}

func TestNewRuntimeStateTracksActiveRuntimeConfig(t *testing.T) {
	config := Config{
		HTTPEnabled: true,
		HTTPAddr:    ":9180",
		TCPEnabled:  false,
		TCPAddr:     ":9100",
		TargetMode:  "system-print-queue",
		Transform:   "epson-escpos",
	}

	got := newRuntimeState(config)
	if got.TargetMode != "system-print-queue" {
		t.Fatalf("unexpected target mode %q", got.TargetMode)
	}
	if got.Transform != "epson-escpos" {
		t.Fatalf("unexpected transform %q", got.Transform)
	}
	if !got.HTTPEnabled || got.TCPEnabled {
		t.Fatalf("unexpected listener state: %+v", got)
	}
}
