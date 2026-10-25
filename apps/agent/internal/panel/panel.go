package panel

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"brightnet/apps/agent/internal/install"
	"brightnet/apps/agent/internal/runtime"
	"brightnet/internal/desk"
	"brightnet/internal/discover"
)

//go:embed index.html
var page embed.FS

var (
	OnWindow func()
	OnStop   func()
)

type Settings struct {
	BNCPAddr      string          `json:"bncp_addr"`
	CoreURL       string          `json:"core_url"`
	PairingCode   string          `json:"pairing_code"`
	StatePath     string          `json:"state_path"`
	DockerSocket  string          `json:"docker_socket"`
	MachineIDPath string          `json:"machine_id_path"`
	HostnamePath  string          `json:"hostname_path"`
	DHCPScopes    string          `json:"dhcp_scopes"`
	Events        *runtime.Events `json:"events,omitempty"`
}

type Status struct {
	mu         sync.Mutex
	Connected  bool
	Core       string
	Peers      []string
	Error      string
	Lines      []string
	Hostname   string
	Interfaces int
	Devices    int
	Containers int
}

func (s *Status) Line(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Lines = append(s.Lines, time.Now().Format("15:04:05")+"  "+line)
	if len(s.Lines) > 40 {
		s.Lines = append([]string(nil), s.Lines[len(s.Lines)-40:]...)
	}
}

func (s *Status) Link(connected bool, core string, peers []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Connected = connected
	s.Core = core
	s.Peers = append([]string(nil), peers...)
	if connected {
		s.Error = ""
	}
}

func (s *Status) Observe(hostname string, interfaces, devices, containers int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Hostname = hostname
	s.Interfaces = interfaces
	s.Devices = devices
	s.Containers = containers
}

func (s *Status) Trouble(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Error = err
	if err != "" {
		s.Connected = false
	}
}

func (s *Status) view() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	peers := append([]string(nil), s.Peers...)
	lines := append([]string(nil), s.Lines...)
	return map[string]any{
		"connected": s.Connected, "core": s.Core, "peers": peers, "error": s.Error, "lines": lines,
		"hostname": s.Hostname, "interfaces": s.Interfaces, "devices": s.Devices, "containers": s.Containers,
	}
}

type Control struct {
	mu        sync.Mutex
	settings  Settings
	status    *Status
	cancel    context.CancelFunc
	again     bool
	wake      chan struct{}
	container bool
	file      string
}

func New(settings Settings, container bool, file string) *Control {
	return &Control{settings: settings, status: &Status{}, wake: make(chan struct{}, 1), container: container, file: file}
}

func (c *Control) Status() *Status { return c.status }

func (c *Control) Current() Settings {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.settings
}

func (c *Control) Apply(next Settings) error {
	next = withDefaults(next)
	if err := Save(c.file, next); err != nil {
		return err
	}
	c.mu.Lock()
	c.settings = next
	c.again = true
	cancel := c.cancel
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
	if cancel != nil {
		cancel()
	}
	return nil
}

func (c *Control) Loop(ctx context.Context, hold bool) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		c.mu.Lock()
		cfg := c.runtime()
		runCtx, cancel := context.WithCancel(ctx)
		c.cancel = cancel
		c.again = false
		c.mu.Unlock()
		err := runtime.Run(runCtx, cfg)
		cancel()
		c.mu.Lock()
		again := c.again
		c.mu.Unlock()
		if again {
			continue
		}
		if ctx.Err() != nil {
			return nil
		}
		if err != nil && !hold {
			c.status.Trouble(err.Error())
			return err
		}
		if err != nil {
			c.status.Trouble(err.Error())
		}
		select {
		case <-ctx.Done():
			return nil
		case <-c.wake:
		}
	}
}

func (c *Control) runtime() runtime.Config {
	settings := c.settings
	return runtime.Config{
		CoreURL: settings.CoreURL, BNCPAddr: settings.BNCPAddr, PairingCode: settings.PairingCode,
		StatePath: settings.StatePath, DockerSocket: settings.DockerSocket, MachineIDPath: settings.MachineIDPath,
		HostnamePath: settings.HostnamePath, DHCPScopes: settings.DHCPScopes, InContainer: c.container,
		Events: settings.Events,
		Log:    c.status.Line, Link: c.status.Link, Trouble: c.status.Trouble, Observe: c.status.Observe,
	}
}

func Serve(ctx context.Context, ctrl *Control) (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /face.css", func(w http.ResponseWriter, _ *http.Request) { desk.Asset(w, "face.css") })
	mux.HandleFunc("GET /frame.js", func(w http.ResponseWriter, _ *http.Request) { desk.Asset(w, "frame.js") })
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		pageFile, err := page.Open("index.html")
		if err != nil {
			http.Error(w, "page indisponible", http.StatusInternalServerError)
			return
		}
		defer pageFile.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.Copy(w, pageFile)
	})
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, ctrl.Current())
	})
	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var next Settings
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&next); err != nil {
			http.Error(w, "réglage refusé", http.StatusBadRequest)
			return
		}
		if err := ctrl.Apply(next); err != nil {
			http.Error(w, "réglage non enregistré", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, ctrl.Status().view())
	})
	mux.HandleFunc("GET /api/install", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, ctrl.installState())
	})
	mux.HandleFunc("POST /api/install", func(w http.ResponseWriter, _ *http.Request) {
		state := ctrl.installEnable()
		if state.Ready {
			desk.Stay(true)
		}
		writeJSON(w, state)
	})
	mux.HandleFunc("POST /api/window", func(w http.ResponseWriter, _ *http.Request) {
		if OnWindow != nil {
			OnWindow()
		}
		writeJSON(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/stop", func(w http.ResponseWriter, _ *http.Request) {
		if OnStop != nil {
			OnStop()
		}
		writeJSON(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/discover", func(w http.ResponseWriter, r *http.Request) {
		found, err := discover.Browse(r.Context())
		if found == nil {
			found = []discover.Core{}
		}
		if err != nil && len(found) == 0 {
			writeJSON(w, map[string]any{"cores": []discover.Core{}, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"cores": found})
	})
	var ln net.Listener
	var err error
	for port := 17820; port < 17830; port++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			break
		}
	}
	if ln == nil {
		return "", err
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(ln) }()
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	return "http://" + ln.Addr().String(), nil
}

func (c *Control) installState() install.State {
	exe, err := os.Executable()
	if err != nil {
		return install.State{}
	}
	return install.Inspect(exe)
}

func (c *Control) installEnable() install.State {
	exe, err := os.Executable()
	if err != nil {
		return install.State{Error: "Le programme est introuvable."}
	}
	return install.Enable(exe)
}

func ConfigFile(statePath string) string {
	dir := filepath.Dir(statePath)
	if dir == "" || dir == "." {
		dir = "data"
	}
	return filepath.Join(dir, "node-gui.json")
}

func Load(path string) (Settings, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Settings{}, err
	}
	var settings Settings
	if err := json.Unmarshal(body, &settings); err != nil {
		return Settings{}, err
	}
	return withDefaults(settings), nil
}

func Save(path string, settings Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o600)
}

func withDefaults(settings Settings) Settings {
	if settings.StatePath == "" {
		settings.StatePath = "./data/agent-state.json"
	}
	if settings.DockerSocket == "" {
		settings.DockerSocket = "/var/run/docker.sock"
	}
	if settings.MachineIDPath == "" {
		settings.MachineIDPath = "/etc/machine-id"
	}
	return settings
}

func Missing(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
