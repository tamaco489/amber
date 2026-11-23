package slack

import "fmt"

// APIError は、Slack が ok: false で返したエラー
type APIError struct {
	Method string // 呼んだ Web API (chat.postMessage など)
	Code   string // Slack が返した error の値 (channel_not_found など)
}

func (e *APIError) Error() string {
	return fmt.Sprintf("slack %s: %s", e.Method, e.Code)
}
