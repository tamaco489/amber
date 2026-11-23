package slack

import (
	"strings"
	"time"
)

// Slack の通知の色の帯 (Slack のブランドカラー)
//   - ColorNeutral は、デプロイ中の親の投稿に使う。結果 (緑か赤) が出る前であることを、返信の色と分けて示す
const (
	ColorNeutral = "#1D1C1D"
	ColorInfo    = "#36C5F0"
	ColorSuccess = "#2EB67D"
	ColorWarning = "#ECB22E"
	ColorDanger  = "#E01E5A"
)

// 値が無いときに出す文言
const none = "none"

// 時刻は JST で出す
//   - Lambda の provided.al2023 にタイムゾーンのデータを持ち込まずに済むよう、固定の +09:00 を使う
var jst = time.FixedZone("JST", 9*60*60)

// FormatJST は、時刻を 2026-11-23 11:12:30 (JST) の形にする
func FormatJST(t time.Time) string {
	return t.In(jst).Format("2006-01-02 15:04:05") + " (JST)"
}

// mrkdwn の本文に埋め込む値を、Slack の決まりどおりにエスケープする (&、<、> だけ)
//
// DOC: https://docs.slack.dev/messaging/formatting-message-text#escaping
var mrkdwnEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func escape(s string) string {
	if s == "" {
		return none
	}
	return mrkdwnEscaper.Replace(s)
}

func field(name, value string) Text {
	return Text{Type: "mrkdwn", Text: "*" + name + "*\n" + escape(value)}
}

func header(text string) Block {
	return Block{Type: "header", Text: &Text{Type: "plain_text", Text: text, Emoji: true}}
}

func message(channel, threadTS, text, color string, blocks ...Block) Message {
	return Message{
		Channel:     channel,
		ThreadTS:    threadTS,
		Text:        text,
		Attachments: []Attachment{{Color: color, Blocks: blocks}},
	}
}

// shortTaskDefinition は、タスク定義の ARN から <family>:<revision> を取り出す
func shortTaskDefinition(arn string) string {
	_, short, ok := strings.CutLast(arn, "task-definition/")
	if !ok {
		return arn
	}
	return short
}
