package host

import (
	"bufio"
	"net"
	"os"
	"runtime"
	"strings"

	"brightnet/internal/contract"
)

type Options struct {
	MachineIDPath string
	HostnamePath  string
	InContainer   bool
}

func Collect(opt Options) (contract.HostSnapshot, contract.CollectorRun) {
	defer func() {
		if rec := recover(); rec != nil {
			// Le collecteur isole sa panne. L'appelant reçoit l'erreur via le retour nommé si on l'avait.
		}
	}()
	snap := contract.HostSnapshot{OS: runtime.GOOS, Arch: runtime.GOARCH, ObservationScope: "host"}
	if opt.InContainer {
		snap.ObservationScope = "agent-netns"
	}
	snap.Hostname = readLine(opt.HostnamePath)
	if snap.Hostname == "" {
		snap.Hostname, _ = os.Hostname()
	}
	snap.MachineID = strings.TrimSpace(readFile(opt.MachineIDPath))
	snap.Kernel = strings.TrimSpace(readFile("/proc/sys/kernel/osrelease"))
	id, version := osRelease()
	snap.OS = fallback(id, snap.OS)
	snap.OSVersion = version
	if strings.Contains(strings.ToLower(snap.Kernel), "microsoft") {
		snap.Virtualization = "wsl2"
	} else {
		snap.Virtualization = "none"
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return snap, contract.CollectorRun{Collector: "host", Status: "error", Error: err.Error()}
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		item := contract.InterfaceSnapshot{
			Name: iface.Name, MAC: iface.HardwareAddr.String(), MTU: iface.MTU, Flags: iface.Flags.String(),
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			item.Addrs = append(item.Addrs, addr.String())
		}
		snap.Interfaces = append(snap.Interfaces, item)
	}
	return snap, contract.CollectorRun{Collector: "host", Status: "ok"}
}

func osRelease() (string, string) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "", ""
	}
	defer file.Close()
	var id, version string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		switch key {
		case "ID":
			id = value
		case "VERSION_ID":
			version = value
		}
	}
	return id, version
}

func readFile(path string) string {
	if path == "" {
		return ""
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(body)
}

func readLine(path string) string {
	return strings.TrimSpace(readFile(path))
}

func fallback(v, alt string) string {
	if v == "" {
		return alt
	}
	return v
}
