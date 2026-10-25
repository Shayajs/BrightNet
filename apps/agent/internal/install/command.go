package install

import "strings"

func StartupValue(exe string) string {
	return `"` + strings.ReplaceAll(exe, `"`, "") + `" --background`
}

func SameExecutable(value, exe string) bool {
	if exe == "" || value == "" {
		return false
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(exe))
}
