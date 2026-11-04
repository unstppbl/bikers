package rooms

import (
	"sync"

	"bikers.kz/backend/internal/models"
)

const subscriberQueueSize = 8

// Subscription exposes received hints without allowing callers to send them.
type Subscription struct {
	Events <-chan models.Event
	ride   string
	queue  chan models.Event
}

// EventBus delivers bounded foreground hints. Persisted state remains authoritative.
type EventBus struct {
	mu          sync.Mutex
	subscribers map[*Subscription]struct{}
}

func NewEventBus() *EventBus { return &EventBus{subscribers: make(map[*Subscription]struct{})} }

func (bus *EventBus) Subscribe(ride string) *Subscription {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	queue := make(chan models.Event, subscriberQueueSize)
	subscription := &Subscription{Events: queue, ride: ride, queue: queue}
	if bus.subscribers == nil {
		bus.subscribers = make(map[*Subscription]struct{})
	}
	bus.subscribers[subscription] = struct{}{}
	return subscription
}

func (bus *EventBus) Unsubscribe(subscription *Subscription) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	delete(bus.subscribers, subscription)
}

func (bus *EventBus) Publish(ride, kind string) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	for subscription := range bus.subscribers {
		if subscription.ride == ride {
			deliver(subscription, models.Event{Type: kind, RideID: ride})
		}
	}
}

func (bus *EventBus) PublishAll(kind string) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	for sub := range bus.subscribers {
		event := models.Event{Type: kind, RideID: sub.ride}
		select {
		case sub.queue <- event:
		default:
			// Global access/visibility changes must displace an older hint when
			// a busy room has filled its bounded queue.
			select {
			case <-sub.queue:
			default:
			}
			select {
			case sub.queue <- event:
			default:
			}
		}
	}
}

func (bus *EventBus) WakeAll() {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	for subscription := range bus.subscribers {
		deliver(subscription, models.Event{})
	}
}

func deliver(subscription *Subscription, event models.Event) {
	select {
	case subscription.queue <- event:
	default:
	}
}
