package calendar

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/store"
	"github.com/gldraphael/status/internal/target"
)

// --- mocks ---

type mockCalendarClient struct {
	body []byte
	err  error
}

func (m *mockCalendarClient) Fetch(_ context.Context) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.body, nil
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

type statusEventSpec struct {
	id        string
	summary   string
	startTime time.Time
	endTime   time.Time
	cancelled bool
}

func statusCalendarBody(events ...statusEventSpec) []byte {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\n")
	b.WriteString("VERSION:2.0\n")
	b.WriteString("X-WR-TIMEZONE:UTC\n")
	for _, ev := range events {
		b.WriteString("BEGIN:VEVENT\n")
		b.WriteString("UID:" + ev.id + "\n")
		b.WriteString("DTSTART:" + ev.startTime.UTC().Format("20060102T150405Z") + "\n")
		b.WriteString("DTEND:" + ev.endTime.UTC().Format("20060102T150405Z") + "\n")
		b.WriteString("SUMMARY:" + ev.summary + "\n")
		if ev.cancelled {
			b.WriteString("STATUS:CANCELLED\n")
		}
		b.WriteString("END:VEVENT\n")
	}
	b.WriteString("END:VCALENDAR")
	return []byte(b.String())
}

// --- syncOnce tests ---

func TestSyncOnce_SetsStatusFromActiveEvent(t *testing.T) {
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	cal := &mockCalendarClient{
		body: statusCalendarBody(statusEventSpec{
			id:        "e1",
			summary:   "Team Sync",
			startTime: now.Add(-10 * time.Minute),
			endTime:   now.Add(50 * time.Minute),
		}),
	}
	tgt := &mockTarget{}
	s, st := newTestSyncer(t, cal, []target.Target{tgt})
	s.nowFunc = func() time.Time { return now }

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

	// Verify the fetched raw snapshot is stored.
	snap, ok, err := st.GetStatusRawSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected status raw snapshot")
	}
	if !strings.Contains(snap.Body, "SUMMARY:Team Sync") || snap.Timezone != "UTC" {
		t.Errorf("stored status raw snapshot: got %+v", snap)
	}
}

func TestSyncOnce_ClearsStatusWhenNoActiveEvents(t *testing.T) {
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	tgt := &mockTarget{}
	cal := &mockCalendarClient{
		body: statusCalendarBody(statusEventSpec{
			id:        "e1",
			summary:   "Past meeting",
			startTime: now.Add(-2 * time.Hour),
			endTime:   now.Add(-1 * time.Hour),
		}),
	}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt})
	s.nowFunc = func() time.Time { return now }

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
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	tgt1, tgt2 := &mockTarget{}, &mockTarget{}
	cal := &mockCalendarClient{
		body: statusCalendarBody(statusEventSpec{
			id:        "e1",
			summary:   "Standup",
			startTime: now.Add(-5 * time.Minute),
			endTime:   now.Add(25 * time.Minute),
		}),
	}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt1, tgt2})
	s.nowFunc = func() time.Time { return now }

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
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	tgt := &mockTarget{}
	cal := &mockCalendarClient{
		body: statusCalendarBody(statusEventSpec{
			id:        "c1",
			summary:   "Cancelled meeting",
			startTime: now.Add(-15 * time.Minute),
			endTime:   now.Add(45 * time.Minute),
			cancelled: true,
		}),
	}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt})
	s.nowFunc = func() time.Time { return now }

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	if tgt.status != nil {
		t.Error("expected Sync(nil): cancelled events must not set status")
	}
}

func TestSyncOnce_SelectsEarliestEndingActiveEvent(t *testing.T) {
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	tgt := &mockTarget{}
	cal := &mockCalendarClient{
		body: statusCalendarBody(
			statusEventSpec{
				id:        "later",
				summary:   "Later event",
				startTime: now.Add(-20 * time.Minute),
				endTime:   now.Add(60 * time.Minute),
			},
			statusEventSpec{
				id:        "earlier",
				summary:   "Earlier event",
				startTime: now.Add(-10 * time.Minute),
				endTime:   now.Add(30 * time.Minute),
			},
		),
	}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt})
	s.nowFunc = func() time.Time { return now }

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	if tgt.status == nil {
		t.Fatal("expected Sync with non-nil status")
	}
	if tgt.status.Text != "Earlier event" {
		t.Errorf("status text: got %q, want %q", tgt.status.Text, "Earlier event")
	}
}

func TestSyncOnce_StoresPersistentStatus(t *testing.T) {
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	tgt := &mockTarget{}
	cal := &mockCalendarClient{
		body: statusCalendarBody(statusEventSpec{
			id:        "e1",
			summary:   "Design Review",
			startTime: now.Add(-15 * time.Minute),
			endTime:   now.Add(45 * time.Minute),
		}),
	}
	s, st := newTestSyncer(t, cal, []target.Target{tgt})
	s.nowFunc = func() time.Time { return now }

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

func TestSyncOnce_UsesCachedRawSnapshotOnFetchError(t *testing.T) {
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	body := statusCalendarBody(statusEventSpec{
		id:        "cached",
		summary:   "Cached Status",
		startTime: now.Add(-15 * time.Minute),
		endTime:   now.Add(45 * time.Minute),
	})
	cal := &mockCalendarClient{err: context.DeadlineExceeded}
	tgt := &mockTarget{}
	s, st := newTestSyncer(t, cal, []target.Target{tgt})
	s.nowFunc = func() time.Time { return now }
	if err := st.SetStatusRawSnapshot(&store.CalendarSnapshot{
		Body:      string(body),
		Timezone:  "UTC",
		FetchedAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("SetStatusRawSnapshot: %v", err)
	}

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !tgt.synced || tgt.status == nil {
		t.Fatal("expected target sync from cached raw snapshot")
	}
	if tgt.status.Text != "Cached Status" {
		t.Fatalf("status text: got %q, want %q", tgt.status.Text, "Cached Status")
	}
	stored, ok, err := st.GetStatus()
	if err != nil || !ok {
		t.Fatalf("status not stored: err=%v ok=%v", err, ok)
	}
	if stored.Text != "Cached Status" {
		t.Fatalf("stored status text: got %q, want %q", stored.Text, "Cached Status")
	}
}

func TestSyncOnce_FetchErrorWithoutCachedRawPreservesCurrentStatus(t *testing.T) {
	cal := &mockCalendarClient{err: context.DeadlineExceeded}
	tgt := &mockTarget{}
	s, st := newTestSyncer(t, cal, []target.Target{tgt})
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	s.nowFunc = func() time.Time { return now }
	if err := st.SetStatus(&store.Status{Emoji: ":calendar:", Text: "Old"}); err != nil {
		t.Fatal(err)
	}

	if err := s.syncOnce(context.Background()); err == nil {
		t.Fatal("expected syncOnce to fail without cached raw snapshot")
	}
	if tgt.synced {
		t.Fatal("expected target not to sync without fetched or cached data")
	}
	stored, ok, err := st.GetStatus()
	if err != nil || !ok {
		t.Fatalf("status not stored: err=%v ok=%v", err, ok)
	}
	if stored.Text != "Old" {
		t.Fatalf("stored status changed: got %q, want %q", stored.Text, "Old")
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
	cal := &mockCalendarClient{body: statusCalendarBody()}
	s := NewSyncer(st, cal, []target.Target{tgt}, zerolog.Nop())
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	s.nowFunc = func() time.Time { return now }

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
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	cal := &mockCalendarClient{
		body: statusCalendarBody(statusEventSpec{
			id:        "e1",
			summary:   "Boot Sync",
			startTime: now.Add(-10 * time.Minute),
			endTime:   now.Add(50 * time.Minute),
		}),
	}
	tgt := &mockTarget{}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt})
	s.nowFunc = func() time.Time { return now }

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
	cal := &mockCalendarClient{body: statusCalendarBody()}
	tgt := &mockTarget{}
	s, _ := newTestSyncer(t, cal, []target.Target{tgt})
	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	s.nowFunc = func() time.Time { return now }

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
