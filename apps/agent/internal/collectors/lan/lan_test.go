package lan

import (
	"os"
	"strings"
	"testing"

	"brightnet/internal/contract"
)

func TestNeighborsSkipGatewayAndBridges(t *testing.T) {
	table := "IP address HW type Flags HW address Mask Device\n" +
		"192.0.2.2 0x1 0x2 fc:9d:05:34:26:71 * eth0\n" +
		"192.0.2.1 0x1 0x2 74:24:9f:95:44:03 * eth0\n" +
		"172.20.0.2 0x1 0x2 16:8e:fb:5b:9e:27 * br-fa223def886f\n" +
		"192.0.2.9 0x1 0x0 00:00:00:00:00:00 * eth0\n"
	path := t.TempDir() + "/arp"
	if err := os.WriteFile(path, []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	got := neighbors(path, map[string]bool{}, map[string]bool{"192.0.2.1": true})
	if len(got) != 1 || got[0].IP != "192.0.2.2" || got[0].Source != "neighbor" {
		t.Fatalf("voisins = %+v", got)
	}
}

func TestMergePrefersMDNSName(t *testing.T) {
	got := mergeDevices([]contract.DeviceSnapshot{
		{Name: "192.0.2.2", IP: "192.0.2.2", MAC: "aa:bb:cc:dd:ee:ff", Source: "neighbor"},
		{Name: "salon", IP: "192.0.2.2", Source: "mdns"},
	})
	if len(got) != 1 || got[0].Name != "salon" || got[0].MAC == "" || !strings.Contains(got[0].Source, "mdns") {
		t.Fatalf("fusion = %+v", got)
	}
}
