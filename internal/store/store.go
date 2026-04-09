package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
)

// Status is the Slack status derived from active calendar events.
type Status struct {
	Emoji      string    `json:"emoji"`
	Text       string    `json:"text"`
	Expiration time.Time `json:"expiration"`
}

// Event represents a Google Calendar event persisted in the store.
type Event struct {
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Cancelled bool      `json:"cancelled"`
}

// Channel represents a registered Google Calendar push notification channel.
type Channel struct {
	ID         string    `json:"id"`
	ResourceID string    `json:"resource_id"`
	CalendarID string    `json:"calendar_id"`
	UserID     string    `json:"user_id"`
	Expiry     time.Time `json:"expiry"`
}

// Store wraps a Pebble database.
type Store struct {
	db *pebble.DB
}

// New opens (or creates) a Pebble store at the given path.
func New(path string) (*Store, error) {
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("open pebble: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the store.
func (s *Store) Close() error {
	return s.db.Close()
}

// GetStatus returns the current status (O(1) lookup).
func (s *Store) GetStatus() (*Status, bool, error) {
	data, closer, err := s.db.Get(statusKey())
	if errors.Is(err, pebble.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get status: %w", err)
	}
	defer closer.Close()

	var st Status
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, false, fmt.Errorf("unmarshal status: %w", err)
	}
	return &st, true, nil
}

// SetStatus persists the current status.
func (s *Store) SetStatus(st *Status) error {
	data, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	if err := s.db.Set(statusKey(), data, pebble.Sync); err != nil {
		return fmt.Errorf("set status: %w", err)
	}
	return nil
}

// DeleteStatus removes the stored status.
func (s *Store) DeleteStatus() error {
	err := s.db.Delete(statusKey(), pebble.Sync)
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return fmt.Errorf("delete status: %w", err)
	}
	return nil
}

// GetEvent retrieves a stored calendar event.
func (s *Store) GetEvent(eventID string) (*Event, bool, error) {
	data, closer, err := s.db.Get(eventKey(eventID))
	if errors.Is(err, pebble.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get event: %w", err)
	}
	defer closer.Close()

	var ev Event
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil, false, fmt.Errorf("unmarshal event: %w", err)
	}
	return &ev, true, nil
}

// SetEvent persists a calendar event.
func (s *Store) SetEvent(ev *Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if err := s.db.Set(eventKey(ev.ID), data, pebble.Sync); err != nil {
		return fmt.Errorf("set event: %w", err)
	}
	return nil
}

// ListActiveEvents returns events that overlap with now
// (not cancelled, started at or before now, ending after now).
func (s *Store) ListActiveEvents(now time.Time) ([]*Event, error) {
	prefix := eventKeyPrefix()
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("new iter: %w", err)
	}
	defer iter.Close()

	var active []*Event
	for valid := iter.First(); valid; valid = iter.Next() {
		var ev Event
		if err := json.Unmarshal(iter.Value(), &ev); err != nil {
			continue
		}
		if !ev.Cancelled && !ev.StartTime.After(now) && ev.EndTime.After(now) {
			active = append(active, &ev)
		}
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iter events: %w", err)
	}
	return active, nil
}

// GetChannel retrieves a registered push notification channel.
func (s *Store) GetChannel(channelID string) (*Channel, bool, error) {
	data, closer, err := s.db.Get(channelKey(channelID))
	if errors.Is(err, pebble.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get channel: %w", err)
	}
	defer closer.Close()

	var ch Channel
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil, false, fmt.Errorf("unmarshal channel: %w", err)
	}
	return &ch, true, nil
}

// SetChannel persists a push notification channel registration.
func (s *Store) SetChannel(ch *Channel) error {
	data, err := json.Marshal(ch)
	if err != nil {
		return fmt.Errorf("marshal channel: %w", err)
	}
	if err := s.db.Set(channelKey(ch.ID), data, pebble.Sync); err != nil {
		return fmt.Errorf("set channel: %w", err)
	}
	return nil
}

// GetSyncToken returns the stored incremental sync token for a calendar.
func (s *Store) GetSyncToken(calendarID string) (string, bool, error) {
	data, closer, err := s.db.Get(syncTokenKey(calendarID))
	if errors.Is(err, pebble.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get sync token: %w", err)
	}
	defer closer.Close()
	return string(data), true, nil
}

// SetSyncToken persists the incremental sync token for a calendar.
func (s *Store) SetSyncToken(calendarID, token string) error {
	if err := s.db.Set(syncTokenKey(calendarID), []byte(token), pebble.Sync); err != nil {
		return fmt.Errorf("set sync token: %w", err)
	}
	return nil
}
