package bus

import (
	"strings"
	"testing"
)

func TestGraphChangeSurvivesABurst(t *testing.T) {
	b := New()
	_, live, cancel := b.Subscribe(0)
	defer cancel()
	for i := 0; i < 80; i++ {
		b.Publish(map[string]string{"type": "activity", "n": "x"})
	}
	b.Publish(map[string]string{"type": "graph.changed"})
	found := false
	for {
		select {
		case ev := <-live:
			if strings.Contains(string(ev.Body), `"graph.changed"`) {
				found = true
			}
		default:
			if !found {
				t.Fatal("le changement de graphe a été perdu")
			}
			return
		}
	}
}

func TestReplaySkipsAlreadySeen(t *testing.T) {
	b := New()
	first := b.Publish(map[string]string{"type": "activity"})
	b.Publish(map[string]string{"type": "graph.changed"})
	past, _, cancel := b.Subscribe(first.ID)
	defer cancel()
	if len(past) != 1 || !strings.Contains(string(past[0].Body), "graph.changed") {
		t.Fatalf("reprise = %d", len(past))
	}
}
