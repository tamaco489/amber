// Package slack は、Slack の Web API のうち、デプロイの通知に使うものだけを net/http で呼ぶ
//   - chat.postMessage: 親の投稿と、スレッドへの返信
//   - chat.update: 親の投稿の書き換え
//   - reactions.add: 親の投稿へのスタンプ
//
// HTTP のエラー (429 を含む) と、HTTP 200 で返る ok: false は、どちらもエラーとして返す。
// 呼び出し元 (Lambda) がエラーを返すと、非同期呼び出しの再試行と失敗時の送り先に乗る。
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// Message は、chat.postMessage と chat.update に渡す本文
//   - 色の帯を付けるため、Block Kit のブロックは attachments に入れる
//   - Text は、通知のプレビューに出る文
//
// DOC: https://docs.slack.dev/reference/methods/chat.postMessage
// DOC: https://docs.slack.dev/reference/block-kit/blocks
type Message struct {
	Channel     string       `json:"channel"`
	TS          string       `json:"ts,omitempty"`
	ThreadTS    string       `json:"thread_ts,omitempty"`
	Text        string       `json:"text"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment は、色の帯と、その中に並べるブロック
type Attachment struct {
	Color  string  `json:"color"`
	Blocks []Block `json:"blocks"`
}

// Block は、Block Kit のブロック (header、section、context だけを使う)
type Block struct {
	Type     string `json:"type"`
	Text     *Text  `json:"text,omitempty"`
	Fields   []Text `json:"fields,omitempty"`
	Elements []Text `json:"elements,omitempty"`
}

// Text は、Block Kit のテキスト (plain_text か mrkdwn)
type Text struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// APIError は、Slack が ok: false で返したエラー
type APIError struct {
	Method string
	Code   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("slack %s: %s", e.Method, e.Code)
}

// PostMessage は、メッセージを投稿し、その ts を返す
//   - ThreadTS を入れると、そのメッセージのスレッドに返信する
func (c *Client) PostMessage(ctx context.Context, msg Message) (string, error) {
	var res struct {
		TS string `json:"ts"`
	}
	if err := c.call(ctx, "chat.postMessage", msg, &res); err != nil {
		return "", err
	}
	return res.TS, nil
}

// UpdateMessage は、Channel と TS で指したメッセージを、msg の内容に書き換える
//
// DOC: https://docs.slack.dev/reference/methods/chat.update
func (c *Client) UpdateMessage(ctx context.Context, msg Message) error {
	return c.call(ctx, "chat.update", msg, nil)
}

// AddReaction は、メッセージにスタンプを付ける
//   - すでに付いている (already_reacted) ときは、成功として扱う (再試行で同じスタンプを付けようとしても失敗にしない)
//
// DOC: https://docs.slack.dev/reference/methods/reactions.add
func (c *Client) AddReaction(ctx context.Context, channel, ts, name string) error {
	req := map[string]string{"channel": channel, "timestamp": ts, "name": name}
	err := c.call(ctx, "reactions.add", req, nil)

	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == "already_reacted" {
		return nil
	}
	return err
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
