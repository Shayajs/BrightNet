//go:build !windows

package install

type State struct {
	Ready    bool   `json:"ready"`
	Startup  bool   `json:"startup"`
	Firewall bool   `json:"firewall"`
	Error    string `json:"error,omitempty"`
}

func Inspect(string) State { return State{Ready: true, Startup: true, Firewall: true} }

func Enable(string) State { return Inspect("") }

func OpenFirewall(string) error { return nil }

func Claim() bool { return true }

func Release() {}

func Ask(string) {}
