package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"brightnet/apps/agent/internal/runtime"
	"brightnet/internal/discover"
)

func main() {
	coreURL := os.Getenv("BRIGHTNET_CORE_URL")
	bncpAddr := os.Getenv("BRIGHTNET_BNCP_ADDR")
	interactive := terminal(os.Stdin)
	if coreURL == "" && bncpAddr == "" {
		addr, err := findCore(interactive)
		if err != nil {
			slog.Error("aucun Core", "err", err)
			os.Exit(1)
		}
		bncpAddr = addr
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
		DHCPScopes:    os.Getenv("BRIGHTNET_DHCP_SCOPES"),
	}
	if cfg.PairingCode == "" && !enrolled(cfg.StatePath) {
		if !interactive {
			slog.Error("code d'appairage manquant")
			os.Exit(1)
		}
		fmt.Fprint(os.Stderr, "Code d'appairage : ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "" {
			slog.Error("code d'appairage manquant")
			os.Exit(1)
		}
		cfg.PairingCode = strings.TrimSpace(line)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	slog.Info("BrightNet Node démarré", "bncp", cfg.BNCPAddr, "http", cfg.CoreURL)
	if err := runtime.Run(ctx, cfg); err != nil && ctx.Err() == nil {
		slog.Error("agent arrêté", "err", err)
		os.Exit(1)
	}
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

func enrolled(path string) bool {
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var state struct {
		AgentID string `json:"agent_id"`
	}
	if json.Unmarshal(body, &state) != nil {
		return false
	}
	return state.AgentID != ""
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
