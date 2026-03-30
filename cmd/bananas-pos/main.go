package main

import (
	"log"
	"os"
	"strconv"

	"bananas-pos/internal/app"
	"bananas-pos/internal/meta"
	"bananas-pos/internal/singleinstance"
	"bananas-pos/internal/trayicon"
	"bananas-pos/internal/ui"
)

func main() {
	detached, err := detachIfNeeded()
	if err != nil {
		log.Fatalf("detach application: %v", err)
	}
	if detached {
		return
	}

	instance, alreadyRunning, err := singleinstance.Acquire("bananas-pos")
	if err != nil {
		log.Fatalf("acquire instance lock: %v", err)
	}
	if alreadyRunning {
		ui.ShowMessage(meta.AppName, "The app is already running.", trayicon.Resource())
		return
	}
	defer instance.Release()

	config := app.Config{
		HTTPEnabled:  envOrDefault("HTTP_ENABLED", "true") != "false",
		HTTPAddr:     envOrDefault("HTTP_LISTEN_ADDR", envOrDefault("LISTEN_ADDR", ":9180")),
		TCPEnabled:   envOrDefault("TCP_ENABLED", "true") != "false",
		TCPAddr:      envOrDefault("TCP_LISTEN_ADDR", ":9100"),
		TargetMode:   envOrDefault("TARGET_MODE", "system-print-queue"),
		ProxyHTTPURL: envOrDefault("PROXY_HTTP_URL", envOrDefault("TARGET_URL", "http://localhost:9100")),
		EmulatorDPMM: envOrDefaultInt("EMULATOR_DPMM", 8),
	}

	application, err := app.New(config)
	if err != nil {
		log.Fatalf("create application: %v", err)
	}

	if err := application.Run(); err != nil {
		log.Fatalf("application failed: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
