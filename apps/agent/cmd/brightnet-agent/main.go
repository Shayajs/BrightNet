package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"brightnet/apps/agent/internal/panel"
	"brightnet/internal/discover"
)

func main() {
	statePath := env("BRIGHTNET_STATE", "./data/agent-state.json")
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
	gui := wantGUI()
	if settings.BNCPAddr == "" && settings.CoreURL == "" && !gui {
		addr, err := findCore(terminal(os.Stdin))
		if err != nil {
			slog.Error("aucun Core", "err", err)
			os.Exit(1)
		}
		settings.BNCPAddr = addr
	}
	ctrl := panel.New(settings, os.Getenv("BRIGHTNET_IN_CONTAINER") == "1", file)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if gui {
		url, err := panel.Serve(ctx, ctrl)
		if err != nil {
			slog.Error("fenêtre Node indisponible", "err", err)
		} else {
			fmt.Println("Node Parameter  " + url)
			openBrowser(url)
		}
	}
	fmt.Println("BrightNet Node. Le courant passe vers les autres Node du réseau.")
	if err := ctrl.Loop(ctx, gui); err != nil && ctx.Err() == nil {
		slog.Error("agent arrêté", "err", err)
		os.Exit(1)
	}
}

func wantGUI() bool {
	if hasFlag("--cli") || os.Getenv("BRIGHTNET_GUI") == "0" || os.Getenv("BRIGHTNET_IN_CONTAINER") == "1" {
		return false
	}
	if hasFlag("--gui") || os.Getenv("BRIGHTNET_GUI") == "1" {
		return true
	}
	return terminal(os.Stdin)
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args[1:] {
		if arg == flag {
			return true
		}
	}
	return false
}

func findCore(interactive bool) (string, error) {
	cores, err := discover.Browse(context.Background())
	if err != nil {
		slog.Warn("recherche des Core incomplète", "err", err)
	}
	if len(cores) > 0 {
		core, err := discover.Choose(cores, os.Stdin, os.Stdout, interactive)
		if err != nil {
			return "", err
		}
		return core.Addr, nil
	}
	if discover.Local() {
		fmt.Println("Core local  127.0.0.1:1718")
		return "127.0.0.1:1718", nil
	}
	return "", discover.ErrNone
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

func terminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
