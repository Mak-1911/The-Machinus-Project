// Package pubsub provides publish-subscribe messaging for the UI.
package pubsub

import (
	"sync"
)

// Message represents a pubsub message.
type Message struct {
	Topic string
	Data  any
}

// Bus represents a pubsub bus.
type Bus struct {
	mu      sync.RWMutex
	subs    map[string][]chan Message
	closed  bool
}

// NewBus creates a new pubsub bus.
func NewBus() *Bus {
	return &Bus{
		subs: make(map[string][]chan Message),
	}
}

// Subscribe subscribes to a topic.
func (b *Bus) Subscribe(topic string) chan Message {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Message, 10)
	b.subs[topic] = append(b.subs[topic], ch)
	return ch
}

// Publish publishes a message to a topic.
func (b *Bus) Publish(topic string, data any) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	msg := Message{Topic: topic, Data: data}
	for _, ch := range b.subs[topic] {
		select {
		case ch <- msg:
		default:
			// Channel is full, skip
		}
	}
}

// Unsubscribe unsubscribes a channel from a topic.
func (b *Bus) Unsubscribe(topic string, ch chan Message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subs[topic]
	for i, sub := range subs {
		if sub == ch {
			b.subs[topic] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
}

// Close closes the bus and all channels.
func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	for _, subs := range b.subs {
		for _, ch := range subs {
			close(ch)
		}
	}
	b.subs = make(map[string][]chan Message)
}

// Event represents a pubsub event.
type Event[T any] struct {
	Type string
	Data T
}

// Payload returns the event data.
func (e Event[T]) Payload() T {
	return e.Data
}

// DeletedEvent represents a deleted event.
const DeletedEvent = "deleted"

// CreatedEvent represents a created event.
const CreatedEvent = "created"

// UpdatedEvent represents an updated event.
const UpdatedEvent = "updated"
