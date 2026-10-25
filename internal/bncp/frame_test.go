package bncp

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"brightnet/internal/contract"
)

func TestRoundTrip(t *testing.T) {
	body, err := Marshal(contract.EnrollRequest{
		PairingCode: "brightnet-dev-pair", Hostname: "poste", MachineID: "abc", Version: "0.1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, Frame{Type: TypeEnroll, Body: body}); err != nil {
		t.Fatal(err)
	}
	frame, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Type != TypeEnroll || frame.Version != Version {
		t.Fatalf("frame %+v", frame)
	}
	var got contract.EnrollRequest
	if err := Unmarshal(frame.Body, &got); err != nil {
		t.Fatal(err)
	}
	if got.MachineID != "abc" || got.Hostname != "poste" {
		t.Fatalf("corps %+v", got)
	}
}

func TestSnapshotKeepsTime(t *testing.T) {
	at := time.Date(2026, 10, 25, 16, 0, 0, 0, time.UTC)
	body, err := Marshal(contract.Snapshot{Version: contract.Version, CollectedAt: at, Host: contract.HostSnapshot{Hostname: "h"}})
	if err != nil {
		t.Fatal(err)
	}
	var got contract.Snapshot
	if err := Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if !got.CollectedAt.Equal(at) || got.Host.Hostname != "h" {
		t.Fatalf("instantané %+v", got)
	}
}

func TestRejects(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
		want error
	}{
		{"short", []byte("BN"), ErrShort},
		{"magic", append([]byte("XXXX\x01\x01\x00\x00\x00\x00\x00\x00"), []byte{}...), ErrMagic},
		{"version", []byte("BNCP\x02\x01\x00\x00\x00\x00\x00\x00"), ErrVersion},
		{"flags", []byte("BNCP\x01\x01\x00\x01\x00\x00\x00\x00"), ErrFlags},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Read(bytes.NewReader(tc.raw))
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want %v", err, tc.want)
			}
		})
	}
}

func TestTooLarge(t *testing.T) {
	var head [headerSize]byte
	copy(head[:4], magic)
	head[4] = Version
	head[5] = TypeHeartbeat
	head[11] = 1
	head[10] = 0
	// length = MaxBody+1
	n := uint32(MaxBody + 1)
	head[8] = byte(n >> 24)
	head[9] = byte(n >> 16)
	head[10] = byte(n >> 8)
	head[11] = byte(n)
	_, err := Read(bytes.NewReader(head[:]))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err=%v", err)
	}
}
