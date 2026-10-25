package install

import (
	"os"
	"path/filepath"
)

func Remember(path, url string) {
	if path == "" || url == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(url), 0o600)
}
