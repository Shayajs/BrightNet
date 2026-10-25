package discover

import (
	"bytes"
	"strings"
	"testing"
)

func TestChooseSingle(t *testing.T) {
	var out bytes.Buffer
	got, err := Choose([]Core{{Name: "raspberrypi", Addr: "192.0.2.3:1718"}}, strings.NewReader(""), &out, false)
	if err != nil || got.Addr != "192.0.2.3:1718" {
		t.Fatalf("choix = %v %v", got, err)
	}
	if !strings.Contains(out.String(), "raspberrypi") {
		t.Fatal(out.String())
	}
}

func TestChooseManyInteractive(t *testing.T) {
	var out bytes.Buffer
	cores := []Core{{Name: "salon", Addr: "192.0.2.2:1718"}, {Name: "bureau", Addr: "192.0.2.9:1718"}}
	got, err := Choose(cores, strings.NewReader("2\n"), &out, true)
	if err != nil || got.Name != "bureau" {
		t.Fatalf("choix = %v %v", got, err)
	}
}

func TestChooseManyWithoutTerminal(t *testing.T) {
	var out bytes.Buffer
	_, err := Choose([]Core{{Name: "a", Addr: "192.0.2.2:1718"}, {Name: "b", Addr: "192.0.2.3:1718"}}, strings.NewReader(""), &out, false)
	if err != ErrChoice {
		t.Fatal(err)
	}
}

func TestChooseNone(t *testing.T) {
	_, err := Choose(nil, strings.NewReader(""), ioDiscard{}, false)
	if err != ErrNone {
		t.Fatal(err)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
