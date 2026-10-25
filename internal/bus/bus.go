package bus

import (
	"encoding/json"
	"sync"
)

const (
	streamCap = 400
	bufferCap = 32
)

type Event struct {
	ID   uint64
	Body []byte
}

type Bus struct {
	mu   sync.Mutex
	next uint64
	log  []Event
	subs map[*subscriber]struct{}
}

type subscriber struct {
	ch chan Event
}

func New() *Bus {
	return &Bus{subs: map[*subscriber]struct{}{}}
}

func (b *Bus) Publish(v any) Event {
	body, err := json.Marshal(v)
	if err != nil {
		return Event{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	ev := Event{ID: b.next, Body: body}
	b.log = append(b.log, ev)
	if len(b.log) > streamCap {
		b.log = append([]Event(nil), b.log[len(b.log)-streamCap:]...)
	}
	for sub := range b.subs {
		sub.push(ev)
	}
	return ev
}

func (b *Bus) Subscribe(since uint64) (past []Event, live <-chan Event, cancel func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ev := range b.log {
		if ev.ID > since {
			past = append(past, ev)
		}
	}
	sub := &subscriber{ch: make(chan Event, bufferCap)}
	b.subs[sub] = struct{}{}
	live = sub.ch
	cancel = func() {
		b.mu.Lock()
		delete(b.subs, sub)
		b.mu.Unlock()
	}
	return past, live, cancel
}

func (s *subscriber) push(ev Event) {
	select {
	case s.ch <- ev:
		return
	default:
	}
	queued := make([]Event, 0, bufferCap+1)
	for {
		select {
		case old := <-s.ch:
			queued = append(queued, old)
		default:
			queued = append(queued, ev)
			for _, item := range compact(queued) {
				select {
				case s.ch <- item:
				default:
					return
				}
			}
			return
		}
	}
}

func compact(events []Event) []Event {
	if len(events) <= bufferCap {
		return events
	}
	lastGraph := -1
	for i, ev := range events {
		if eventType(ev.Body) == "graph.changed" {
			lastGraph = i
		}
	}
	keep := map[int]bool{}
	if lastGraph >= 0 {
		keep[lastGraph] = true
	}
	for i := len(events) - 1; i >= 0 && len(keep) < bufferCap; i-- {
		keep[i] = true
	}
	out := make([]Event, 0, len(keep))
	for i, ev := range events {
		if keep[i] {
			out = append(out, ev)
		}
	}
	return out
}

func eventType(body []byte) string {
	var head struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(body, &head)
	return head.Type
}
