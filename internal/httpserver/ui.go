package httpserver

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func WithUI(api http.Handler, root string) http.Handler {
	root = strings.TrimSpace(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if root == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("<!doctype html><meta charset=utf-8><title>BrightNet</title><p>Interface non construite. Le serveur API répond sur ce port.</p>"))
			return
		}
		rel := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		target := filepath.Join(root, rel)
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			http.ServeFile(w, r, target)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
	})
}
