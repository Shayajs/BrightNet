package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"brightnet/apps/agent/internal/runtime"
)

func main() {
	coreURL := os.Getenv("BRIGHTNET_CORE_URL")
	bncpAddr := os.Getenv("BRIGHTNET_BNCP_ADDR")
	if coreURL == "" && bncpAddr == "" {
		bncpAddr = "127.0.0.1:1718"
	}
	cfg := runtime.Config{
		CoreURL:       coreURL,
		BNCPAddr:      bncpAddr,
		PairingCode:   os.Getenv("BRIGHTNET_PAIRING_CODE"),
		StatePath:     env("BRIGHTNET_STATE", "./data/agent-state.json"),
		DockerSocket:  env("BRIGHTNET_DOCKER_SOCKET", "/var/run/docker.sock"),
		MachineIDPath: env("BRIGHTNET_MACHINE_ID_PATH", "/etc/machine-id"),
		HostnamePath:  os.Getenv("BRIGHTNET_HOSTNAME_PATH"),
		InContainer:   os.Getenv("BRIGHTNET_IN_CONTAINER") == "1",
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	slog.Info("BrightNet Node démarré", "bncp", cfg.BNCPAddr, "http", cfg.CoreURL)
	if err := runtime.Run(ctx, cfg); err != nil && ctx.Err() == nil {
		slog.Error("agent arrêté", "err", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
