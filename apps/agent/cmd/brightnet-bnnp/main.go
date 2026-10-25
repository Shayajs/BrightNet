package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"brightnet/apps/agent/internal/install"
	"brightnet/apps/agent/internal/panel"
	"brightnet/internal/desk"
	"brightnet/internal/discover"
)

func main() {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	if len(os.Args) > 1 && os.Args[1] == "--setup" {
		if err := install.OpenFirewall(exe); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
	background := len(os.Args) > 1 && os.Args[1] == "--background"
	base := filepath.Join(configDir(), "BrightNet")
	if !install.Claim() {
		install.Ask(filepath.Join(base, "panel.url"))
		return
	}
	defer install.Release()

	statePath := env("BRIGHTNET_STATE", filepath.Join(base, "node-state.json"))
	file := panel.ConfigFile(statePath)
	settings := panel.Settings{
		BNCPAddr: env("BRIGHTNET_BNCP_ADDR", ""), CoreURL: env("BRIGHTNET_CORE_URL", ""),
		PairingCode: os.Getenv("BRIGHTNET_PAIRING_CODE"), StatePath: statePath,
		DockerSocket:  env("BRIGHTNET_DOCKER_SOCKET", "/var/run/docker.sock"),
		MachineIDPath: env("BRIGHTNET_MACHINE_ID_PATH", "/etc/machine-id"),
		HostnamePath:  os.Getenv("BRIGHTNET_HOSTNAME_PATH"), DHCPScopes: os.Getenv("BRIGHTNET_DHCP_SCOPES"),
	}
	if saved, err := panel.Load(file); err == nil {
		settings = saved
	}
	if settings.BNCPAddr == "" && settings.CoreURL == "" {
		if cores, err := discover.Browse(context.Background()); err == nil && len(cores) == 1 {
			settings.BNCPAddr = cores[0].Addr
		}
	}
	ctrl := panel.New(settings, false, file)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	openWindow := make(chan struct{}, 1)
	panel.OnWindow = func() {
		if desk.Active() {
			desk.Reveal()
			return
		}
		select {
		case openWindow <- struct{}{}:
		default:
		}
	}
	panel.OnStop = func() {
		desk.Stay(false)
		desk.Quit()
		stop()
	}
	url, err := panel.Serve(ctx, ctrl)
	if err != nil {
		slog.Error("fenêtre BNNP indisponible", "err", err)
		os.Exit(1)
	}
	install.Remember(filepath.Join(base, "panel.url"), url)
	go func() {
		if err := ctrl.Loop(ctx, true); err != nil && ctx.Err() == nil {
			slog.Error("node arrêté", "err", err)
		}
	}()
	ready := install.Inspect(exe).Ready
	desk.Stay(ready)
	show := func() {
		if desk.Open("BrightNet Node Parameter", url) {
			stop()
		}
	}
	if background && ready {
		select {
		case <-ctx.Done():
		case <-openWindow:
			show()
		}
		return
	}
	show()
	<-ctx.Done()
}

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return dir
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
