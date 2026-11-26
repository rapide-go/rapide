package broadcast

import (
	"encoding/json"
	"sync"
)

// Message is a typed event broadcasted over a channel.
type Message struct {
	Event string
	Data  []byte
}

// Hub is an in-process pub/sub broadcaster powered by Go channels (no Pusher/Reverb needed).
type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Message]struct{}
}

// NewHub creates a new broadcasting Hub.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[chan Message]struct{}),
	}
}

// Default is the global application broadcast Hub.
var Default = NewHub()

// Subscribe joins a channel and returns a receive channel plus an unsubscribe function.
func (h *Hub) Subscribe(channel string) (<-chan Message, func()) {
	ch := make(chan Message, 32)
	h.mu.Lock()
	if h.subscribers[channel] == nil {
		h.subscribers[channel] = make(map[chan Message]struct{})
	}
	h.subscribers[channel][ch] = struct{}{}
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		if subs, ok := h.subscribers[channel]; ok {
			if _, exists := subs[ch]; exists {
				delete(subs, ch)
				close(ch)
			}
			if len(subs) == 0 {
				delete(h.subscribers, channel)
			}
		}
		h.mu.Unlock()
	}
	return ch, unsub
}

// PublishJSON encodes payload to JSON and fans it out non-blockingly to all subscribers on channel.
func (h *Hub) PublishJSON(channel, event string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := Message{Event: event, Data: raw}

	h.mu.RLock()
	subs := h.subscribers[channel]
	for ch := range subs {
		select {
		case ch <- msg:
		default:
			// drop for slow subscriber rather than blocking publishers
		}
	}
	h.mu.RUnlock()
	return nil
}

// Publish broadcasts a JSON payload on the Default global Hub.
func Publish(channel, event string, payload any) error {
	return Default.PublishJSON(channel, event, payload)
}
