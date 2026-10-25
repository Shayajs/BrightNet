package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"brightnet/internal/bus"
	"brightnet/internal/store"
)

func TestDownloadServesOnlyCatalog(t *testing.T) {
	dir := t.TempDir()
	name := "brightnet-node-linux-amd64"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("node"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "brightnet.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	srv := New(st, bus.New())
	srv.Downloads = dir
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/api/v1/downloads/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || string(body) != "node" || res.Header.Get("Content-Disposition") == "" {
		t.Fatalf("fichier %d %q", res.StatusCode, body)
	}

	missing, err := http.Get(ts.URL + "/api/v1/downloads/brightnet.db")
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Body.Close()
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("hors catalogue %d", missing.StatusCode)
	}
}
