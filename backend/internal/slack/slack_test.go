package slack

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCall(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		call       func(t *testing.T, c *Client) error
		wantErr    bool
		wantAPIErr string
	}{
		{
			name:   "post message returns ts",
			status: http.StatusOK,
			body:   `{"ok":true,"ts":"1700000000.000100"}`,
			call: func(t *testing.T, c *Client) error {
				ts, err := c.PostMessage(t.Context(), Message{Channel: "C1", Text: "hi"})
				if err == nil && ts != "1700000000.000100" {
					t.Errorf("ts = %q, want 1700000000.000100", ts)
				}
				return err
			},
		},
		{
			name:   "ok false is an error",
			status: http.StatusOK,
			body:   `{"ok":false,"error":"channel_not_found"}`,
			call: func(t *testing.T, c *Client) error {
				return c.UpdateMessage(t.Context(), Message{Channel: "C1", TS: "1"})
			},
			wantErr:    true,
			wantAPIErr: "channel_not_found",
		},
		{
			name:   "rate limited is an error",
			status: http.StatusTooManyRequests,
			body:   ``,
			call: func(t *testing.T, c *Client) error {
				return c.UpdateMessage(t.Context(), Message{Channel: "C1", TS: "1"})
			},
			wantErr: true,
		},
		{
			name:    "broken json is an error",
			status:  http.StatusOK,
			body:    `<html>`,
			call:    func(t *testing.T, c *Client) error { return c.AddReaction(t.Context(), "C1", "1", "x") },
			wantErr: true,
		},
		{
			name:   "already reacted is not an error",
			status: http.StatusOK,
			body:   `{"ok":false,"error":"already_reacted"}`,
			call:   func(t *testing.T, c *Client) error { return c.AddReaction(t.Context(), "C1", "1", "x") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer xoxb-test" {
					t.Errorf("Authorization = %q, want Bearer xoxb-test", got)
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("request body is not json: %v", err)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := NewClient("xoxb-test").WithBaseURL(srv.URL + "/")
			err := tt.call(t, c)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantAPIErr != "" {
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.Code != tt.wantAPIErr {
					t.Errorf("err = %v, want APIError %q", err, tt.wantAPIErr)
				}
			}
		})
	}
}
