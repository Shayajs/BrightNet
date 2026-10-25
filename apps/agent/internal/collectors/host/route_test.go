package host

import (
	"strings"
	"testing"
)

func TestParseRouteGateways(t *testing.T) {
	table := "Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT\n" +
		"eth0 00000000 0101A8C0 0003 0 0 100 00000000 0 0 0\n" +
		"eth0 0001A8C0 00000000 0001 0 0 100 00FFFFFF 0 0 0\n" +
		"docker0 00000000 010011AC 0003 0 0 0 00000000 0 0 0\n"
	got := parseRouteGateways(strings.NewReader(table))
	if len(got) != 2 || got[0] != "192.168.1.1" || got[1] != "172.17.0.1" {
		t.Fatalf("passerelles = %v", got)
	}
}
