package integration_test

import (
	"strings"
	"testing"

	"github.com/tamaco489/amber/backend/internal/slack"
)

// headerOf は、タイトル (text の太字の中身)
func headerOf(m slack.Message) string {
	return strings.Trim(m.Text, "*")
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

// assertTexts は、投稿の text を、順に確かめる
func assertTexts(t *testing.T, kind string, msgs []slack.Message, want []string) {
	t.Helper()
	if len(msgs) != len(want) {
		t.Fatalf("%ss = %d, want %d", kind, len(msgs), len(want))
	}
	for i, w := range want {
		if got := msgs[i].Text; got != w {
			t.Errorf("%s %d text = %q, want %q", kind, i, got, w)
		}
	}
}
