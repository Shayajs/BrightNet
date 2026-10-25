package lan

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"brightnet/apps/agent/internal/collectors/host"
	"brightnet/internal/contract"
	"brightnet/internal/discover"
)

var deviceServices = []string{
	"_workstation._tcp", "_device-info._tcp", "_googlecast._tcp", "_airplay._tcp",
	"_ipp._tcp", "_printer._tcp", "_smb._tcp", "_http._tcp", "_ssh._tcp",
}

type Options struct {
	ARPPath    string
	RoutePath  string
	DHCPScopes string
}

func Collect(ctx context.Context, opt Options) ([]contract.DeviceSnapshot, contract.CollectorRun, string) {
	if opt.ARPPath == "" {
		opt.ARPPath = "/proc/net/arp"
	}
	own := ownIPs()
	gateways := map[string]bool{}
	for _, ip := range host.DefaultGateways() {
		gateways[ip] = true
	}
	devices := neighbors(opt.ARPPath, own, gateways)
	for _, found := range browse(ctx) {
		ip, _, _ := net.SplitHostPort(found.Addr)
		if ip == "" || own[ip] || gateways[ip] {
			continue
		}
		devices = append(devices, contract.DeviceSnapshot{Name: found.Name, IP: ip, Source: "mdns"})
	}
	note := dhcpNote(opt.DHCPScopes)
	return mergeDevices(devices), contract.CollectorRun{Collector: "lan", Status: "ok"}, note
}

func neighbors(path string, own, gateways map[string]bool) []contract.DeviceSnapshot {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	var out []contract.DeviceSnapshot
	scanner := bufio.NewScanner(file)
	header := true
	for scanner.Scan() {
		if header {
			header = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 6 || fields[2] == "0x0" {
			continue
		}
		ip, mac, iface := fields[0], strings.ToLower(fields[3]), fields[5]
		if virtualIface(iface) || mac == "00:00:00:00:00:00" || own[ip] || gateways[ip] {
			continue
		}
		if net.ParseIP(ip) == nil {
			continue
		}
		out = append(out, contract.DeviceSnapshot{Name: ip, IP: ip, MAC: mac, Source: "neighbor"})
	}
	return out
}

func browse(ctx context.Context) []discover.Core {
	ctx, cancel := context.WithTimeout(ctx, 900*time.Millisecond)
	defer cancel()
	found := make(chan []discover.Core, len(deviceServices))
	for _, service := range deviceServices {
		service := service
		go func() {
			items, _ := discover.BrowseType(ctx, service, 700*time.Millisecond)
			found <- items
		}()
	}
	var all []discover.Core
	for range deviceServices {
		all = append(all, <-found...)
	}
	return all
}

func mergeDevices(in []contract.DeviceSnapshot) []contract.DeviceSnapshot {
	byMAC := map[string]int{}
	byIP := map[string]int{}
	byName := map[string]int{}
	var out []contract.DeviceSnapshot
	for _, item := range in {
		if item.IP == "" && item.MAC == "" {
			continue
		}
		item.Name = cleanName(item.Name)
		if i, ok := byMAC[item.MAC]; ok && item.MAC != "" {
			out[i] = richer(out[i], item)
			continue
		}
		if i, ok := byIP[item.IP]; ok && item.IP != "" {
			out[i] = richer(out[i], item)
			indexDevice(byMAC, byIP, byName, out[i], i)
			continue
		}
		if i, ok := byName[item.Name]; ok && item.Name != "" && item.Name != item.IP {
			out[i] = richer(out[i], item)
			indexDevice(byMAC, byIP, byName, out[i], i)
			continue
		}
		out = append(out, item)
		indexDevice(byMAC, byIP, byName, item, len(out)-1)
	}
	return out
}

func indexDevice(byMAC, byIP, byName map[string]int, item contract.DeviceSnapshot, i int) {
	if item.MAC != "" {
		byMAC[item.MAC] = i
	}
	if item.IP != "" {
		byIP[item.IP] = i
	}
	if item.Name != "" && item.Name != item.IP {
		byName[item.Name] = i
	}
}

func cleanName(name string) string {
	name = strings.ReplaceAll(name, "\\ ", " ")
	name = strings.Trim(name, ".")
	return name
}

func richer(base, extra contract.DeviceSnapshot) contract.DeviceSnapshot {
	if extra.MAC != "" {
		base.MAC = extra.MAC
	}
	if extra.IP != "" {
		base.IP = extra.IP
	}
	if extra.Source == "mdns" && extra.Name != "" && extra.Name != extra.IP {
		base.Name = extra.Name
		if base.Source == "neighbor" {
			base.Source = "neighbor+mdns"
		} else {
			base.Source = extra.Source
		}
		base.Detail = extra.Detail
	}
	return base
}

func dhcpNote(dir string) string {
	if dir == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	scopes := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".scope") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		body := make([]byte, 2)
		file, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		_, _ = file.Read(body)
		_ = file.Close()
		if string(body) != "SC" {
			continue
		}
		scopes++
		if info.Size() > 512 {
			return "baux présents"
		}
	}
	if scopes == 0 {
		return ""
	}
	return "aucun bail"
}

func virtualIface(name string) bool {
	switch {
	case name == "lo", name == "docker0":
		return true
	case strings.HasPrefix(name, "br-"), strings.HasPrefix(name, "veth"), strings.HasPrefix(name, "virbr"):
		return true
	default:
		return false
	}
}

func ownIPs() map[string]bool {
	out := map[string]bool{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err == nil && ip != nil {
				out[ip.String()] = true
			}
		}
	}
	return out
}
