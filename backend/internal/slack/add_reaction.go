package slack

import (
	"context"
	"errors"
)

// AddReaction は、メッセージにスタンプを付ける
//   - すでに付いている (already_reacted) ときは、成功として扱う (再試行で同じスタンプを付けようとしても失敗にしない)
//
// DOC: https://docs.slack.dev/reference/methods/reactions.add
func (c *Client) AddReaction(ctx context.Context, channel, ts, name string) error {
	req := map[string]string{"channel": channel, "timestamp": ts, "name": name}
	err := c.call(ctx, "reactions.add", req, nil)

	if apiErr, ok := errors.AsType[*APIError](err); ok && apiErr.Code == "already_reacted" {
		return nil
	}
	return err
}
