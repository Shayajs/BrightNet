package bncp

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"brightnet/internal/bus"
	"brightnet/internal/contract"
	"brightnet/internal/store"
)

func TestEnrollAndSharedHost(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "brightnet.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.UpsertDevPairingCode(t.Context(), "pair-test", time.Hour); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _ = NewServer(st, bus.New()).Serve(ctx, ln) }()

	client := &Client{Addr: ln.Addr().String()}
	t.Cleanup(func() { _ = client.Close() })
	var first contract.AgentConfig
	if err := client.RoundTrip(ctx, TypeEnroll, contract.EnrollRequest{PairingCode: "pair-test", Hostname: "poste", MachineID: "same-machine", Version: "test"}, &first); err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
	var second contract.AgentConfig
	if err := client.RoundTrip(ctx, TypeEnroll, contract.EnrollRequest{PairingCode: "pair-test", Hostname: "poste", MachineID: "same-machine", Version: "test"}, &second); err != nil {
		t.Fatal(err)
	}
	if first.AgentID == "" || first.AgentID == second.AgentID || first.Token == "" {
		t.Fatalf("appairages %+v %+v", first, second)
	}
	send := func(id, token, iface string) {
		t.Helper()
		msg := map[string]any{
			"agent_id": id,
			"token":    token,
			"snapshot": contract.Snapshot{
				Version: contract.Version, CollectedAt: time.Now().UTC(),
				Host: contract.HostSnapshot{
					Hostname: "poste", OS: "linux", MachineID: "same-machine", ObservationScope: "host",
					Interfaces: []contract.InterfaceSnapshot{{Name: iface, Flags: "up", Addrs: []string{"10.0.0.8/24"}}},
				},
				CollectorRuns: []contract.CollectorRun{{Collector: "host", Status: "ok"}},
			},
		}
		var ack contract.AgentConfig
		if err := client.RoundTrip(ctx, TypeSnapshot, msg, &ack); err != nil {
			t.Fatal(err)
		}
		if ack.SnapshotIntervalSeconds != 20 || len(ack.Collectors) != 2 {
			t.Fatalf("ack %+v", ack)
		}
	}
	send(first.AgentID, first.Token, "eth0")
	_ = client.Close()
	send(second.AgentID, second.Token, "wlan0")

	graph, err := st.Graph(ctx, "physical", "")
	if err != nil {
		t.Fatal(err)
	}
	hosts := 0
	ifaces := 0
	for _, node := range graph.Nodes {
		if node.Kind == "host" && node.Status != "absent" {
			hosts++
		}
		if node.Kind == "network_interface" && node.Status != "absent" {
			ifaces++
		}
	}
	if hosts != 1 || ifaces != 2 {
		t.Fatalf("hosts=%d interfaces=%d", hosts, ifaces)
	}
}

func TestDirectReachesTheOtherNode(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "brightnet.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.UpsertDevPairingCode(t.Context(), "pair-test", time.Hour); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { _ = NewServer(st, bus.New()).Serve(ctx, ln) }()

	got := make(chan Direct, 1)
	left := &Client{Addr: ln.Addr().String()}
	right := &Client{Addr: ln.Addr().String(), OnDirect: func(msg Direct) { got <- msg }}
	t.Cleanup(func() { _ = left.Close(); _ = right.Close() })
	var a, b contract.AgentConfig
	if err := left.RoundTrip(ctx, TypeEnroll, contract.EnrollRequest{PairingCode: "pair-test", Hostname: "a", MachineID: "machine-a", Version: "test"}, &a); err != nil {
		t.Fatal(err)
	}
	if err := right.RoundTrip(ctx, TypeEnroll, contract.EnrollRequest{PairingCode: "pair-test", Hostname: "b", MachineID: "machine-b", Version: "test"}, &b); err != nil {
		t.Fatal(err)
	}
	if err := left.RoundTrip(ctx, TypeHeartbeat, map[string]string{"agent_id": a.AgentID, "token": a.Token}, &contract.AgentConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := right.RoundTrip(ctx, TypeHeartbeat, map[string]string{"agent_id": b.AgentID, "token": b.Token}, &contract.AgentConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := left.RoundTrip(ctx, TypeDirect, map[string]string{"agent_id": a.AgentID, "token": a.Token, "to": b.AgentID, "payload": "présent"}, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-got:
		if msg.From != a.AgentID || msg.Payload != "présent" {
			t.Fatalf("message = %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dBNCP non reçu")
	}
}
