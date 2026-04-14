package github

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gldraphael/status/internal/target"
)

func TestBuildGraphQLMutation_WithStatus(t *testing.T) {
	st := &target.Status{
		Emoji:      ":rocket:",
		Text:       "Shipping a new feature",
		Expiration: time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC),
	}

	mutation := buildGraphQLMutation(st)

	// Verify the mutation contains the expected components
	if !strings.Contains(mutation, `changeUserStatus`) {
		t.Errorf("mutation missing changeUserStatus")
	}
	if !strings.Contains(mutation, `"Shipping a new feature"`) {
		t.Errorf("mutation missing message: %s", mutation)
	}
	if !strings.Contains(mutation, `":rocket:"`) {
		t.Errorf("mutation missing emoji: %s", mutation)
	}
	if !strings.Contains(mutation, `2026-04-07T00:00:00Z`) {
		t.Errorf("mutation missing expiry: %s", mutation)
	}
}

func TestBuildGraphQLMutation_NoExpiry(t *testing.T) {
	st := &target.Status{
		Emoji: ":calendar:",
		Text:  "In a meeting",
	}

	mutation := buildGraphQLMutation(st)

	// Should not have expiresAt in the input when expiration is zero
	// Check only the input part (before the closing bracket)
	inputPart := strings.Split(mutation, "}")[0]
	if strings.Contains(inputPart, `expiresAt`) {
		t.Errorf("mutation input should not have expiresAt: %s", mutation)
	}
	if !strings.Contains(mutation, `"In a meeting"`) {
		t.Errorf("mutation missing message")
	}
}

func TestBuildGraphQLMutation_ClearsStatus(t *testing.T) {
	mutation := buildGraphQLMutation(nil)

	// Clearing status uses empty strings
	if !strings.Contains(mutation, `message: ""`) {
		t.Errorf("mutation should clear message: %s", mutation)
	}
	if !strings.Contains(mutation, `emoji: ""`) {
		t.Errorf("mutation should clear emoji: %s", mutation)
	}
}

func TestEscapeGraphQLString_WithQuotes(t *testing.T) {
	result := escapeGraphQLString(`It's "quoted"`)
	expected := `"It's \"quoted\""`
	if result != expected {
		t.Errorf("escapeGraphQLString: got %q, want %q", result, expected)
	}
}

func TestEscapeGraphQLString_WithBackslash(t *testing.T) {
	result := escapeGraphQLString(`C:\path\to\file`)
	expected := `"C:\\path\\to\\file"`
	if result != expected {
		t.Errorf("escapeGraphQLString: got %q, want %q", result, expected)
	}
}

func TestEscapeGraphQLString_Empty(t *testing.T) {
	result := escapeGraphQLString("")
	expected := `""`
	if result != expected {
		t.Errorf("escapeGraphQLString: got %q, want %q", result, expected)
	}
}

func TestGraphQLMutation_IsValidJSON(t *testing.T) {
	st := &target.Status{
		Emoji:      ":smile:",
		Text:       "All systems operational",
		Expiration: time.Now().UTC().Add(2 * time.Hour),
	}

	mutation := buildGraphQLMutation(st)
	payload := map[string]string{"query": mutation}

	// Should be serializable to JSON
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal to json: %v", err)
	}

	// Verify it can be unmarshaled
	var decoded map[string]string
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal from json: %v", err)
	}

	if _, ok := decoded["query"]; !ok {
		t.Errorf("decoded json missing query field")
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
