package app

import "testing"

func TestSettingsApplySeparatesPersistedPortsFromBaseConfig(t *testing.T) {
	base := Config{
		HTTPEnabled:  true,
		HTTPAddr:     "127.0.0.1:9180",
		TCPEnabled:   true,
		TCPAddr:      ":9100",
		TargetMode:   "system-print-queue",
		PrinterName:  "",
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
		PrinterName: "Kitchen",
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
	if got.PrinterName != "" {
		t.Fatalf("expected printer name to be cleared outside system-print-queue, got %q", got.PrinterName)
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
		PrinterName: "Kitchen",
		Transform:   "epson-escpos",
	}

	got := newRuntimeState(config)
	if got.TargetMode != "system-print-queue" {
		t.Fatalf("unexpected target mode %q", got.TargetMode)
	}
	if got.Transform != "epson-escpos" {
		t.Fatalf("unexpected transform %q", got.Transform)
	}
	if got.PrinterName != "Kitchen" {
		t.Fatalf("unexpected printer name %q", got.PrinterName)
	}
	if !got.HTTPEnabled || got.TCPEnabled {
		t.Fatalf("unexpected listener state: %+v", got)
	}
}

func TestHealthInfoIncludesQueueForSystemPrintQueue(t *testing.T) {
	application := &App{active: runtimeState{
		TCPAddr:     ":9100",
		Station:     "Kitchen",
		TargetMode:  "system-print-queue",
		PrinterName: "Zebra",
	}}

	got := application.healthInfo()
	if got.Station != "Kitchen" {
		t.Fatalf("expected station Kitchen, got %q", got.Station)
	}
	if got.TCPPort != "9100" {
		t.Fatalf("expected TCP port 9100, got %q", got.TCPPort)
	}
	if got.Queue != "Zebra" {
		t.Fatalf("expected queue Zebra, got %q", got.Queue)
	}
}

func TestHealthInfoOmitsQueueOutsideSystemPrintQueue(t *testing.T) {
	application := &App{active: runtimeState{
		TCPAddr:     ":9100",
		TargetMode:  "http-proxy",
		PrinterName: "Zebra",
	}}

	if got := application.healthInfo(); got.Queue != "" {
		t.Fatalf("expected queue to be empty, got %q", got.Queue)
	}
}

func TestSettingsFromFormClearsPrinterOutsideSystemQueue(t *testing.T) {
	got, err := settingsFromForm("HTTP Proxy", "Kitchen", "None", true, "9180", true, "9100", "", false)
	if err != nil {
		t.Fatalf("settingsFromForm() error = %v", err)
	}
	if got.PrinterName != "" {
		t.Fatalf("expected printer name to be cleared, got %q", got.PrinterName)
	}
}

func TestSettingsFromFormRequiresPrinterForSystemQueue(t *testing.T) {
	_, err := settingsFromForm("System Print Queue", "", "None", true, "9180", true, "9100", "", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "select a printer" {
		t.Fatalf("unexpected error %q", err)
	}
}

func TestSettingsFromFormKeepsAutoStartValue(t *testing.T) {
	got, err := settingsFromForm("HTTP Proxy", "", "None", true, "9180", true, "9100", "", true)
	if err != nil {
		t.Fatalf("settingsFromForm() error = %v", err)
	}
	if !got.AutoStart {
		t.Fatal("expected auto-start to be enabled")
	}
}

func TestSettingsFromFormKeepsStationOptional(t *testing.T) {
	got, err := settingsFromForm("HTTP Proxy", "", "None", true, "9180", true, "9100", " Kitchen ", false)
	if err != nil {
		t.Fatalf("settingsFromForm() error = %v", err)
	}
	if got.Station != "Kitchen" {
		t.Fatalf("expected station to be trimmed, got %q", got.Station)
	}
}

func TestWindowsAutoStartCommandQuotesExecutablePath(t *testing.T) {
	got := windowsAutoStartCommand(`C:\Program Files\Bananas POS\bananas-pos.exe`)
	want := `"C:\Program Files\Bananas POS\bananas-pos.exe"`
	if got != want {
		t.Fatalf("unexpected command: got %q want %q", got, want)
	}
}

func TestWindowsAutoStartCommandEscapesEmbeddedQuotes(t *testing.T) {
	got := windowsAutoStartCommand(`C:\Apps\Bananas "Beta"\bananas-pos.exe`)
	want := `"C:\Apps\Bananas \"Beta\"\bananas-pos.exe"`
	if got != want {
		t.Fatalf("unexpected command: got %q want %q", got, want)
	}
}
