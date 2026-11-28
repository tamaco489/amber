package slack_test

import (
	"testing"

	"github.com/tamaco489/amber/backend/internal/slack"
)

func TestWithMention(t *testing.T) {
	msg := slack.Message{Channel: testChannel, Text: "*:rocket: Deploying v3: prd-amber-api*"}

	tests := map[string]struct {
		userID string
		want   string
	}{
		"本文の先頭でメンションする": {
			userID: "U0123ABCD",
			want:   "<@U0123ABCD> *:rocket: Deploying v3: prd-amber-api*",
		},
		"ユーザー ID が空なら、本文を変えない": {
			want: "*:rocket: Deploying v3: prd-amber-api*",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := slack.WithMention(msg, tt.userID)
			if got.Text != tt.want {
				t.Errorf("text = %q, want %q", got.Text, tt.want)
			}
			if got.Channel != msg.Channel {
				t.Errorf("channel = %q, want %q", got.Channel, msg.Channel)
			}
		})
	}
}
