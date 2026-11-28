package slack

// WithMention は、本文 (text) の先頭で、Slack のユーザーにメンションする。userID が空なら、そのまま返す
//   - メンションは <@ユーザー ID> で書く。GitHub のユーザー名では届かない
//   - text に入れる。text は、色の帯の上に出る本文と、通知のポップアップに使われる
//
// DOC: https://docs.slack.dev/messaging/formatting-message-text#mentioning-users
func WithMention(msg Message, userID string) Message {
	if userID == "" {
		return msg
	}
	msg.Text = "<@" + userID + "> " + msg.Text
	return msg
}
