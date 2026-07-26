package event

import (
	"errors"
	"sync"
)

var ErrQueueFull = errors.New("event delivery queue is full")

type Delivery struct {
	Event            Event
	NotificationText string
}

type Dispatcher struct {
	mu            sync.RWMutex
	matcher       *Matcher
	notifications chan Delivery
	sounds        chan string
	onError       func(error)
}

func NewDispatcher(events []Event, queueSize int, onError func(error)) (*Dispatcher, error) {
	matcher, err := Compile(events)
	if err != nil {
		return nil, err
	}
	if queueSize < 1 {
		queueSize = 1
	}
	return &Dispatcher{
		matcher:       matcher,
		notifications: make(chan Delivery, queueSize),
		sounds:        make(chan string, queueSize),
		onError:       onError,
	}, nil
}

func (d *Dispatcher) Replace(events []Event) error {
	matcher, err := Compile(events)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.matcher = matcher
	d.mu.Unlock()
	return nil
}

func (d *Dispatcher) ObserveLiveLine(line string) {
	d.mu.RLock()
	matches := d.matcher.Matches(line)
	d.mu.RUnlock()
	for _, matched := range matches {
		if matched.Notification != "" {
			delivery := Delivery{
				Event:            matched,
				NotificationText: matched.NotificationText(),
			}
			select {
			case d.notifications <- delivery:
			default:
				d.report(ErrQueueFull)
			}
		}
		if matched.Sound != "" {
			select {
			case d.sounds <- matched.Sound:
			default:
				d.report(ErrQueueFull)
			}
		}
	}
}

func (d *Dispatcher) Notifications() <-chan Delivery {
	return d.notifications
}

func (d *Dispatcher) Sounds() <-chan string {
	return d.sounds
}

func (d *Dispatcher) report(err error) {
	if d.onError != nil {
		d.onError(err)
	}
}
