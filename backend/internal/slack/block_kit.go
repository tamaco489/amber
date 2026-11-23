package slack

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
