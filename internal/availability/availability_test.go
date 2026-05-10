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
		{Name: "Morning", Start: "09:00", End: "12:00"},
		{Name: "Afternoon", Start: "12:00", End: "16:30"},
		{Name: "Evening", Start: "17:30", End: "22:00"},
	})
	if err != nil {
		t.Fatalf("ParseBlocks: %v", err)
	}
	return blocks
}

func testWorkingHours(t *testing.T) WorkingHours {
	t.Helper()
	workingHours, err := ParseWorkingHours("09:00", "17:50")
	if err != nil {
		t.Fatalf("ParseWorkingHours: %v", err)
	}
	return workingHours
}

func londonTime(t *testing.T, year int, month time.Month, day, hour, minute int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	return time.Date(year, month, day, hour, minute, 0, 0, loc)
}

func baseCalendarBody() string {
	return `BEGIN:VCALENDAR
VERSION:2.0
X-WR-TIMEZONE:Europe/London
END:VCALENDAR`
}

func holidayCalendarBody() string {
	return `{"england-and-wales":{"division":"england-and-wales","events":[{"date":"2026-04-06","title":"Easter Monday"},{"date":"2026-12-25","title":"Christmas Day"}]}}`
}

func TestCompute_SuppressesWeekdayWorkingHours(t *testing.T) {
	blocks := testBlocks(t)
	opts := ComputeOptions{
		WorkingHours: testWorkingHours(t),
		Now:          londonTime(t, 2026, 4, 7, 8, 30), // Tuesday
	}

	entries, err := Compute(baseCalendarBody(), "Europe/London", blocks, opts)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected availability entries")
	}
	if got, want := entries[0].Date, "2026-04-11"; got != want {
		t.Fatalf("first available date: got %q, want %q", got, want)
	}
	if got, want := entries[0].Block, "Morning"; got != want {
		t.Fatalf("first available block: got %q, want %q", got, want)
	}
}

func TestCompute_AllowsWeekendBlocks(t *testing.T) {
	blocks := testBlocks(t)
	opts := ComputeOptions{
		WorkingHours: testWorkingHours(t),
		Now:          londonTime(t, 2026, 4, 11, 8, 30), // Saturday
	}

	entries, err := Compute(baseCalendarBody(), "Europe/London", blocks, opts)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected availability entries")
	}
	if got, want := entries[0].Date, "2026-04-11"; got != want {
		t.Fatalf("first available date: got %q, want %q", got, want)
	}
	if got, want := entries[0].Block, "Morning"; got != want {
		t.Fatalf("first available block: got %q, want %q", got, want)
	}
}

func TestCompute_AllowsBankHolidayBlocks(t *testing.T) {
	blocks := testBlocks(t)
	opts := ComputeOptions{
		WorkingHours:               testWorkingHours(t),
		HolidayDates:               []string{"2026-04-06"},
		ExcludeEnglandBankHolidays: true,
		Now:                        londonTime(t, 2026, 4, 6, 8, 30), // Easter Monday
	}

	entries, err := Compute(baseCalendarBody(), "Europe/London", blocks, opts)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected availability entries")
	}
	if got, want := entries[0].Date, "2026-04-06"; got != want {
		t.Fatalf("first available date: got %q, want %q", got, want)
	}
	if got, want := entries[0].Block, "Morning"; got != want {
		t.Fatalf("first available block: got %q, want %q", got, want)
	}
}

func TestCompute_SkipsStartedBlocks(t *testing.T) {
	blocks := testBlocks(t)
	opts := ComputeOptions{
		WorkingHours:               testWorkingHours(t),
		HolidayDates:               []string{"2026-04-06"},
		ExcludeEnglandBankHolidays: true,
		Now:                        londonTime(t, 2026, 4, 6, 9, 30),
	}

	entries, err := Compute(baseCalendarBody(), "Europe/London", blocks, opts)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected availability entries")
	}
	if got, want := entries[0].Date, "2026-04-06"; got != want {
		t.Fatalf("first available date: got %q, want %q", got, want)
	}
	if got, want := entries[0].Block, "Afternoon"; got != want {
		t.Fatalf("first available block: got %q, want %q", got, want)
	}
}

func TestCompute_RespectsCalendarEvents(t *testing.T) {
	blocks := testBlocks(t)
	body := `BEGIN:VCALENDAR
VERSION:2.0
X-WR-TIMEZONE:Europe/London
BEGIN:VEVENT
UID:busy-morning
DTSTART:20260406T090000
DTEND:20260406T103000
SUMMARY:Busy morning
END:VEVENT
END:VCALENDAR`

	opts := ComputeOptions{
		WorkingHours:               testWorkingHours(t),
		HolidayDates:               []string{"2026-04-06"},
		ExcludeEnglandBankHolidays: true,
		Now:                        londonTime(t, 2026, 4, 6, 8, 30),
	}

	entries, err := Compute(body, "Europe/London", blocks, opts)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected availability entries")
	}
	if got, want := entries[0].Date, "2026-04-06"; got != want {
		t.Fatalf("first available date: got %q, want %q", got, want)
	}
	if got, want := entries[0].Block, "Afternoon"; got != want {
		t.Fatalf("first available block: got %q, want %q", got, want)
	}
}

func TestCompute_IgnoresTransparentEvents(t *testing.T) {
	blocks := testBlocks(t)
	body := `BEGIN:VCALENDAR
VERSION:2.0
X-WR-TIMEZONE:Europe/London
BEGIN:VEVENT
UID:free-morning
DTSTART:20260406T090000
DTEND:20260406T103000
SUMMARY:Free morning
TRANSP:TRANSPARENT
END:VEVENT
END:VCALENDAR`

	opts := ComputeOptions{
		WorkingHours:               testWorkingHours(t),
		HolidayDates:               []string{"2026-04-06"},
		ExcludeEnglandBankHolidays: true,
		Now:                        londonTime(t, 2026, 4, 6, 8, 30),
	}

	entries, err := Compute(body, "Europe/London", blocks, opts)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected availability entries")
	}
	// With the transparent event, the Morning block should still be available.
	if got, want := entries[0].Date, "2026-04-06"; got != want {
		t.Fatalf("first available date: got %q, want %q", got, want)
	}
	if got, want := entries[0].Block, "Morning"; got != want {
		t.Fatalf("first available block: got %q, want %q", got, want)
	}
}

func TestHandler_AuthorizationAndResponse(t *testing.T) {
	st := newTestStore(t)
	blocks := testBlocks(t)
	if err := st.SetAvailabilitySnapshot(&store.AvailabilitySnapshot{
		Body:      baseCalendarBody(),
		Timezone:  "Europe/London",
		FetchedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetAvailabilitySnapshot: %v", err)
	}

	p := NewProvider(st, blocks, testWorkingHours(t), false)
	h := NewHandler(p, "secret", zerolog.Nop())

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

func TestHandler_HolidaySnapshotRequiredWhenEnabled(t *testing.T) {
	st := newTestStore(t)
	blocks := testBlocks(t)
	if err := st.SetAvailabilitySnapshot(&store.AvailabilitySnapshot{
		Body:      baseCalendarBody(),
		Timezone:  "Europe/London",
		FetchedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetAvailabilitySnapshot: %v", err)
	}

	p := NewProvider(st, blocks, testWorkingHours(t), true)
	h := NewHandler(p, "secret", zerolog.Nop())

	req := httptest.NewRequest(http.MethodGet, "/api/availability", nil)
	req.Header.Set("Authorization", "secret")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestProvider_GetEntriesJSON(t *testing.T) {
	st := newTestStore(t)
	blocks := testBlocks(t)
	if err := st.SetAvailabilitySnapshot(&store.AvailabilitySnapshot{
		Body:      baseCalendarBody(),
		Timezone:  "Europe/London",
		FetchedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SetAvailabilitySnapshot: %v", err)
	}

	p := NewProvider(st, blocks, testWorkingHours(t), false)

	data, err := p.GetEntriesJSON(context.Background())
	if err != nil {
		t.Fatalf("GetEntriesJSON: %v", err)
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected entries")
	}
}

func TestSyncer_StoresAvailabilitySnapshot(t *testing.T) {
	st := newTestStore(t)
	blocks := testBlocks(t)
	p := NewProvider(st, blocks, testWorkingHours(t), false)
	cal := &mockFetchClient{
		body: baseCalendarBody(),
	}
	s := NewSyncer(st, p, cal, zerolog.Nop())

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatalf("syncOnce: %v", err)
	}

	snap, ok, err := st.GetAvailabilitySnapshot()
	if err != nil {
		t.Fatalf("GetAvailabilitySnapshot: %v", err)
	}
	if !ok {
		t.Fatal("expected stored availability snapshot")
	}
	if snap.Body == "" || snap.Timezone != "Europe/London" {
		t.Fatalf("unexpected availability snapshot: %+v", snap)
	}
}

func TestSyncer_PreservesAvailabilitySnapshotOnFetchError(t *testing.T) {
	st := newTestStore(t)
	blocks := testBlocks(t)
	p := NewProvider(st, blocks, testWorkingHours(t), false)
	seed := &store.AvailabilitySnapshot{
		Body:      "seed",
		Timezone:  "UTC",
		FetchedAt: time.Now().UTC(),
	}
	if err := st.SetAvailabilitySnapshot(seed); err != nil {
		t.Fatalf("SetAvailabilitySnapshot: %v", err)
	}

	s := NewSyncer(st, p, &mockFetchClient{err: context.Canceled}, zerolog.Nop())

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

func TestSyncHolidaySnapshot_StoresSnapshot(t *testing.T) {
	st := newTestStore(t)
	client := &mockFetchClient{body: holidayCalendarBody()}

	if err := SyncHolidaySnapshot(context.Background(), st, client, zerolog.Nop()); err != nil {
		t.Fatalf("SyncHolidaySnapshot: %v", err)
	}

	snap, ok, err := st.GetHolidaySnapshot()
	if err != nil {
		t.Fatalf("GetHolidaySnapshot: %v", err)
	}
	if !ok {
		t.Fatal("expected stored holiday snapshot")
	}
	if snap.Body == "" || len(snap.Dates) != 2 {
		t.Fatalf("unexpected holiday snapshot: %+v", snap)
	}
	if snap.Dates[0] != "2026-04-06" || snap.Dates[1] != "2026-12-25" {
		t.Fatalf("unexpected holiday dates: %+v", snap.Dates)
	}
}

func TestSyncer_SetsAvailabilityDirtyOnChanges(t *testing.T) {
	st := newTestStore(t)
	blocks := testBlocks(t)
	p := NewProvider(st, blocks, testWorkingHours(t), false)
	// Fix time to make the 10-day window deterministic.
	p.nowFunc = func() time.Time {
		return londonTime(t, 2026, 4, 6, 8, 30)
	}

	cal := &mockFetchClient{
		body: baseCalendarBody(),
	}
	s := NewSyncer(st, p, cal, zerolog.Nop())

	// First sync should set dirty JSON since there's no last deployed state.
	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatalf("first syncOnce: %v", err)
	}
	dirtyJSON, ok, err := st.GetAvailabilityDirty()
	if err != nil {
		t.Fatalf("first GetAvailabilityDirty: %v", err)
	}
	if !ok {
		t.Error("expected dirty JSON after first sync")
	}

	// Pretend we deployed.
	if err := st.SetLastDeployedAvailability(dirtyJSON); err != nil {
		t.Fatalf("SetLastDeployedAvailability: %v", err)
	}
	if err := st.ClearAvailabilityDirty(); err != nil {
		t.Fatalf("ClearAvailabilityDirty: %v", err)
	}

	// Second sync with same body should not set dirty JSON.
	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatalf("second syncOnce: %v", err)
	}
	_, ok, err = st.GetAvailabilityDirty()
	if err != nil {
		t.Fatalf("second GetAvailabilityDirty: %v", err)
	}
	if ok {
		t.Error("expected no dirty JSON after second sync with same body")
	}

	// Third sync with different body should set dirty JSON.
	// Add an event within the 10-day window (2026-04-06 to 2026-04-16).
	cal.body = `BEGIN:VCALENDAR
VERSION:2.0
X-WR-TIMEZONE:Europe/London
BEGIN:VEVENT
UID:new-event
DTSTART:20260411T090000
DTEND:20260411T120000
SUMMARY:New event
END:VEVENT
END:VCALENDAR`

	if err := s.syncOnce(context.Background()); err != nil {
		t.Fatalf("third syncOnce: %v", err)
	}
	_, ok, err = st.GetAvailabilityDirty()
	if err != nil {
		t.Fatalf("third GetAvailabilityDirty: %v", err)
	}
	if !ok {
		t.Error("expected dirty JSON after third sync with changed body")
	}
}

type mockFetchClient struct {
	body string
	err  error
}

func (m *mockFetchClient) Fetch(_ context.Context) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []byte(m.body), nil
}
