package slack

import "context"

// UpdateMessage は、Channel と TS で指したメッセージを、msg の内容に書き換える
//
// DOC: https://docs.slack.dev/reference/methods/chat.update
func (c *Client) UpdateMessage(ctx context.Context, msg Message) error {
	return c.call(ctx, "chat.update", msg, nil)
}
