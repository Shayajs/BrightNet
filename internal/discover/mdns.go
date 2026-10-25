package discover

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

const service = "_brightnet._tcp"

func Announce(addr string) (func(), error) {
	ip, port, ok := announceTarget(addr)
	if !ok {
		slog.Info("BNCP local, aucune annonce")
		return func() {}, nil
	}
	name := instanceName()
	zone, err := mdns.NewMDNSService(name, service, "local.", name+".local.", port, []net.IP{ip}, []string{"role=core"})
	if err != nil {
		return nil, err
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: zone, Logger: log.New(io.Discard, "", 0)})
	if err != nil {
		return nil, err
	}
	slog.Info("Core annoncé", "name", name, "addr", net.JoinHostPort(ip.String(), strconv.Itoa(port)))
	return func() { _ = server.Shutdown() }, nil
}

func Browse(ctx context.Context) ([]Core, error) {
	entries := make(chan *mdns.ServiceEntry, 16)
	found := map[string]Core{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for entry := range entries {
			core, ok := fromEntry(entry)
			if !ok {
				continue
			}
			if _, seen := found[core.Name]; seen {
				continue
			}
			found[core.Name] = core
		}
	}()
	params := mdns.DefaultParams(service)
	params.Entries = entries
	params.Timeout = 1200 * time.Millisecond
	params.DisableIPv6 = true
	params.Logger = log.New(io.Discard, "", 0)
	err := mdns.QueryContext(ctx, params)
	close(entries)
	<-done
	cores := make([]Core, 0, len(found))
	for _, core := range found {
		cores = append(cores, core)
	}
	sort.Slice(cores, func(i, j int) bool { return cores[i].Name < cores[j].Name })
	return cores, err
}

func BrowseType(ctx context.Context, serviceName string, timeout time.Duration) ([]Core, error) {
	entries := make(chan *mdns.ServiceEntry, 16)
	found := map[string]Core{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for entry := range entries {
			core, ok := fromEntry(entry)
			if !ok {
				continue
			}
			if _, seen := found[core.Addr]; seen {
				continue
			}
			found[core.Addr] = core
		}
	}()
	params := mdns.DefaultParams(serviceName)
	params.Entries = entries
	params.Timeout = timeout
	params.DisableIPv6 = true
	params.Logger = log.New(io.Discard, "", 0)
	err := mdns.QueryContext(ctx, params)
	close(entries)
	<-done
	cores := make([]Core, 0, len(found))
	for _, core := range found {
		cores = append(cores, core)
	}
	return cores, err
}

func Local() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:1718", 400*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func announceTarget(addr string) (net.IP, int, bool) {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, 0, false
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return nil, 0, false
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() {
		return nil, 0, false
	}
	return ip, port, true
}

func fromEntry(entry *mdns.ServiceEntry) (Core, bool) {
	if entry == nil || entry.Port == 0 {
		return Core{}, false
	}
	ip := entry.AddrV4
	if ip == nil {
		ip = entry.Addr
	}
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return Core{}, false
	}
	name := entry.Name
	if cut := strings.Index(name, "._"); cut > 0 {
		name = name[:cut]
	}
	name = strings.Trim(name, ".")
	name = strings.ReplaceAll(name, "\\ ", " ")
	if name == "" {
		name = "core"
	}
	return Core{Name: name, Addr: net.JoinHostPort(ip.String(), strconv.Itoa(entry.Port))}, true
}

func instanceName() string {
	host, err := os.Hostname()
	if err != nil {
		return "brightnet"
	}
	var b strings.Builder
	for _, r := range strings.ToLower(host) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "brightnet"
	}
	return name
}
