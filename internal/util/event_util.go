package util

import (
	"fmt"
	"sync"
)

type Event struct {
	ID     string
	Status string
}

func (e Event) Format() string {
	var s string

	if e.ID != "" {
		s += fmt.Sprintf("id: %s\n", e.ID)
	}

	if e.Status != "" {
		s += fmt.Sprintf("status: %s\n", e.Status)
	}

	s += "\n"

	return s
}

type Broker struct {
	clients map[string]map[chan Event]struct{}
	mu      sync.RWMutex
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[string]map[chan Event]struct{}),
	}
}

func (b *Broker) Subscribe(ingestionID string) chan Event {
	ch := make(chan Event, 10)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.clients[ingestionID] == nil {
		b.clients[ingestionID] = make(map[chan Event]struct{})
	}
	b.clients[ingestionID][ch] = struct{}{}
	return ch
}

func (b *Broker) Unsubscribe(ingestionID string, ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subscribers, ok := b.clients[ingestionID]

	if !ok {
		return
	}

	if _, ok := subscribers[ch]; !ok {
		return
	}

	delete(subscribers, ch)
	close(ch)

	if len(subscribers) == 0 {
		delete(b.clients, ingestionID)
	}
}

func (b *Broker) Publish(ingestionID string, event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subscribers, ok := b.clients[ingestionID]

	if !ok {
		return
	}

	for ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}

}
