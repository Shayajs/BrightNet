package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"brightnet/internal/bncp"
	"brightnet/internal/httpserver"
	"brightnet/internal/hub"
	"brightnet/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "health" {
		os.Exit(health())
	}
	listen := env("BRIGHTNET_LISTEN", "127.0.0.1:8787")
	uiListen := env("BRIGHTNET_UI_LISTEN", "127.0.0.1:1717")
	bncpListen := env("BRIGHTNET_BNCP_LISTEN", "127.0.0.1:1718")
	webRoot := os.Getenv("BRIGHTNET_WEB_ROOT")
	data := env("BRIGHTNET_DATA", "./data/brightnet.db")
	cert, key := os.Getenv("BRIGHTNET_TLS_CERT"), os.Getenv("BRIGHTNET_TLS_KEY")
	if (cert == "") != (key == "") {
		slog.Error("certificat et clé TLS doivent être fournis ensemble")
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(data), 0o755); err != nil {
		slog.Error("données inaccessibles", "err", err)
		os.Exit(1)
	}
	st, err := store.Open(data)
	if err != nil {
		slog.Error("base inaccessible", "err", err)
		os.Exit(1)
	}
	defer st.Close()
	ctx := context.Background()
	if err := st.EnsureAdmin(ctx, os.Getenv("BRIGHTNET_ADMIN_USER"), os.Getenv("BRIGHTNET_ADMIN_PASSWORD")); err != nil {
		slog.Error("administrateur non créé", "err", err)
		os.Exit(1)
	}
	if err := st.UpsertDevPairingCode(ctx, os.Getenv("BRIGHTNET_DEV_PAIRING_CODE"), 24*time.Hour); err != nil {
		slog.Error("code de développement non préparé", "err", err)
		os.Exit(1)
	}
	h := hub.New()
	api := httpserver.New(st, h).Handler()
	apiSrv := &http.Server{Addr: listen, Handler: api, ReadHeaderTimeout: 5 * time.Second}
	uiSrv := &http.Server{Addr: uiListen, Handler: httpserver.WithUI(api, webRoot), ReadHeaderTimeout: 5 * time.Second}
	bncpLn, err := listenTLS(bncpListen, cert, key)
	if err != nil {
		slog.Error("écoute BNCP impossible", "err", err)
		os.Exit(1)
	}
	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go serveHTTP(apiSrv, cert, key, "API")
	go serveHTTP(uiSrv, cert, key, "interface")
	go func() {
		slog.Info("BNCP en écoute", "addr", bncpListen)
		if err := bncp.NewServer(st, h).Serve(stop, bncpLn); err != nil && stop.Err() == nil {
			slog.Error("BNCP arrêté", "err", err)
			os.Exit(1)
		}
	}()
	<-stop.Done()
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = apiSrv.Shutdown(shutdown)
	_ = uiSrv.Shutdown(shutdown)
	_ = bncpLn.Close()
}

func serveHTTP(srv *http.Server, cert, key, name string) {
	slog.Info("BrightNet Core en écoute", "role", name, "addr", srv.Addr)
	var err error
	if cert != "" {
		err = srv.ListenAndServeTLS(cert, key)
	} else {
		err = srv.ListenAndServe()
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("serveur arrêté", "role", name, "err", err)
		os.Exit(1)
	}
}

func listenTLS(addr, cert, key string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil || cert == "" {
		return ln, err
	}
	pair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	return tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}), nil
}

func health() int {
	url := env("BRIGHTNET_HEALTH_URL", "http://127.0.0.1:8787/api/v1/health")
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
