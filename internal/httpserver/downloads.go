package httpserver

import (
	"net/http"
	"os"
	"path/filepath"
)

type Download struct {
	ID        string `json:"id"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Title     string `json:"title"`
	File      string `json:"file"`
	Note      string `json:"note"`
	Available bool   `json:"available"`
}

func nodeDownloads() []Download {
	return []Download{
		{ID: "bnnp-windows", OS: "windows", Arch: "amd64", Title: "BNNP", File: "brightnet-bnnp-windows-amd64.exe", Note: "BrightNet Node Parameter. Choisit ce que cette machine envoie."},
		{ID: "bnca-windows", OS: "windows", Arch: "amd64", Title: "BNCA", File: "brightnet-bnca-windows-amd64.exe", Note: "BrightNet Core Admin. Gère le Core, sans collecter cette machine."},
		{ID: "linux-amd64", OS: "linux", Arch: "amd64", Title: "Linux", File: "brightnet-node-linux-amd64", Note: "Même programme, pour un PC Linux 64 bits."},
		{ID: "linux-arm64", OS: "linux", Arch: "arm64", Title: "Linux", File: "brightnet-node-linux-arm64", Note: "Même programme, pour une machine ARM 64 bits."},
		{ID: "android", OS: "android", Arch: "universal", Title: "Android", File: "brightnet-android.apk", Note: "BrightNet sur le téléphone. Même carte, plein écran."},
	}
}

func (s *Server) downloads(w http.ResponseWriter, _ *http.Request) {
	items := nodeDownloads()
	for i := range items {
		items[i].Available = s.downloadPath(items[i].File) != ""
	}
	writeJSON(w, http.StatusOK, map[string]any{"downloads": items})
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	for _, item := range nodeDownloads() {
		if item.File != name {
			continue
		}
		path := s.downloadPath(name)
		if path == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+item.File+"\"")
		http.ServeFile(w, r, path)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) downloadPath(name string) string {
	if s.Downloads == "" || filepath.Base(name) != name {
		return ""
	}
	path := filepath.Join(s.Downloads, name)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return ""
	}
	return path
}
