package calendar

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/store"
	"github.com/gldraphael/status/internal/target"
)

// --- mocks ---

type mockCalendarClient struct {
	events []ChangedEvent
	err    error
}

func (m *mockCalendarClient) FetchEvents(_ context.Context) ([]ChangedEvent, error) {
	return m.events, m.err
}

// mockTarget records the last Sync call for assertions.
type mockTarget struct {
	status *target.Status
	synced bool
	err    error
}

func (m *mockTarget) Sync(_ context.Context, st *target.Status) error {
	m.synced = true
	m.status = st
	return m.err
}

// --- helpers ---

func newTestSyncer(t *testing.T, cal calendarClient, targets []target.Target) (*Syncer, *store.Store) {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	s := NewSyncer(st, cal, targets, zerolog.Nop())
	return s, st
}

// --- syncOnce tests ---

func TestSyncOnce_SetsStatusFromActiveEvent(t *testing.T) {
	now := time.Now()
	cal := &mockCalendarClient{
		events: []ChangedEvent{
			{
				ID:        "e1",
				Summary:   "Team Sync",
				StartTime: now.Add(-10 * time.Minute),
				EndTime:   now.Add(50 * time.Minute),
			},
		},
	}
	tgt := &mockTarget{}
	s, st := newTestSyncer(t, cal, []target.Target{tgt})

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !tgt.synced || tgt.status == nil {
		t.Fatal("expected Sync to be called with a non-nil status")
	}
	if tgt.status.Text != "Team Sync" {
		t.Errorf("status text: got %q, want %q", tgt.status.Text, "Team Sync")
	}
	if tgt.status.Emoji != ":calendar:" {
		t.Errorf("status emoji: got %q, want %q", tgt.status.Emoji, ":calendar:")
	}

	// Verify event is stored.
	events, err := st.ListActiveEvents(now)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Summary != "Team Sync" {
		t.Errorf("stored event: got %v", events)
	}
}

func TestSyncOnce_ClearsStatusWhenNoActiveEvents(t *testing.T) {
	now := time.Now()
	tgt := &mockTarget{}
	cal := &mockCalendarClient{
		events: []ChangedEvent{
			{
				ID:        "e1",
				Summary:   "Past meeting",
				StartTime: now.Add(-2 * time.Hour),
				EndTime:   now.Add(-1 * time.Hour),
			},
		},
	}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt})

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !tgt.synced {
		t.Fatal("expected Sync to be called")
	}
	if tgt.status != nil {
		t.Error("expected Sync(nil): no active events")
	}
}

func TestSyncOnce_MultipleTargets(t *testing.T) {
	now := time.Now()
	tgt1, tgt2 := &mockTarget{}, &mockTarget{}
	cal := &mockCalendarClient{
		events: []ChangedEvent{
			{
				ID:        "e1",
				Summary:   "Standup",
				StartTime: now.Add(-5 * time.Minute),
				EndTime:   now.Add(25 * time.Minute),
			},
		},
	}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt1, tgt2})

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	for i, tgt := range []*mockTarget{tgt1, tgt2} {
		if !tgt.synced || tgt.status == nil {
			t.Errorf("target %d: expected Sync with non-nil status", i+1)
		}
		if tgt.status.Text != "Standup" {
			t.Errorf("target %d: wrong status text", i+1)
		}
	}
}

func TestSyncOnce_CancelledEventsIgnored(t *testing.T) {
	now := time.Now()
	tgt := &mockTarget{}
	cal := &mockCalendarClient{
		events: []ChangedEvent{
			{
				ID:        "c1",
				Summary:   "Cancelled meeting",
				StartTime: now.Add(-15 * time.Minute),
				EndTime:   now.Add(45 * time.Minute),
				Cancelled: true,
			},
		},
	}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt})

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	if tgt.status != nil {
		t.Error("expected Sync(nil): cancelled events must not set status")
	}
}

func TestSyncOnce_StoresPersistentStatus(t *testing.T) {
	now := time.Now()
	tgt := &mockTarget{}
	cal := &mockCalendarClient{
		events: []ChangedEvent{
			{
				ID:        "e1",
				Summary:   "Design Review",
				StartTime: now.Add(-15 * time.Minute),
				EndTime:   now.Add(45 * time.Minute),
			},
		},
	}
	s, st := newTestSyncer(t, cal, []target.Target{tgt})

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Verify status is persisted.
	stored, ok, err := st.GetStatus()
	if err != nil || !ok {
		t.Fatalf("status not stored: err=%v ok=%v", err, ok)
	}
	if stored.Text != "Design Review" {
		t.Errorf("stored status text: got %q", stored.Text)
	}
	if stored.Emoji != ":calendar:" {
		t.Errorf("stored status emoji: got %q", stored.Emoji)
	}
}

func TestSyncOnce_DeletesStatusWhenIdle(t *testing.T) {
	// Seed with a status.
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	if err := st.SetStatus(&store.Status{Emoji: ":calendar:", Text: "Old"}); err != nil {
		t.Fatal(err)
	}

	// Sync with no active events should delete the status.
	tgt := &mockTarget{}
	cal := &mockCalendarClient{events: []ChangedEvent{}}
	s := NewSyncer(st, cal, []target.Target{tgt}, zerolog.Nop())

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, ok, err := st.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected stored status to be deleted")
	}
}

// --- Run loop tests ---

func TestRun_SyncsImmediatelyOnStartup(t *testing.T) {
	now := time.Now()
	cal := &mockCalendarClient{
		events: []ChangedEvent{
			{
				ID:        "e1",
				Summary:   "Boot Sync",
				StartTime: now.Add(-10 * time.Minute),
				EndTime:   now.Add(50 * time.Minute),
			},
		},
	}
	tgt := &mockTarget{}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run should call syncOnce immediately.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	if err := s.Run(ctx, 10*time.Second); err != nil {
		t.Fatal(err)
	}

	if !tgt.synced {
		t.Error("expected sync on startup")
	}
}

func TestRun_ContextCancellationStopsLoop(t *testing.T) {
	cal := &mockCalendarClient{events: []ChangedEvent{}}
	tgt := &mockTarget{}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	if err := s.Run(ctx, 1*time.Second); err != nil {
		t.Fatal(err)
	}
	// Should exit cleanly when context is cancelled.
}
