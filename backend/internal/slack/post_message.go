package slack

import "context"

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
