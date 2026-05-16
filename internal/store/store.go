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

// CalendarSnapshot stores the latest fetched raw calendar feed.
type CalendarSnapshot struct {
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

func setBatchJSON(batch *pebble.Batch, key []byte, value any, label string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", label, err)
	}
	if err := batch.Set(key, data, nil); err != nil {
		return fmt.Errorf("set %s: %w", label, err)
	}
	return nil
}

func deleteBatchKey(batch *pebble.Batch, key []byte, label string) error {
	err := batch.Delete(key, nil)
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return fmt.Errorf("delete %s: %w", label, err)
	}
	return nil
}

func (s *Store) commitBatch(label string, fn func(*pebble.Batch) error) error {
	batch := s.db.NewBatch()
	defer batch.Close()

	if err := fn(batch); err != nil {
		return err
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		return fmt.Errorf("commit %s: %w", label, err)
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
	ok, err := s.getJSON(statusCurrentKey(), &st, "status")
	if err != nil || !ok {
		return nil, ok, err
	}
	return &st, true, nil
}

// SetStatus persists the current status.
func (s *Store) SetStatus(st *Status) error {
	return s.setJSON(statusCurrentKey(), st, "status")
}

// DeleteStatus removes the stored status.
func (s *Store) DeleteStatus() error {
	return s.deleteKey(statusCurrentKey(), "status")
}

// ReplaceStatusCurrent stores or clears the currently active status projection.
func (s *Store) ReplaceStatusCurrent(current *Status) error {
	if current == nil {
		return s.DeleteStatus()
	}
	return s.SetStatus(current)
}

// GetStatusRawSnapshot returns the latest fetched raw status calendar snapshot.
func (s *Store) GetStatusRawSnapshot() (*CalendarSnapshot, bool, error) {
	var snap CalendarSnapshot
	ok, err := s.getJSON(statusRawKey(), &snap, "status raw snapshot")
	if err != nil || !ok {
		return nil, ok, err
	}
	return &snap, true, nil
}

// SetStatusRawSnapshot persists the latest raw status calendar snapshot.
func (s *Store) SetStatusRawSnapshot(snap *CalendarSnapshot) error {
	return s.setJSON(statusRawKey(), snap, "status raw snapshot")
}

// ReplaceStatusProjection atomically stores the fetched raw status snapshot
// and the currently active status projection.
func (s *Store) ReplaceStatusProjection(raw *CalendarSnapshot, current *Status) error {
	return s.commitBatch("status projection", func(batch *pebble.Batch) error {
		if err := setBatchJSON(batch, statusRawKey(), raw, "status raw snapshot"); err != nil {
			return err
		}
		if current == nil {
			return deleteBatchKey(batch, statusCurrentKey(), "status")
		}
		return setBatchJSON(batch, statusCurrentKey(), current, "status")
	})
}

// GetAvailabilityRawSnapshot returns the stored raw availability calendar snapshot.
func (s *Store) GetAvailabilityRawSnapshot() (*CalendarSnapshot, bool, error) {
	var snap CalendarSnapshot
	ok, err := s.getJSON(availabilityRawKey(), &snap, "availability raw snapshot")
	if err != nil || !ok {
		return nil, ok, err
	}
	return &snap, true, nil
}

// SetAvailabilityRawSnapshot persists the latest raw availability calendar snapshot.
func (s *Store) SetAvailabilityRawSnapshot(snap *CalendarSnapshot) error {
	return s.setJSON(availabilityRawKey(), snap, "availability raw snapshot")
}

// GetAvailabilityCurrent returns the precomputed availability API response JSON.
func (s *Store) GetAvailabilityCurrent() ([]byte, bool, error) {
	return s.getBytes(availabilityCurrentKey(), "current availability")
}

// SetAvailabilityCurrent persists the precomputed availability API response JSON.
func (s *Store) SetAvailabilityCurrent(data []byte) error {
	return s.setBytes(availabilityCurrentKey(), data, "current availability")
}

// ClearAvailabilityCurrent removes the precomputed availability API response JSON.
func (s *Store) ClearAvailabilityCurrent() error {
	return s.deleteKey(availabilityCurrentKey(), "current availability")
}

// ReplaceAvailabilityProjection atomically stores the fetched raw availability
// snapshot and the precomputed API response JSON.
func (s *Store) ReplaceAvailabilityProjection(raw *CalendarSnapshot, currentJSON []byte) error {
	return s.commitBatch("availability projection", func(batch *pebble.Batch) error {
		if err := setBatchJSON(batch, availabilityRawKey(), raw, "availability raw snapshot"); err != nil {
			return err
		}
		if err := batch.Set(availabilityCurrentKey(), currentJSON, nil); err != nil {
			return fmt.Errorf("set current availability: %w", err)
		}
		return nil
	})
}

// GetAvailabilityDirty returns the pending availability entries JSON if the calendar has changed since the last deployment.
func (s *Store) GetAvailabilityDirty() ([]byte, bool, error) {
	return s.getBytes(availabilityDeployDirtyKey(), "availability dirty")
}

// SetAvailabilityDirty persists the pending availability entries JSON.
func (s *Store) SetAvailabilityDirty(data []byte) error {
	return s.setBytes(availabilityDeployDirtyKey(), data, "availability dirty")
}

// ClearAvailabilityDirty removes the availability dirty flag.
func (s *Store) ClearAvailabilityDirty() error {
	return s.deleteKey(availabilityDeployDirtyKey(), "availability dirty")
}

// GetLastDeployedAvailability returns the availability entries JSON from the last successful deployment.
func (s *Store) GetLastDeployedAvailability() ([]byte, bool, error) {
	return s.getBytes(availabilityDeployLastKey(), "availability last deployed")
}

// SetLastDeployedAvailability persists the availability entries JSON from a successful deployment.
func (s *Store) SetLastDeployedAvailability(data []byte) error {
	return s.setBytes(availabilityDeployLastKey(), data, "availability last deployed")
}

// MarkAvailabilityDeployed atomically stores the deployed availability JSON and
// clears the pending dirty marker.
func (s *Store) MarkAvailabilityDeployed(data []byte) error {
	return s.commitBatch("availability deployment state", func(batch *pebble.Batch) error {
		if err := batch.Set(availabilityDeployLastKey(), data, nil); err != nil {
			return fmt.Errorf("set availability last deployed: %w", err)
		}
		return deleteBatchKey(batch, availabilityDeployDirtyKey(), "availability dirty")
	})
}

// GetHolidaySnapshot returns the stored bank holiday snapshot.
func (s *Store) GetHolidaySnapshot() (*HolidaySnapshot, bool, error) {
	var snap HolidaySnapshot
	ok, err := s.getJSON(availabilityHolidaysEnglandKey(), &snap, "holiday snapshot")
	if err != nil || !ok {
		return nil, ok, err
	}
	return &snap, true, nil
}

// SetHolidaySnapshot persists the latest bank holiday snapshot.
func (s *Store) SetHolidaySnapshot(snap *HolidaySnapshot) error {
	return s.setJSON(availabilityHolidaysEnglandKey(), snap, "holiday snapshot")
}
