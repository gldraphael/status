package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
)

// Status is the target status derived from active calendar events.
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

// AvailabilitySnapshot stores the latest fetched availability calendar data.
type AvailabilitySnapshot struct {
	Body      string    `json:"body"`
	Timezone  string    `json:"timezone"`
	FetchedAt time.Time `json:"fetched_at"`
}

// HolidaySnapshot stores the latest fetched England bank holiday data.
type HolidaySnapshot struct {
	Body      string    `json:"body"`
	Dates     []string  `json:"dates"`
	FetchedAt time.Time `json:"fetched_at"`
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

func (s *Store) getBytes(key []byte, label string) ([]byte, bool, error) {
	data, closer, err := s.db.Get(key)
	if errors.Is(err, pebble.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get %s: %w", label, err)
	}
	defer closer.Close()

	buf := make([]byte, len(data))
	copy(buf, data)
	return buf, true, nil
}

func (s *Store) setBytes(key, data []byte, label string) error {
	if err := s.db.Set(key, data, pebble.Sync); err != nil {
		return fmt.Errorf("set %s: %w", label, err)
	}
	return nil
}

func (s *Store) deleteKey(key []byte, label string) error {
	err := s.db.Delete(key, pebble.Sync)
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return fmt.Errorf("delete %s: %w", label, err)
	}
	return nil
}

func (s *Store) getJSON(key []byte, value any, label string) (bool, error) {
	data, ok, err := s.getBytes(key, label)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return false, fmt.Errorf("unmarshal %s: %w", label, err)
	}
	return true, nil
}

func (s *Store) setJSON(key []byte, value any, label string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", label, err)
	}
	return s.setBytes(key, data, label)
}

// GetStatus returns the current status (O(1) lookup).
func (s *Store) GetStatus() (*Status, bool, error) {
	var st Status
	ok, err := s.getJSON(statusKey(), &st, "status")
	if err != nil || !ok {
		return nil, ok, err
	}
	return &st, true, nil
}

// SetStatus persists the current status.
func (s *Store) SetStatus(st *Status) error {
	return s.setJSON(statusKey(), st, "status")
}

// DeleteStatus removes the stored status.
func (s *Store) DeleteStatus() error {
	return s.deleteKey(statusKey(), "status")
}

// GetAvailabilitySnapshot returns the stored availability calendar snapshot.
func (s *Store) GetAvailabilitySnapshot() (*AvailabilitySnapshot, bool, error) {
	var snap AvailabilitySnapshot
	ok, err := s.getJSON(availabilityKey(), &snap, "availability snapshot")
	if err != nil || !ok {
		return nil, ok, err
	}
	return &snap, true, nil
}

// SetAvailabilitySnapshot persists the latest availability calendar snapshot.
func (s *Store) SetAvailabilitySnapshot(snap *AvailabilitySnapshot) error {
	return s.setJSON(availabilityKey(), snap, "availability snapshot")
}

// GetAvailabilityDirty returns the pending availability entries JSON if the calendar has changed since the last deployment.
func (s *Store) GetAvailabilityDirty() ([]byte, bool, error) {
	return s.getBytes(availabilityDirtyKey(), "availability dirty")
}

// SetAvailabilityDirty persists the pending availability entries JSON.
func (s *Store) SetAvailabilityDirty(data []byte) error {
	return s.setBytes(availabilityDirtyKey(), data, "availability dirty")
}

// ClearAvailabilityDirty removes the availability dirty flag.
func (s *Store) ClearAvailabilityDirty() error {
	return s.deleteKey(availabilityDirtyKey(), "availability dirty")
}

// GetLastDeployedAvailability returns the availability entries JSON from the last successful deployment.
func (s *Store) GetLastDeployedAvailability() ([]byte, bool, error) {
	return s.getBytes(availabilityLastDeployedKey(), "availability last deployed")
}

// SetLastDeployedAvailability persists the availability entries JSON from a successful deployment.
func (s *Store) SetLastDeployedAvailability(data []byte) error {
	return s.setBytes(availabilityLastDeployedKey(), data, "availability last deployed")
}

// GetHolidaySnapshot returns the stored bank holiday snapshot.
func (s *Store) GetHolidaySnapshot() (*HolidaySnapshot, bool, error) {
	var snap HolidaySnapshot
	ok, err := s.getJSON(availabilityHolidaysKey(), &snap, "holiday snapshot")
	if err != nil || !ok {
		return nil, ok, err
	}
	return &snap, true, nil
}

// SetHolidaySnapshot persists the latest bank holiday snapshot.
func (s *Store) SetHolidaySnapshot(snap *HolidaySnapshot) error {
	return s.setJSON(availabilityHolidaysKey(), snap, "holiday snapshot")
}

// GetEvent retrieves a stored calendar event.
func (s *Store) GetEvent(eventID string) (*Event, bool, error) {
	var ev Event
	ok, err := s.getJSON(eventKey(eventID), &ev, "event")
	if err != nil || !ok {
		return nil, ok, err
	}
	return &ev, true, nil
}

// SetEvent persists a calendar event.
func (s *Store) SetEvent(ev *Event) error {
	return s.setJSON(eventKey(ev.ID), ev, "event")
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
