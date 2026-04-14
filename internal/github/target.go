package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/forPelevin/gomoji"
	"github.com/gldraphael/status/internal/target"
)

// Target syncs status with the GitHub user profile status API using personal access tokens.
// The GitHub Profile Status API requires GraphQL mutations, so we use direct GraphQL requests.
//
// Required token scope: user
type Target struct {
	token  string
	client *http.Client
}

// NewTarget creates a GitHub target for the given personal access token.
func NewTarget(token string) *Target {
	return &Target{
		token: token,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Sync implements target.Target. A nil status clears the GitHub user profile status.
func (t *Target) Sync(ctx context.Context, st *target.Status) error {
	if st != nil {
		emoji, text := extractFirstEmoji(st.Text)
		if emoji != "" {
			st.Emoji = emoji
			st.Text = text
		}
	}

	mutation := buildGraphQLMutation(st)
	payload := map[string]string{"query": mutation}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal graphql: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://api.github.com/graphql",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("github request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the GraphQL response to check for errors
	var gqlResp graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return fmt.Errorf("decode graphql response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("graphql error: %v", gqlResp.Errors[0].Message)
	}

	return nil
}

// graphQLResponse represents the response structure from GitHub's GraphQL API.
type graphQLResponse struct {
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Data interface{} `json:"data"`
}

// buildGraphQLMutation constructs the GraphQL mutation for changing user status.
// When st is nil, it clears the status by setting empty strings.
func buildGraphQLMutation(st *target.Status) string {
	if st == nil {
		// Clear status by setting message and emoji to empty strings (GitHub treats empty as clear)
		return `mutation { changeUserStatus(input: { message: "", emoji: "" }) { status { message emoji expiresAt } } }`
	}

	message := escapeGraphQLString(st.Text)
	emoji := escapeGraphQLString(st.Emoji)

	// Build the mutation with expiration if present
	var expiresAtArg string
	if !st.Expiration.IsZero() {
		expiresAt := escapeGraphQLString(st.Expiration.UTC().Format(time.RFC3339))
		expiresAtArg = fmt.Sprintf(`, expiresAt: %s`, expiresAt)
	}

	return fmt.Sprintf(
		`mutation { changeUserStatus(input: { message: %s, emoji: %s%s }) { status { message emoji expiresAt } } }`,
		message, emoji, expiresAtArg,
	)
}

// escapeGraphQLString escapes a string for use in a GraphQL query.
// It handles backslashes and quotes.
func escapeGraphQLString(s string) string {
	// First escape backslashes, then escape quotes
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return fmt.Sprintf(`"%s"`, s)
}

// extractFirstEmoji returns the first emoji found in the string and the remaining text.
func extractFirstEmoji(s string) (string, string) {
	emojis := gomoji.CollectAll(s)
	if len(emojis) == 0 {
		return "", s
	}

	first := emojis[0].Character
	// Remove ONLY the first occurrence of the emoji character.
	rest := strings.Replace(s, first, "", 1)
	return first, strings.TrimSpace(rest)
}
