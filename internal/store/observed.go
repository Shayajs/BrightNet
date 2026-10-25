package store

import (
	"database/sql"
	"net"
	"strings"
	"time"

	"brightnet/internal/contract"
)

func applyNetworks(tx *sql.Tx, agentID, hostID string, snap contract.Snapshot, seen map[string]bool, quiet bool, at time.Time) (map[string]string, string, bool, []contract.Activity, error) {
	ids := map[string]string{}
	var activities []contract.Activity
	changed := false
	var primary string
	for _, iface := range snap.Host.Interfaces {
		if iface.Name == "" || virtualIface(iface.Name) || strings.Contains(iface.Flags, "loopback") {
			continue
		}
		for _, raw := range iface.Addrs {
			cidr, ipnet, ok := prefixOf(raw)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			if _, exists := ids[cidr]; exists {
				continue
			}
			key := "lan:" + scopeOf(agentID, snap.Host.MachineID) + ":" + cidr
			props := map[string]any{"cidr": cidr, "interface": iface.Name}
			if snap.Host.DHCP != "" {
				props["dhcp"] = snap.Host.DHCP
			}
			id, ch, ev, err := upsertEntity(tx, entityWrite{
				Kind: "network", Name: cidr, Key: key, Status: "online",
				Simulated: snap.Simulated, AgentID: agentID, At: at, Properties: props,
			}, quiet)
			if err != nil {
				return nil, "", false, nil, err
			}
			changed = changed || ch
			activities = append(activities, ev...)
			seen[key] = true
			ids[cidr] = id
			if primary == "" {
				primary = id
			}
			if containsGateway(ipnet, snap.Host.Gateways) {
				primary = id
			}
		}
	}
	if primary != "" {
		if _, err := tx.Exec(`UPDATE entities SET parent_id=? WHERE id=?`, primary, hostID); err != nil {
			return nil, "", false, nil, err
		}
	}
	return ids, primary, changed, activities, nil
}

func networkFor(ids map[string]string, ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	for cidr, id := range ids {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err == nil && ipnet.Contains(parsed) {
			return id
		}
	}
	return ""
}

func prefixOf(raw string) (string, *net.IPNet, bool) {
	if !strings.Contains(raw, "/") {
		return "", nil, false
	}
	ip, ipnet, err := net.ParseCIDR(raw)
	if err != nil || ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return "", nil, false
	}
	return ipnet.String(), ipnet, true
}

func containsGateway(ipnet *net.IPNet, gateways []string) bool {
	for _, gateway := range gateways {
		ip := net.ParseIP(gateway)
		if ip != nil && ipnet.Contains(ip) {
			return true
		}
	}
	return false
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

func scopeOf(agentID, machineID string) string {
	if machineID != "" {
		return machineID
	}
	return agentID
}

func deviceKey(agentID, machineID string, dev contract.DeviceSnapshot) string {
	scope := scopeOf(agentID, machineID)
	if dev.MAC != "" {
		return "device:" + scope + ":" + strings.ToLower(dev.MAC)
	}
	return "device:" + scope + ":ip:" + dev.IP
}
