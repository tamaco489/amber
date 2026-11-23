package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultBaseURL は、Slack の Web API の URL
const DefaultBaseURL = "https://slack.com/api/"

// 1 回の呼び出しを待つ上限
//   - Lambda のタイムアウトより十分に短くし、待ち切れずに関数ごと止まらないようにする
const requestTimeout = 10 * time.Second

// Client は、ボットトークンで Slack の Web API を呼ぶ
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewClient は、ボットトークン (xoxb-...) を使うクライアントを作る
func NewClient(token string) *Client {
	return &Client{
		token:      token,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

// WithBaseURL は、呼び出し先を差し替えたクライアントを返す (テスト用)
func (c *Client) WithBaseURL(baseURL string) *Client {
	cc := *c
	cc.baseURL = baseURL
	return &cc
}

// call は、Web API を JSON で呼び、ok: false をエラーにする
//
// DOC: https://docs.slack.dev/apis/web-api/
// DOC: https://docs.slack.dev/apis/web-api/rate-limits
func (c *Client) call(ctx context.Context, method string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("slack %s: encode request: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+method, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("slack %s: build request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("slack %s: read response: %w", method, err)
	}

	// 429 (レート制限) も、ここでエラーにする。待ち直しは Lambda の再試行に任せる
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack %s: http %d (retry-after: %q)", method, resp.StatusCode, resp.Header.Get("Retry-After"))
	}

	var envelope struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("slack %s: decode response: %w", method, err)
	}
	if !envelope.OK {
		return &APIError{Method: method, Code: envelope.Error}
	}

	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("slack %s: decode response: %w", method, err)
		}
	}
	return nil
}
