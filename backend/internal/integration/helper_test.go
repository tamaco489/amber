package integration_test

import (
	"strings"

	"github.com/tamaco489/amber/backend/internal/slack"
)

// headerOf は、先頭のブロック (見出し) の文
func headerOf(m slack.Message) string {
	if len(m.Attachments) == 0 || len(m.Attachments[0].Blocks) == 0 || m.Attachments[0].Blocks[0].Text == nil {
		return ""
	}
	return m.Attachments[0].Blocks[0].Text.Text
}

// colorOf は、帯の色
func colorOf(m slack.Message) string {
	if len(m.Attachments) == 0 {
		return ""
	}
	return m.Attachments[0].Color
}

// bodyOf は、ブロックの mrkdwn の文をつなげたもの (項目の値を確かめるため)
func bodyOf(m slack.Message) string {
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
		writeLine(b, blk.Text.Text)
	}
	for _, f := range blk.Fields {
		writeLine(b, f.Text)
	}
	for _, e := range blk.Elements {
		writeLine(b, e.Text)
	}
}

func writeLine(b *strings.Builder, s string) {
	b.WriteString(s)
	b.WriteByte('\n')
}
