// Package eqldbsync uploads durable parser observations independently of any
// frontend. It does not participate in logfile replay or UI representation.
package eqldbsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/uija/eqdps/legacy/internal/dropcollector"
	"github.com/uija/eqdps/legacy/internal/eqldb"
	"github.com/uija/eqdps/legacy/internal/eqldbqueue"
	"github.com/uija/eqdps/legacy/internal/skyquest"
)

const (
	batchSize    = 2000
	pollInterval = 5 * time.Second
	maxBatches   = 3
)

type API interface {
	SubmitPlaneOfSkyEvents(context.Context, string, string, string, []eqldb.PlaneOfSkyEvent) error
	SubmitKillObservations(context.Context, string, []eqldb.KillObservation) error
	SubmitDropObservations(context.Context, string, []eqldb.DropObservation) error
}

type Runner struct {
	Client  API
	Store   eqldb.Store
	Queue   *eqldbqueue.Queue
	OnError func(error)

	lastError string
	wake      chan struct{}
}

func Default(onError func(error)) (*Runner, error) {
	store, err := eqldb.DefaultStore()
	if err != nil {
		return nil, err
	}
	queue, err := eqldbqueue.Default()
	if err != nil {
		return nil, err
	}
	return &Runner{
		Client: eqldb.NewClient(), Store: store, Queue: queue, OnError: onError,
		wake: make(chan struct{}, 1),
	}, nil
}

func (r *Runner) Run(ctx context.Context) {
	r.runOnce(ctx)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
			r.runOnce(ctx)
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

// Trigger asks the background runner to retry immediately. The buffered wake
// channel deliberately coalesces repeated queue and connection notifications.
func (r *Runner) Trigger() {
	if r == nil || r.wake == nil {
		return
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *Runner) runOnce(ctx context.Context) {
	err := r.upload(ctx)
	if err == nil {
		r.lastError = ""
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	if message := err.Error(); message != r.lastError {
		r.lastError = message
		if r.OnError != nil {
			r.OnError(err)
		}
	}
}

func (r *Runner) upload(ctx context.Context) error {
	if r.Client == nil || r.Queue == nil {
		return nil
	}
	state, err := r.Store.Load()
	if err != nil {
		return err
	}
	if state.AccessToken == "" {
		return nil
	}
	acquired, _, err := r.Store.AcquireLease("event-upload", time.Now(), 10*time.Minute)
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer r.Store.ReleaseLease("event-upload")

	if err := r.uploadPlaneOfSky(ctx, state.AccessToken); err != nil {
		return r.handleAPIError(state, err)
	}
	enabled, err := dropcollector.CollectionEnabled()
	if err != nil {
		return fmt.Errorf("read kill and drop upload preference: %w", err)
	}
	if !enabled {
		return nil
	}
	if err := r.uploadKills(ctx, state.AccessToken); err != nil {
		return r.handleAPIError(state, err)
	}
	if err := r.uploadDrops(ctx, state.AccessToken); err != nil {
		return r.handleAPIError(state, err)
	}
	return nil
}

func (r *Runner) uploadPlaneOfSky(ctx context.Context, token string) error {
	for batch := 0; batch < maxBatches; batch++ {
		entries, err := r.Queue.Batch(eqldbqueue.PlaneOfSky, batchSize)
		if err != nil || len(entries) == 0 {
			return err
		}
		events := make([]eqldb.PlaneOfSkyEvent, 0, len(entries))
		eventIndexes := make(map[string]int)
		var first *skyquest.SyncEvent
		endOffset := int64(0)
		for _, entry := range entries {
			var queued skyquest.SyncEvent
			if err := json.Unmarshal(entry.Payload, &queued); err != nil {
				return fmt.Errorf("decode queued Plane of Sky event: %w", err)
			}
			if first == nil {
				copy := queued
				first = &copy
			}
			if queued.Character != first.Character || queued.Server != first.Server {
				break
			}
			event := eqldb.PlaneOfSkyEvent{
				Type: queued.Type, Rune: queued.Rune, Amount: queued.Amount, Quest: queued.Quest,
				Timestamp: formatLogTime(queued.Timestamp),
			}
			key := event.Type + "\x00" + event.Rune + "\x00" + event.Quest + "\x00" + event.Timestamp
			if index, exists := eventIndexes[key]; exists {
				events[index].Amount += event.Amount
			} else {
				eventIndexes[key] = len(events)
				events = append(events, event)
			}
			endOffset = entry.EndOffset
		}
		if len(events) == 0 {
			if endOffset > 0 {
				if err := r.Queue.Acknowledge(eqldbqueue.PlaneOfSky, endOffset); err != nil {
					return err
				}
			}
			if first != nil {
				return errors.New("could not group queued Plane of Sky events")
			}
			continue
		}
		if first == nil {
			return errors.New("queued Plane of Sky batch has no identity")
		}
		if err := r.Client.SubmitPlaneOfSkyEvents(ctx, token, first.Character, first.Server, events); err != nil {
			return err
		}
		if err := r.Queue.Acknowledge(eqldbqueue.PlaneOfSky, endOffset); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) uploadKills(ctx context.Context, token string) error {
	for batch := 0; batch < maxBatches; batch++ {
		enabled, err := dropcollector.CollectionEnabled()
		if err != nil {
			return err
		}
		if !enabled {
			return nil
		}
		entries, err := r.Queue.Batch(eqldbqueue.Kills, batchSize)
		if err != nil || len(entries) == 0 {
			return err
		}
		events := make([]eqldb.KillObservation, 0, len(entries))
		for _, entry := range entries {
			var queued dropcollector.Observation
			if err := json.Unmarshal(entry.Payload, &queued); err != nil {
				return fmt.Errorf("decode queued kill observation: %w", err)
			}
			events = append(events, eqldb.KillObservation{
				Timestamp: formatLogTime(queued.Timestamp), Zone: queued.Zone, Mob: queued.Mob,
			})
		}
		if len(events) == 0 {
			if err := r.Queue.Acknowledge(eqldbqueue.Kills, entries[len(entries)-1].EndOffset); err != nil {
				return err
			}
			continue
		}
		if err := r.Client.SubmitKillObservations(ctx, token, events); err != nil {
			return err
		}
		if err := r.Queue.Acknowledge(eqldbqueue.Kills, entries[len(entries)-1].EndOffset); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) uploadDrops(ctx context.Context, token string) error {
	for batch := 0; batch < maxBatches; batch++ {
		enabled, err := dropcollector.CollectionEnabled()
		if err != nil {
			return err
		}
		if !enabled {
			return nil
		}
		entries, err := r.Queue.Batch(eqldbqueue.Drops, batchSize)
		if err != nil || len(entries) == 0 {
			return err
		}
		events := make([]eqldb.DropObservation, 0, len(entries))
		for _, entry := range entries {
			var queued dropcollector.Observation
			if err := json.Unmarshal(entry.Payload, &queued); err != nil {
				return fmt.Errorf("decode queued drop observation: %w", err)
			}
			events = append(events, eqldb.DropObservation{
				Timestamp: formatLogTime(queued.Timestamp), Zone: queued.Zone, Mob: queued.Mob,
				Item: queued.Item, Amount: queued.Quantity,
			})
		}
		if len(events) == 0 {
			if err := r.Queue.Acknowledge(eqldbqueue.Drops, entries[len(entries)-1].EndOffset); err != nil {
				return err
			}
			continue
		}
		if err := r.Client.SubmitDropObservations(ctx, token, events); err != nil {
			return err
		}
		if err := r.Queue.Acknowledge(eqldbqueue.Drops, entries[len(entries)-1].EndOffset); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) handleAPIError(state eqldb.State, err error) error {
	var apiErr *eqldb.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized {
		state.AccessToken = ""
		state.ConnectionID = ""
		state.Scope = ""
		if saveErr := r.Store.Save(state); saveErr != nil {
			return fmt.Errorf("%w; remove revoked EQLDB token: %v", err, saveErr)
		}
	}
	return err
}

func formatLogTime(value time.Time) string {
	return value.Format("Mon Jan 02 15:04:05 2006")
}
