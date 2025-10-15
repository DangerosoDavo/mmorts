package production

import (
	"sync"
	"time"

	"github.com/gravitas-015/inventory"
)

// EventType represents the type of production event.
type EventType int

const (
	// EventJobStarted is emitted when a job begins.
	EventJobStarted EventType = iota
	// EventJobProgress is emitted periodically for progress updates.
	EventJobProgress
	// EventJobCompleted is emitted when a job finishes successfully.
	EventJobCompleted
	// EventJobFailed is emitted when a job fails.
	EventJobFailed
	// EventJobCancelled is emitted when a job is cancelled.
	EventJobCancelled
)

// String returns a human-readable representation of the event type.
func (t EventType) String() string {
	switch t {
	case EventJobStarted:
		return "JobStarted"
	case EventJobProgress:
		return "JobProgress"
	case EventJobCompleted:
		return "JobCompleted"
	case EventJobFailed:
		return "JobFailed"
	case EventJobCancelled:
		return "JobCancelled"
	default:
		return "Unknown"
	}
}

// Event represents a production event.
type Event struct {
	Type      EventType         `json:"type"`
	Job       *Job              `json:"job"`
	Timestamp time.Time         `json:"timestamp"`
	Data      map[string]any    `json:"data,omitempty"`
}

// EventBus manages event subscriptions and delivery.
type EventBus interface {
	// Subscribe registers a handler for events for a specific owner.
	Subscribe(owner inventory.OwnerID, handler func(Event))

	// Unsubscribe removes the handler for an owner.
	Unsubscribe(owner inventory.OwnerID)

	// Publish sends an event to subscribed handlers.
	Publish(event Event)
}

// SimpleEventBus is a basic in-memory event bus implementation.
type SimpleEventBus struct {
	mu        sync.RWMutex
	handlers  map[inventory.OwnerID]func(Event)
	bufferSize int
}

// NewSimpleEventBus creates a new event bus with default buffer size.
func NewSimpleEventBus() *SimpleEventBus {
	return &SimpleEventBus{
		handlers:   make(map[inventory.OwnerID]func(Event)),
		bufferSize: 100, // Default buffer size
	}
}

// NewSimpleEventBusWithBuffer creates a new event bus with specified buffer size.
func NewSimpleEventBusWithBuffer(bufferSize int) *SimpleEventBus {
	return &SimpleEventBus{
		handlers:   make(map[inventory.OwnerID]func(Event)),
		bufferSize: bufferSize,
	}
}

// Subscribe registers a handler for events for a specific owner.
func (bus *SimpleEventBus) Subscribe(owner inventory.OwnerID, handler func(Event)) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.handlers[owner] = handler
}

// Unsubscribe removes the handler for an owner.
func (bus *SimpleEventBus) Unsubscribe(owner inventory.OwnerID) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	delete(bus.handlers, owner)
}

// Publish sends an event to subscribed handlers.
// Handlers are called asynchronously in separate goroutines to prevent blocking.
func (bus *SimpleEventBus) Publish(event Event) {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	// Get handler for the job's owner
	if event.Job != nil && event.Job.Owner != "" {
		if handler, exists := bus.handlers[event.Job.Owner]; exists {
			// Call handler asynchronously to prevent blocking
			go handler(event)
		}
	}
}

// NullEventBus is an event bus that does nothing (for testing or when events not needed).
type NullEventBus struct{}

// NewNullEventBus creates a new null event bus.
func NewNullEventBus() *NullEventBus {
	return &NullEventBus{}
}

// Subscribe does nothing.
func (bus *NullEventBus) Subscribe(owner inventory.OwnerID, handler func(Event)) {}

// Unsubscribe does nothing.
func (bus *NullEventBus) Unsubscribe(owner inventory.OwnerID) {}

// Publish does nothing.
func (bus *NullEventBus) Publish(event Event) {}
