package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"brightnet/apps/agent/internal/collectors/docker"
	"brightnet/apps/agent/internal/collectors/host"
	"brightnet/apps/agent/internal/collectors/lan"
	"brightnet/apps/agent/internal/dockerclient"
	"brightnet/internal/bncp"
	"brightnet/internal/contract"
)

const version = "0.1.0"

type Config struct {
	CoreURL       string
	BNCPAddr      string
	PairingCode   string
	StatePath     string
	DockerSocket  string
	MachineIDPath string
	HostnamePath  string
	InContainer   bool
	DHCPScopes    string
}

type state struct {
	AgentID  string `json:"agent_id"`
	Token    string `json:"token"`
	Interval int    `json:"interval"`
}

type Agent struct {
	cfg        Config
	client     *http.Client
	wire       wire
	mu         sync.RWMutex
	collectMu  sync.Mutex
	state      state
	queue      []contract.DockerEvent
	knownPeers map[string]bool
}

func Run(ctx context.Context, cfg Config) error {
	if cfg.CoreURL == "" && cfg.BNCPAddr == "" {
		return errors.New("BRIGHTNET_CORE_URL ou BRIGHTNET_BNCP_ADDR manquant")
	}
	agent := &Agent{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}}
	if cfg.BNCPAddr != "" {
		client := &bncp.Client{Addr: cfg.BNCPAddr, OnDirect: func(msg bncp.Direct) {
			slog.Info("dBNCP", "de", msg.From, "message", msg.Payload)
		}}
		agent.wire = bncpWire{client: client}
	} else {
		agent.wire = httpWire{agent: agent}
	}
	if err := agent.load(); err != nil {
		return err
	}
	if agent.state.AgentID == "" {
		if err := agent.enroll(ctx); err != nil {
			return err
		}
	}
	if agent.state.Interval <= 0 {
		agent.state.Interval = 20
	}
	go agent.heartbeats(ctx)
	go agent.events(ctx)
	agent.loop(ctx)
	return nil
}

func (a *Agent) loop(ctx context.Context) {
	a.snapshot(ctx)
	ticker := time.NewTicker(time.Duration(a.state.Interval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.snapshot(ctx)
			ticker.Reset(time.Duration(a.state.Interval) * time.Second)
		}
	}
}

func (a *Agent) heartbeats(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cfg, err := a.wire.Heartbeat(ctx, a.agentID(), a.token())
			if errors.Is(err, errUnauthorized) {
				_ = a.enroll(ctx)
			} else if err == nil {
				if cfg.SnapshotIntervalSeconds > 0 {
					a.mu.Lock()
					a.state.Interval = cfg.SnapshotIntervalSeconds
					a.mu.Unlock()
				}
				a.greet(ctx, cfg.Peers)
			}
		}
	}
}

func (a *Agent) events(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if _, err := os.Stat(a.cfg.DockerSocket); err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
		stream, errs := dockerclient.New(a.cfg.DockerSocket).Events(ctx)
		pending := false
		var timer *time.Timer
		for stream != nil {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errs:
				if ok && err != nil {
					slog.Warn("flux Docker interrompu", "err", err)
				}
				errs = nil
			case event, ok := <-stream:
				if !ok {
					stream = nil
					break
				}
				if !event.Relevant() {
					break
				}
				a.mu.Lock()
				a.queue = append(a.queue, event)
				if len(a.queue) > 100 {
					a.queue = a.queue[len(a.queue)-100:]
				}
				batch := append([]contract.DockerEvent(nil), a.queue...)
				a.queue = nil
				a.mu.Unlock()
				if err := a.wire.Events(ctx, a.agentID(), a.token(), batch); errors.Is(err, errUnauthorized) {
					_ = a.enroll(ctx)
				}
				if timer != nil {
					timer.Stop()
				}
				pending = true
				timer = time.AfterFunc(700*time.Millisecond, func() {
					if pending {
						a.snapshot(ctx)
					}
				})
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (a *Agent) snapshot(ctx context.Context) {
	a.collectMu.Lock()
	defer a.collectMu.Unlock()
	hostSnap, hostRun := host.Collect(host.Options{
		MachineIDPath: a.cfg.MachineIDPath, HostnamePath: a.cfg.HostnamePath, InContainer: a.cfg.InContainer,
	})
	if hostRun.Status == "" {
		hostRun = contract.CollectorRun{Collector: "host", Status: "error", Error: "collecteur interrompu"}
	}
	dockerSnap, dockerRun := docker.Collect(ctx, a.cfg.DockerSocket)
	if dockerRun.Status == "" {
		dockerRun = contract.CollectorRun{Collector: "docker", Status: "error", Error: "collecteur interrompu"}
	}
	devices, lanRun, dhcp := lan.Collect(ctx, lan.Options{DHCPScopes: a.cfg.DHCPScopes})
	hostSnap.DHCP = dhcp
	body := contract.Snapshot{
		Version: contract.Version, CollectedAt: time.Now().UTC(),
		Host: hostSnap, Devices: devices, Docker: &dockerSnap,
		CollectorRuns: []contract.CollectorRun{hostRun, dockerRun, lanRun},
	}
	cfg, err := a.wire.Snapshot(ctx, a.agentID(), a.token(), body)
	if errors.Is(err, errUnauthorized) {
		if enrollErr := a.enroll(ctx); enrollErr == nil {
			cfg, err = a.wire.Snapshot(ctx, a.agentID(), a.token(), body)
		}
	}
	if err != nil {
		slog.Warn("instantané non envoyé", "err", err)
		return
	}
	if cfg.SnapshotIntervalSeconds > 0 {
		a.mu.Lock()
		a.state.Interval = cfg.SnapshotIntervalSeconds
		err = a.saveLocked()
		a.mu.Unlock()
		if err != nil {
			slog.Warn("état local non enregistré", "err", err)
		}
	}
}

func (a *Agent) greet(ctx context.Context, peers []string) {
	a.mu.Lock()
	if a.knownPeers == nil {
		a.knownPeers = map[string]bool{}
	}
	var fresh []string
	for _, peer := range peers {
		if !a.knownPeers[peer] {
			a.knownPeers[peer] = true
			fresh = append(fresh, peer)
		}
	}
	a.mu.Unlock()
	for _, peer := range fresh {
		if err := a.wire.Direct(ctx, a.agentID(), a.token(), peer, "présent"); err != nil {
			slog.Debug("dBNCP non ouvert", "pair", peer, "err", err)
		}
	}
}

func (a *Agent) agentID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state.AgentID
}

func (a *Agent) token() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state.Token
}

func (a *Agent) enroll(ctx context.Context) error {
	hostSnap, _ := host.Collect(host.Options{MachineIDPath: a.cfg.MachineIDPath, HostnamePath: a.cfg.HostnamePath, InContainer: a.cfg.InContainer})
	cfg, err := a.wire.Enroll(ctx, contract.EnrollRequest{
		PairingCode: a.cfg.PairingCode, Hostname: hostSnap.Hostname, MachineID: hostSnap.MachineID, Version: version,
	})
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = state{AgentID: cfg.AgentID, Token: cfg.Token, Interval: cfg.SnapshotIntervalSeconds}
	return a.saveLocked()
}

func (a *Agent) post(ctx context.Context, path string, body any, dest any, auth bool) (int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.CoreURL+path, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth {
		a.mu.RLock()
		token := a.state.Token
		a.mu.RUnlock()
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if dest != nil && res.StatusCode < 300 {
		if err := json.NewDecoder(res.Body).Decode(dest); err != nil {
			return res.StatusCode, err
		}
	}
	if res.StatusCode >= 300 {
		return res.StatusCode, errors.New(res.Status)
	}
	return res.StatusCode, nil
}

func (a *Agent) load() error {
	body, err := os.ReadFile(a.cfg.StatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(body, &a.state)
}

func (a *Agent) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(a.cfg.StatePath), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(a.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.cfg.StatePath, body, 0o600)
}
