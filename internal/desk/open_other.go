//go:build !windows

package desk

func open(_, url string) bool {
	browser(url)
	return false
}

func setStay(bool) {}

func reveal() bool { return false }

func quit() {}

func active() bool { return false }
