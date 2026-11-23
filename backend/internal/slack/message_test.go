package slack_test

import (
	"testing"
	"time"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/slack"
)

func TestFormatJST(t *testing.T) {
	tests := map[string]struct {
		in   time.Time
		want string
	}{
		"UTC を JST に直す": {
			in:   time.Date(2026, 11, 23, 2, 12, 30, 0, time.UTC),
			want: "2026-11-23 11:12:30 (JST)",
		},
		"日付をまたぐ": {
			in:   time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC),
			want: "2026-11-24 00:00:00 (JST)",
		},
		"ほかのタイムゾーンを JST に直す": {
			in:   time.Date(2026, 11, 23, 0, 0, 0, 0, time.FixedZone("X", -5*60*60)),
			want: "2026-11-23 14:00:00 (JST)",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := slack.FormatJST(tt.in); got != tt.want {
				t.Errorf("FormatJST = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReplyMessage(t *testing.T) {
	tests := map[string]struct {
		ev         *deploy.Event
		wantHeader string
		wantColor  string
		wantBody   []string
		notInBody  []string
	}{
		"開始": {
			ev:         startedEvent(true),
			wantHeader: ":rocket: Deploy started: v3",
			wantColor:  slack.ColorInfo,
			wantBody: []string{
				"*Triggered by*\ntamaco489",
				"*Deployment ID*\n" + testDeployment,
				"*Task definition*\nprd-amber-api:7",
				"*Run*\n" + testRunURL,
			},
			notInBody: []string{"Pushed", "arn:aws:ecs"},
		},
		"同じタグの流し直し": {
			ev:         startedEvent(false),
			wantHeader: ":rocket: Redeploy started: v3 (existing image)",
			wantColor:  slack.ColorInfo,
		},
		"進行中": {
			ev:         ecsEvent(deploy.KindInProgress, "ECS deployment ecs-svc/1234567890 in progress."),
			wantHeader: ":hourglass_flowing_sand: Deployment in progress: prd-amber-api",
			wantColor:  slack.ColorWarning,
			wantBody:   []string{"*Service*\nprd-amber-api", "*Reason*\nECS deployment", "2026-11-23 11:12:30 (JST)"},
		},
		"ロールバック": {
			ev:         ecsEvent(deploy.KindRollback, "ECS deployment circuit breaker: rolling back to deploymentId ecs-svc/111."),
			wantHeader: ":rewind: Rollback started: prd-amber-api",
			wantColor:  slack.ColorDanger,
		},
		"完了": {
			ev:         ecsEvent(deploy.KindCompleted, "ECS deployment ecs-svc/1234567890 completed."),
			wantHeader: ":white_check_mark: Deployment completed: prd-amber-api",
			wantColor:  slack.ColorSuccess,
		},
		"失敗": {
			ev:         ecsEvent(deploy.KindFailed, "ECS deployment circuit breaker: task failed to start."),
			wantHeader: ":x: Deployment failed: prd-amber-api",
			wantColor:  slack.ColorDanger,
			wantBody:   []string{"task failed to start."},
		},
		"reason をエスケープする": {
			ev:       ecsEvent(deploy.KindFailed, "a <b> & c"),
			wantBody: []string{"a &lt;b&gt; &amp; c"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			msg := slack.ReplyMessage(tt.ev, testChannel, testParentTS)

			if msg.ThreadTS != testParentTS || msg.Channel != testChannel {
				t.Errorf("thread_ts = %q, channel = %q", msg.ThreadTS, msg.Channel)
			}
			assertMessage(t, msg, tt.wantHeader, tt.wantColor, tt.wantBody, tt.notInBody)
		})
	}
}

func TestParentMessage(t *testing.T) {
	completed := parentWith(deploy.StatusCompleted)
	completed.CompletedAt = testTime

	tests := map[string]struct {
		parent     *deploy.Parent
		wantHeader string
		wantColor  string
		wantBody   []string
	}{
		"デプロイ中": {
			parent:     parentWith(deploy.StatusDeploying),
			wantHeader: ":rocket: Deploying v3: prd-amber-api",
			wantColor:  slack.ColorNeutral,
			wantBody: []string{
				"*Status*\nDeploying",
				"*Triggered by*\ntamaco489",
				"*Task definition*\nprd-amber-api:7",
				"<" + testECSURL + "|ECS deployment>   <" + testRunURL + "|GitHub Actions run>",
			},
		},
		"バージョンがまだ分からない": {
			parent:     &deploy.Parent{Service: testService, Status: deploy.StatusDeploying},
			wantHeader: ":rocket: Deploying: prd-amber-api",
			wantBody:   []string{"*Triggered by*\nnone", "ECS deployment: none   GitHub Actions run: none"},
		},
		"GitHub から始めていないデプロイでも、ECS のリンクを先頭に付ける": {
			parent:   &deploy.Parent{Service: testService, Status: deploy.StatusDeploying, ECSDeploymentURL: testECSURL},
			wantBody: []string{"<" + testECSURL + "|ECS deployment>   GitHub Actions run: none"},
		},
		"完了": {
			parent:     completed,
			wantHeader: ":white_check_mark: Deployed v3: prd-amber-api",
			wantColor:  slack.ColorSuccess,
			wantBody:   []string{"*Status*\nCompleted", "*Completed at*\n2026-11-23 11:12:30 (JST)"},
		},
		"ロールバック中": {
			parent:     parentWith(deploy.StatusRollingBack),
			wantHeader: ":rewind: Rolling back v3: prd-amber-api",
			wantColor:  slack.ColorDanger,
			wantBody:   []string{"*Status*\nRolling back"},
		},
		"失敗": {
			parent:     parentWith(deploy.StatusFailed),
			wantHeader: ":x: Failed to deploy v3: prd-amber-api",
			wantColor:  slack.ColorDanger,
			wantBody:   []string{"*Status*\nFailed"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			msg := slack.ParentMessage(tt.parent)

			assertMessage(t, msg, tt.wantHeader, tt.wantColor, tt.wantBody, nil)
		})
	}
}
