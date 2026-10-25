package desk

import (
	"embed"
	"net/http"
)

//go:embed face.css frame.js
var assets embed.FS

func Asset(w http.ResponseWriter, name string) {
	body, err := assets.ReadFile(name)
	if err != nil {
		http.Error(w, "page indisponible", http.StatusNotFound)
		return
	}
	switch name {
	case "face.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	}
	_, _ = w.Write(body)
}
