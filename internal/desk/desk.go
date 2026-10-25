package desk

import (
	"os/exec"
	"runtime"
)

func Open(title, url string) bool {
	return open(title, url)
}

func Stay(on bool) { setStay(on) }

func Reveal() bool { return reveal() }

func Quit() { quit() }

func Active() bool { return active() }

func browser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
