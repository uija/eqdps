package event

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrQueueFull = errors.New("event delivery queue is full")

type Delivery struct {
	Event            Event
	NotificationText string
}

type Dispatcher struct {
	mu             sync.Mutex
	matcher        *Matcher
	events         []Event
	timers         map[string]spellTimerState
	nextGeneration uint64
	gracePeriod    time.Duration
	timerUnit      time.Duration
	notifications  chan Delivery
	sounds         chan string
	onError        func(error)
	timerChanges   chan struct{}
}

type spellTimerState struct {
	graceGeneration  uint64
	expiryGeneration uint64
	grace            *time.Timer
	expiry           *time.Timer
	expiresAt        time.Time
}

var (
	beginCastingRE = regexp.MustCompile(`^You begin casting (.+)\.$`)
	interruptedRE  = regexp.MustCompile(`^Your (.+) spell is interrupted\.$`)
	spellRankRE    = regexp.MustCompile(`^(Rk\. )?[IVXLCDM]+$`)
)

const spellTimerGracePeriod = time.Second

func NewDispatcher(events []Event, queueSize int, onError func(error)) (*Dispatcher, error) {
	return newDispatcher(events, queueSize, spellTimerGracePeriod, onError)
}

func newDispatcher(events []Event, queueSize int, gracePeriod time.Duration, onError func(error)) (*Dispatcher, error) {
	matcher, err := Compile(events)
	if err != nil {
		return nil, err
	}
	if queueSize < 1 {
		queueSize = 1
	}
	return &Dispatcher{
		matcher:       matcher,
		events:        append([]Event(nil), events...),
		timers:        make(map[string]spellTimerState),
		gracePeriod:   gracePeriod,
		timerUnit:     time.Second,
		notifications: make(chan Delivery, queueSize),
		sounds:        make(chan string, queueSize),
		onError:       onError,
		timerChanges:  make(chan struct{}, 1),
	}, nil
}

func (d *Dispatcher) Replace(events []Event) error {
	matcher, err := Compile(events)
	if err != nil {
		return err
	}
	d.mu.Lock()
	hadActiveTimers := false
	for id, state := range d.timers {
		previous, previousOK := findEvent(d.events, id)
		replacement, replacementOK := findEvent(events, id)
		if previousOK && replacementOK && previous == replacement {
			continue
		}
		if state.grace != nil {
			state.grace.Stop()
		}
		if state.expiry != nil {
			hadActiveTimers = true
			state.expiry.Stop()
		}
		delete(d.timers, id)
	}
	d.matcher = matcher
	d.events = append(d.events[:0], events...)
	d.mu.Unlock()
	if hadActiveTimers {
		d.signalTimerChange()
	}
	return nil
}

func findEvent(events []Event, id string) (Event, bool) {
	for _, configured := range events {
		if configured.ID == id {
			return configured, true
		}
	}
	return Event{}, false
}

func (d *Dispatcher) ObserveLiveLine(line string) {
	d.mu.Lock()
	matches := d.matcher.Matches(line)
	d.observeSpellTimersLocked(timerMessage(line))
	d.mu.Unlock()
	for _, matched := range matches {
		d.enqueue(matched)
	}
}

func (d *Dispatcher) observeSpellTimersLocked(message string) {
	if match := beginCastingRE.FindStringSubmatch(message); match != nil {
		for _, configured := range d.events {
			if !configured.Active || configured.TriggerType != TriggerSpellTimer || !spellNameMatches(match[1], configured.SpellName) {
				continue
			}
			state := d.timers[configured.ID]
			if state.grace != nil {
				state.grace.Stop()
			}
			d.nextGeneration++
			state.graceGeneration = d.nextGeneration
			generation := state.graceGeneration
			state.grace = time.AfterFunc(d.gracePeriod, func() {
				d.finishSpellTimerGrace(configured, generation)
			})
			d.timers[configured.ID] = state
		}
		return
	}
	if match := interruptedRE.FindStringSubmatch(message); match != nil {
		for _, configured := range d.events {
			if configured.TriggerType != TriggerSpellTimer || !spellNameMatches(match[1], configured.SpellName) {
				continue
			}
			state, ok := d.timers[configured.ID]
			if ok && state.grace != nil {
				state.grace.Stop()
				state.grace = nil
				if state.expiry == nil {
					delete(d.timers, configured.ID)
				} else {
					d.timers[configured.ID] = state
				}
			}
		}
	}
}

func (d *Dispatcher) finishSpellTimerGrace(configured Event, generation uint64) {
	d.mu.Lock()
	state, ok := d.timers[configured.ID]
	if !ok || state.graceGeneration != generation || state.grace == nil {
		d.mu.Unlock()
		return
	}
	state.grace = nil
	if state.expiry != nil {
		state.expiry.Stop()
	}
	remaining := time.Duration(configured.TimerSeconds)*d.timerUnit - d.gracePeriod
	if remaining <= 0 {
		delete(d.timers, configured.ID)
		d.mu.Unlock()
		d.enqueue(configured)
		return
	}
	d.nextGeneration++
	state.expiryGeneration = d.nextGeneration
	expiryGeneration := state.expiryGeneration
	state.expiresAt = time.Now().Add(remaining)
	state.expiry = time.AfterFunc(remaining, func() {
		d.expireSpellTimer(configured, expiryGeneration)
	})
	d.timers[configured.ID] = state
	d.mu.Unlock()
	d.signalTimerChange()
}

func (d *Dispatcher) expireSpellTimer(configured Event, generation uint64) {
	d.mu.Lock()
	state, ok := d.timers[configured.ID]
	if !ok || state.expiryGeneration != generation || state.expiry == nil {
		d.mu.Unlock()
		return
	}
	state.expiry = nil
	if state.grace == nil {
		delete(d.timers, configured.ID)
	} else {
		d.timers[configured.ID] = state
	}
	d.mu.Unlock()
	d.signalTimerChange()
	d.enqueue(configured)
}

func (d *Dispatcher) ActiveTimers() []ActiveTimer {
	d.mu.Lock()
	defer d.mu.Unlock()
	active := make([]ActiveTimer, 0, len(d.timers))
	for _, configured := range d.events {
		state, ok := d.timers[configured.ID]
		if !ok || state.expiry == nil {
			continue
		}
		active = append(active, ActiveTimer{
			ID: configured.ID, Title: configured.Title, SpellName: configured.SpellName, ExpiresAt: state.expiresAt,
		})
	}
	sort.SliceStable(active, func(i, j int) bool {
		if active[i].ExpiresAt.Equal(active[j].ExpiresAt) {
			return active[i].Title < active[j].Title
		}
		return active[i].ExpiresAt.Before(active[j].ExpiresAt)
	})
	return active
}

func (d *Dispatcher) TimerChanges() <-chan struct{} {
	return d.timerChanges
}

func (d *Dispatcher) signalTimerChange() {
	select {
	case d.timerChanges <- struct{}{}:
	default:
	}
}

func (d *Dispatcher) enqueue(configured Event) {
	if configured.Notification != "" {
		select {
		case d.notifications <- Delivery{Event: configured, NotificationText: configured.NotificationText()}:
		default:
			d.report(ErrQueueFull)
		}
	}
	if configured.Sound != "" {
		select {
		case d.sounds <- configured.Sound:
		default:
			d.report(ErrQueueFull)
		}
	}
}

func timerMessage(line string) string {
	message := logPrefix.ReplaceAllString(strings.TrimRight(line, "\r\n"), "")
	message = strings.TrimPrefix(message, "--")
	return strings.TrimSuffix(message, "--")
}

func spellNameMatches(castName, selectedName string) bool {
	if castName == selectedName {
		return true
	}
	suffix, ok := strings.CutPrefix(castName, selectedName+" ")
	return ok && spellRankRE.MatchString(suffix)
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
