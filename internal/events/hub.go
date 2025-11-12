package events

import "sync"

// Hub keeps per-job websocket subscribers and fan-outs updates from Kafka.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[chan []byte]struct{}),
	}
}

func (h *Hub) Subscribe(jobID string) (<-chan []byte, func()) {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	if _, ok := h.subscribers[jobID]; !ok {
		h.subscribers[jobID] = make(map[chan []byte]struct{})
	}
	h.subscribers[jobID][ch] = struct{}{}
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		if subs, ok := h.subscribers[jobID]; ok {
			if _, exists := subs[ch]; exists {
				delete(subs, ch)
				if len(subs) == 0 {
					delete(h.subscribers, jobID)
				}
			}
		}
		h.mu.Unlock()
	}

	return ch, cancel
}

func (h *Hub) Publish(jobID string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	subs, ok := h.subscribers[jobID]
	if !ok {
		return
	}
	for ch := range subs {
		select {
		case ch <- payload:
		default:
		}
	}
}
