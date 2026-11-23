// Package slacktest は、テストで Slack のメッセージの中身を取り出す
package slacktest

import (
	"strings"

	"github.com/tamaco489/amber/backend/internal/slack"
)

// HeaderOf は、先頭のブロック (見出し) の文
func HeaderOf(m slack.Message) string {
	if len(m.Attachments) == 0 || len(m.Attachments[0].Blocks) == 0 || m.Attachments[0].Blocks[0].Text == nil {
		return ""
	}
	return m.Attachments[0].Blocks[0].Text.Text
}

// ColorOf は、帯の色
func ColorOf(m slack.Message) string {
	if len(m.Attachments) == 0 {
		return ""
	}
	return m.Attachments[0].Color
}

// BodyOf は、ブロックの mrkdwn の文をつなげたもの (項目の値を確かめるため)
func BodyOf(m slack.Message) string {
	var b strings.Builder
	for _, a := range m.Attachments {
		for _, blk := range a.Blocks {
			writeBlock(&b, blk)
		}
	}
	return b.String()
}

func writeBlock(b *strings.Builder, blk slack.Block) {
	if blk.Text != nil {
		b.WriteString(blk.Text.Text + "\n")
	}
	for _, f := range blk.Fields {
		b.WriteString(f.Text + "\n")
	}
	for _, e := range blk.Elements {
		b.WriteString(e.Text + "\n")
	}
}
