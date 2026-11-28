package slack

// WithMention は、本文 (text) の先頭で、Slack のユーザーにメンションする。userID が空なら、そのまま返す
//   - メンションは <@ユーザー ID> で書く。GitHub のユーザー名では届かない
//   - text に入れる。text は、色の帯の上に出る本文と、通知のポップアップに使われる
//   - メンションのあとで改行し、見出しを次の行に出す。同じ行に並べると、見出しが読みにくいため
//
// DOC: https://docs.slack.dev/messaging/formatting-message-text#mentioning-users
// DOC: https://docs.slack.dev/messaging/formatting-message-text#line-breaks
func WithMention(msg Message, userID string) Message {
	if userID == "" {
		return msg
	}
	msg.Text = "<@" + userID + ">\n" + msg.Text
	return msg
}
