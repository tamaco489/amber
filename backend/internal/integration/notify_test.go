package integration_test

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/notifier"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// TestNotify は、生のイベントを順に渡し、Slack に届く親の投稿と返信の本文を確かめる
func TestNotify(t *testing.T) {
	type reply struct {
		header string // 返信の見出し
		color  string // 返信の帯の色
	}

	tests := map[string]struct {
		events      []events.CloudWatchEvent
		wantHeader  string   // 親の今の見出し
		wantColor   string   // 親の今の帯の色
		wantBody    []string // 親の今の本文に含まれる文
		wantReplies []reply  // 返信 (届いた順)
	}{
		"開始、進行中、完了": {
			events:     []events.CloudWatchEvent{evStarted, evInProgress, evCompleted},
			wantHeader: ":white_check_mark: Deployed v3: prd-amber-api",
			wantColor:  slack.ColorSuccess,
			wantBody: []string{
				"*Status*\nCompleted",
				"*Triggered by*\ntamaco489",
				"*Task definition*\nprd-amber-api:7",
				"*Completed at*\n2026-11-23 11:12:30 (JST)",
				"<" + testRunURL + "|GitHub Actions run>",
			},
			wantReplies: []reply{
				{header: ":rocket: Deploy started: v3", color: slack.ColorInfo},
				{header: ":hourglass_flowing_sand: Deployment in progress", color: slack.ColorWarning},
				{header: ":white_check_mark: Deployment completed", color: slack.ColorSuccess},
			},
		},
		"進行中が開始より先に届く": {
			events:     []events.CloudWatchEvent{evInProgress, evStarted},
			wantHeader: ":rocket: Deploying v3: prd-amber-api",
			wantColor:  slack.ColorInfo,
			wantBody:   []string{"*Status*\nDeploying", "*Triggered by*\ntamaco489"},
			wantReplies: []reply{
				{header: ":hourglass_flowing_sand: Deployment in progress", color: slack.ColorWarning},
				{header: ":rocket: Deploy started: v3", color: slack.ColorInfo},
			},
		},
		"GitHub から始めていないデプロイ": {
			events:     []events.CloudWatchEvent{evInProgress},
			wantHeader: ":rocket: Deploying: prd-amber-api",
			wantColor:  slack.ColorInfo,
			wantBody:   []string{"*Triggered by*\nnone", "GitHub Actions run: none"},
			wantReplies: []reply{
				{header: ":hourglass_flowing_sand: Deployment in progress", color: slack.ColorWarning},
			},
		},
		"ロールバック": {
			events:     []events.CloudWatchEvent{evStarted, evInProgress, evRollback},
			wantHeader: ":rewind: Rolling back v3: prd-amber-api",
			wantColor:  slack.ColorDanger,
			wantBody:   []string{"*Status*\nRolling back"},
			wantReplies: []reply{
				{header: ":rocket: Deploy started: v3", color: slack.ColorInfo},
				{header: ":hourglass_flowing_sand: Deployment in progress", color: slack.ColorWarning},
				{header: ":rewind: Rollback started", color: slack.ColorDanger},
			},
		},
		"失敗": {
			events:     []events.CloudWatchEvent{evStarted, evRollback, evFailed},
			wantHeader: ":x: Failed to deploy v3: prd-amber-api",
			wantColor:  slack.ColorDanger,
			wantBody:   []string{"*Status*\nFailed"},
			wantReplies: []reply{
				{header: ":rocket: Deploy started: v3", color: slack.ColorInfo},
				{header: ":rewind: Rollback started", color: slack.ColorDanger},
				{header: ":x: Deployment failed", color: slack.ColorDanger},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fs := &fakeSlack{}
			n := &notifier.Notifier{
				Slack:   fs,
				Store:   newFakeStore(),
				Channel: testChannel,
				Service: testService,
				Now:     func() time.Time { return testTime },
			}

			for _, ev := range tt.events {
				if err := n.Handle(t.Context(), ev); err != nil {
					t.Fatalf("handle %s: %v", ev.ID, err)
				}
			}

			parent := fs.lastParent()
			if got := headerOf(parent); got != tt.wantHeader {
				t.Errorf("parent header = %q, want %q", got, tt.wantHeader)
			}
			if got := colorOf(parent); got != tt.wantColor {
				t.Errorf("parent color = %q, want %q", got, tt.wantColor)
			}
			body := bodyOf(parent)
			for _, want := range tt.wantBody {
				if !strings.Contains(body, want) {
					t.Errorf("parent body does not contain %q:\n%s", want, body)
				}
			}

			replies := fs.replies()
			if len(replies) != len(tt.wantReplies) {
				t.Fatalf("replies = %d, want %d", len(replies), len(tt.wantReplies))
			}
			for i, want := range tt.wantReplies {
				if got := headerOf(replies[i]); got != want.header {
					t.Errorf("reply %d header = %q, want %q", i, got, want.header)
				}
				if got := colorOf(replies[i]); got != want.color {
					t.Errorf("reply %d color = %q, want %q", i, got, want.color)
				}
			}
		})
	}
}
