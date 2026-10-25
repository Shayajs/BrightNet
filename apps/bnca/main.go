package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"brightnet/internal/desk"
)

func main() {
	session := &coreSession{client: &http.Client{Timeout: 20 * time.Second}}
	jar, err := cookiejar.New(nil)
	if err != nil {
		slog.Error("session impossible", "err", err)
		os.Exit(1)
	}
	session.client.Jar = jar
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", page)
	mux.HandleFunc("GET /face.css", func(w http.ResponseWriter, _ *http.Request) { desk.Asset(w, "face.css") })
	mux.HandleFunc("GET /frame.js", func(w http.ResponseWriter, _ *http.Request) { desk.Asset(w, "frame.js") })
	mux.HandleFunc("GET /api/session", session.view)
	mux.HandleFunc("POST /api/login", session.login)
	mux.HandleFunc("POST /api/logout", session.logout)
	mux.HandleFunc("GET /api/admin", session.admin)
	mux.HandleFunc("DELETE /api/agents/{id}", session.forget)
	listener, err := net.Listen("tcp", "127.0.0.1:1721")
	if err != nil {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		slog.Error("fenêtre BNCA indisponible", "err", err)
		os.Exit(1)
	}
	server := &http.Server{Handler: localOnly(mux), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { _ = server.Serve(listener) }()
	url := "http://" + listener.Addr().String() + "/"
	if desk.Open("BrightNet Core Admin", url) {
		stop()
	}
	<-ctx.Done()
	shut, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(shut)
}

func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || host != "127.0.0.1" && host != "::1" {
			http.Error(w, "réservé à cette machine", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type coreSession struct {
	mu     sync.Mutex
	base   string
	client *http.Client
}

func (s *coreSession) view(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	base := s.base
	s.mu.Unlock()
	writeJSON(w, map[string]any{"ok": base != "", "url": base})
}

func (s *coreSession) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
		http.Error(w, "accès refusé", http.StatusBadRequest)
		return
	}
	base, err := coreURL(body.URL)
	if err != nil {
		http.Error(w, "adresse du Core refusée", http.StatusBadRequest)
		return
	}
	payload, _ := json.Marshal(map[string]string{"username": body.Username, "password": body.Password})
	res, err := s.client.Post(base+"/api/v1/login", "application/json", strings.NewReader(string(payload)))
	if err != nil {
		http.Error(w, "Core injoignable", http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode != http.StatusOK {
		http.Error(w, "accès refusé", res.StatusCode)
		return
	}
	s.mu.Lock()
	s.base = base
	s.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true, "url": base})
}

func (s *coreSession) logout(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	s.base = ""
	s.mu.Unlock()
	jar, _ := cookiejar.New(nil)
	s.client.Jar = jar
	writeJSON(w, map[string]any{"ok": false})
}

func (s *coreSession) admin(w http.ResponseWriter, r *http.Request) {
	s.forward(w, r, http.MethodGet, "/api/v1/admin")
}

func (s *coreSession) forget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !safeID(id) {
		http.Error(w, "node inconnu", http.StatusBadRequest)
		return
	}
	s.forward(w, r, http.MethodDelete, "/api/v1/agents/"+id)
}

func (s *coreSession) forward(w http.ResponseWriter, r *http.Request, method, path string) {
	s.mu.Lock()
	base := s.base
	s.mu.Unlock()
	if base == "" {
		http.Error(w, "accès refusé", http.StatusUnauthorized)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), method, base+path, nil)
	if err != nil {
		http.Error(w, "Core injoignable", http.StatusBadGateway)
		return
	}
	res, err := s.client.Do(req)
	if err != nil {
		http.Error(w, "Core injoignable", http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	for key, values := range res.Header {
		if strings.EqualFold(key, "Content-Type") {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}

func coreURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "http://192.168.1.3:1717"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return "", errors.New("adresse refusée")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("adresse refusée")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func safeID(id string) bool {
	if id == "" || len(id) > 80 {
		return false
	}
	for _, char := range id {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

//go:embed page.html
var pages embed.FS

func page(w http.ResponseWriter, _ *http.Request) {
	file, err := pages.Open("page.html")
	if err != nil {
		http.Error(w, "page indisponible", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, file)
}
