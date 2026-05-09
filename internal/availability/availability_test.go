package availability

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/config"
	"github.com/gldraphael/status/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func testBlocks(t *testing.T) []Block {
	t.Helper()
	blocks, err := ParseBlocks([]config.AvailabilityBlockConfig{
		{Name: "First half", Start: "09:00", End: "15:00"},
		{Name: "Second half", Start: "14:00", End: "20:00"},
		{Name: "Morning", Start: "09:00", End: "12:00"},
		{Name: "Afternoon", Start: "12:00", End: "16:30"},
		{Name: "Evening", Start: "17:30", End: "22:00"},
	})
	if err != nil {
		t.Fatalf("ParseBlocks: %v", err)
	}
	return blocks
}

func TestCompute_SelectsFirstUpcomingFreeBlock(t *testing.T) {
	blocks := testBlocks(t)
	body := `BEGIN:VCALENDAR
VERSION:2.0
X-WR-TIMEZONE:Europe/London
BEGIN:VEVENT
UID:busy-1
DTSTART:20260406T083000Z
DTEND:20260406T090000Z
SUMMARY:Busy
END:VEVENT
END:VCALENDAR`

	now := time.Date(2026, 4, 6, 8, 30, 0, 0, time.UTC)
	entries, err := Compute(body, "Europe/London", blocks, now)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected availability entries")
	}
	if got, want := entries[0].Date, "2026-04-06"; got != want {
		t.Fatalf("first date: got %q, want %q", got, want)
	}
	if got, want := entries[0].Block, "Second half"; got != want {
		t.Fatalf("first block: got %q, want %q", got, want)
	}
	if got, want := entries[0].DayOfWeek, "Monday"; got != want {
		t.Fatalf("first day_of_week: got %q, want %q", got, want)
	}
}

func TestCompute_OmitsTodayWhenNoBlockIsFree(t *testing.T) {
	blocks := testBlocks(t)
	body := `BEGIN:VCALENDAR
VERSION:2.0
X-WR-TIMEZONE:Europe/London
BEGIN:VEVENT
UID:busy-all-day
DTSTART:20260406T120000Z
DTEND:20260406T220000Z
SUMMARY:Busy
END:VEVENT
END:VCALENDAR`

	now := time.Date(2026, 4, 6, 8, 30, 0, 0, time.UTC)
	entries, err := Compute(body, "Europe/London", blocks, now)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected future availability entries")
	}
	if got, want := entries[0].Date, "2026-04-07"; got != want {
		t.Fatalf("first returned day should skip today: got %q, want %q", got, want)
	}
}

func TestHandler_AuthorizationAndResponse(t *testing.T) {
	st := newTestStore(t)
	blocks := testBlocks(t)
	body := `BEGIN:VCALENDAR
VERSION:2.0
X-WR-TIMEZONE:Europe/London
END:VCALENDAR`
	if err := st.SetAvailabilitySnapshot(&store.AvailabilitySnapshot{
		Body:      body,
		Timezone:  "Europe/London",
		FetchedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetAvailabilitySnapshot: %v", err)
	}

	h := NewHandler(st, "secret", blocks, zerolog.Nop())

	t.Run("unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/availability", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("authorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/availability", nil)
		req.Header.Set("Authorization", "secret")
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
		}

		var entries []Entry
		if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if len(entries) == 0 {
			t.Fatal("expected at least one availability entry")
		}
		if entries[0].Date == "" || entries[0].Block == "" || entries[0].DayOfWeek == "" {
			t.Fatalf("unexpected response entry: %+v", entries[0])
		}
	})
}

func TestHandler_MissingSnapshot(t *testing.T) {
	st := newTestStore(t)
	h := NewHandler(st, "secret", testBlocks(t), zerolog.Nop())

	req := httptest.NewRequest(http.MethodGet, "/api/availability", nil)
	req.Header.Set("Authorization", "secret")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestSyncer_StoresSnapshot(t *testing.T) {
	st := newTestStore(t)
	client := &mockAvailabilityClient{
		body: `BEGIN:VCALENDAR
VERSION:2.0
X-WR-TIMEZONE:Europe/London
END:VCALENDAR`,
	}
	s := NewSyncer(st, client, zerolog.Nop())

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatalf("syncOnce: %v", err)
	}

	snap, ok, err := st.GetAvailabilitySnapshot()
	if err != nil {
		t.Fatalf("GetAvailabilitySnapshot: %v", err)
	}
	if !ok {
		t.Fatal("expected stored snapshot")
	}
	if snap.Body == "" || snap.Timezone != "Europe/London" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

func TestSyncer_PreservesSnapshotOnFetchError(t *testing.T) {
	st := newTestStore(t)
	seed := &store.AvailabilitySnapshot{
		Body:      "seed",
		Timezone:  "UTC",
		FetchedAt: time.Now().UTC(),
	}
	if err := st.SetAvailabilitySnapshot(seed); err != nil {
		t.Fatalf("SetAvailabilitySnapshot: %v", err)
	}

	client := &mockAvailabilityClient{err: context.Canceled}
	s := NewSyncer(st, client, zerolog.Nop())

	if err := s.syncOnce(context.Background()); err == nil {
		t.Fatal("expected syncOnce to fail")
	}

	snap, ok, err := st.GetAvailabilitySnapshot()
	if err != nil {
		t.Fatalf("GetAvailabilitySnapshot: %v", err)
	}
	if !ok {
		t.Fatal("expected snapshot to remain after failure")
	}
	if snap.Body != seed.Body || snap.Timezone != seed.Timezone {
		t.Fatalf("snapshot changed unexpectedly: %+v", snap)
	}
}

type mockAvailabilityClient struct {
	body string
	err  error
}

func (m *mockAvailabilityClient) Fetch(_ context.Context) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []byte(m.body), nil
}
