package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gldraphael/status/internal/target"
)

func TestBuildGraphQLPayload_WithStatus(t *testing.T) {
	st := &target.Status{
		Emoji:      ":rocket:",
		Text:       "Shipping a new feature",
		Expiration: time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC),
	}

	payload := buildGraphQLPayload(st)

	if !strings.Contains(payload.Query, `changeUserStatus`) {
		t.Errorf("mutation missing changeUserStatus")
	}
	input := payload.Variables.Input
	if input.Message != "Shipping a new feature" {
		t.Errorf("message: got %q", input.Message)
	}
	if input.Emoji != ":rocket:" {
		t.Errorf("emoji: got %q", input.Emoji)
	}
	if input.ExpiresAt != "2026-04-07T00:00:00Z" {
		t.Errorf("expiresAt: got %q", input.ExpiresAt)
	}
}

func TestBuildGraphQLPayload_NoExpiry(t *testing.T) {
	st := &target.Status{
		Emoji: ":calendar:",
		Text:  "In a meeting",
	}

	payload := buildGraphQLPayload(st)
	input := payload.Variables.Input

	if input.ExpiresAt != "" {
		t.Errorf("input should not have expiresAt: %+v", input)
	}
	if input.Message != "In a meeting" {
		t.Errorf("message: got %q", input.Message)
	}
}

func TestBuildGraphQLPayload_ClearsStatus(t *testing.T) {
	payload := buildGraphQLPayload(nil)
	input := payload.Variables.Input

	if input.Message != "" {
		t.Errorf("message: got %q, want empty", input.Message)
	}
	if input.Emoji != "" {
		t.Errorf("emoji: got %q, want empty", input.Emoji)
	}
}

func TestGraphQLPayload_IsValidJSON(t *testing.T) {
	st := &target.Status{
		Emoji:      ":smile:",
		Text:       "All systems operational",
		Expiration: time.Now().UTC().Add(2 * time.Hour),
	}

	payload := buildGraphQLPayload(st)
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal to json: %v", err)
	}

	var decoded graphQLPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal from json: %v", err)
	}

	if decoded.Query == "" {
		t.Errorf("decoded json missing query field")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSync_DoesNotMutateStatusWhenExtractingEmoji(t *testing.T) {
	tgt := NewTarget("test-token")
	tgt.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"data":{"changeUserStatus":{"status":{}}}}`)),
				Request:    req,
			}, nil
		}),
	}

	st := &target.Status{Emoji: ":calendar:", Text: "💡 Focusing"}
	if err := tgt.Sync(context.Background(), st); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if st.Emoji != ":calendar:" || st.Text != "💡 Focusing" {
		t.Fatalf("status was mutated: %+v", st)
	}
}

func TestNewTarget(t *testing.T) {
	tgt := NewTarget("test-token")
	if tgt.token != "test-token" {
		t.Errorf("token mismatch")
	}
	if tgt.client == nil {
		t.Errorf("http client should be initialized")
	}
}

func TestExtractFirstEmoji(t *testing.T) {
	tests := []struct {
		input     string
		wantEmoji string
		wantText  string
	}{
		{
			input:     "💡 Focusing... 🎯",
			wantEmoji: "💡",
			wantText:  "Focusing... 🎯",
		},
		{
			input:     "🌘 Unwinding...",
			wantEmoji: "🌘",
			wantText:  "Unwinding...",
		},
		{
			input:     "Meeting",
			wantEmoji: "",
			wantText:  "Meeting",
		},
		{
			input:     "🚀 Rocket! 🚀",
			wantEmoji: "🚀",
			wantText:  "Rocket! 🚀",
		},
		{
			input:     "Flag 🇺🇸 in middle",
			wantEmoji: "🇺🇸",
			wantText:  "Flag  in middle", // Note: double space
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotEmoji, gotText := extractFirstEmoji(tt.input)
			if gotEmoji != tt.wantEmoji {
				t.Errorf("extractFirstEmoji() gotEmoji = %v, want %v", gotEmoji, tt.wantEmoji)
			}
			if gotText != tt.wantText {
				t.Errorf("extractFirstEmoji() gotText = %v, want %v", gotText, tt.wantText)
			}
		})
	}
}
