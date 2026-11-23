package slack

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCall(t *testing.T) {
	tests := map[string]struct {
		status     int
		body       string
		call       func(t *testing.T, c *Client) error
		wantErr    bool
		wantAPIErr string
	}{
		"投稿すると ts を返す": {
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
		"ok: false はエラーにする": {
			status: http.StatusOK,
			body:   `{"ok":false,"error":"channel_not_found"}`,
			call: func(t *testing.T, c *Client) error {
				return c.UpdateMessage(t.Context(), Message{Channel: "C1", TS: "1"})
			},
			wantErr:    true,
			wantAPIErr: "channel_not_found",
		},
		"レート制限 (429) はエラーにする": {
			status: http.StatusTooManyRequests,
			body:   ``,
			call: func(t *testing.T, c *Client) error {
				return c.UpdateMessage(t.Context(), Message{Channel: "C1", TS: "1"})
			},
			wantErr: true,
		},
		"壊れた JSON はエラーにする": {
			status:  http.StatusOK,
			body:    `<html>`,
			call:    func(t *testing.T, c *Client) error { return c.AddReaction(t.Context(), "C1", "1", "x") },
			wantErr: true,
		},
		"スタンプが付いている (already_reacted) ときはエラーにしない": {
			status: http.StatusOK,
			body:   `{"ok":false,"error":"already_reacted"}`,
			call:   func(t *testing.T, c *Client) error { return c.AddReaction(t.Context(), "C1", "1", "x") },
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := newTestClient(t, tt.status, tt.body)
			err := tt.call(t, c)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantAPIErr != "" {
				if apiErr, ok := errors.AsType[*APIError](err); !ok || apiErr.Code != tt.wantAPIErr {
					t.Errorf("err = %v, want APIError %q", err, tt.wantAPIErr)
				}
			}
		})
	}
}

// newTestClient は、決めた status と body を返すテストサーバーに向けたクライアントを作る
//   - リクエストに、ボットトークンと JSON の本文が付いていることも確かめる
func newTestClient(t *testing.T, status int, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer xoxb-test" {
			t.Errorf("Authorization = %q, want Bearer xoxb-test", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("request body is not json: %v", err)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewClient("xoxb-test").WithBaseURL(srv.URL + "/")
}
