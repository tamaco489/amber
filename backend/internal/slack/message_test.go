package slack_test

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/deploy/deploytest"
	"github.com/tamaco489/amber/backend/internal/slack"
	"github.com/tamaco489/amber/backend/internal/slack/slacktest"
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
		raw        events.CloudWatchEvent
		wantHeader string
		wantColor  string
		wantBody   []string
		notInBody  []string
	}{
		"開始": {
			raw:        deploytest.Started("1", true),
			wantHeader: ":rocket: Deploy started: v3",
			wantColor:  slack.ColorInfo,
			wantBody: []string{
				"*Triggered by*\ntamaco489",
				"*Deployment ID*\n" + deploytest.Deployment,
				"*Task definition*\n" + deploytest.TaskDef,
				"*Run*\n" + deploytest.RunURL,
			},
			notInBody: []string{"Pushed"},
		},
		"同じタグの流し直し": {
			raw:        deploytest.Started("1", false),
			wantHeader: ":rocket: Redeploy started: v3 (existing image)",
			wantColor:  slack.ColorInfo,
		},
		"進行中": {
			raw:        deploytest.EvInProgress,
			wantHeader: ":hourglass_flowing_sand: Deployment in progress",
			wantColor:  slack.ColorWarning,
			wantBody:   []string{"*Service*\nprd-amber-api", "*Reason*\nECS deployment", "2026-11-23 11:12:30 (JST)"},
		},
		"ロールバック": {
			raw:        deploytest.EvRollback,
			wantHeader: ":rewind: Rollback started",
			wantColor:  slack.ColorDanger,
		},
		"完了": {
			raw:        deploytest.EvCompleted,
			wantHeader: ":white_check_mark: Deployment completed",
			wantColor:  slack.ColorSuccess,
		},
		"失敗": {
			raw:        deploytest.EvFailed,
			wantHeader: ":x: Deployment failed",
			wantColor:  slack.ColorDanger,
			wantBody:   []string{"task failed to start."},
		},
		"reason をエスケープする": {
			raw:      deploytest.ECS("1", deploy.ECSFailed, "a <b> & c"),
			wantBody: []string{"a &lt;b&gt; &amp; c"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			msg := slack.ReplyMessage(deploytest.Parse(tt.raw), deploytest.Channel, deploytest.ParentTS)

			if msg.ThreadTS != deploytest.ParentTS || msg.Channel != deploytest.Channel {
				t.Errorf("thread_ts = %q, channel = %q", msg.ThreadTS, msg.Channel)
			}
			if tt.wantHeader != "" && slacktest.HeaderOf(msg) != tt.wantHeader {
				t.Errorf("header = %q, want %q", slacktest.HeaderOf(msg), tt.wantHeader)
			}
			if tt.wantColor != "" && slacktest.ColorOf(msg) != tt.wantColor {
				t.Errorf("color = %q, want %q", slacktest.ColorOf(msg), tt.wantColor)
			}
			assertBody(t, slacktest.BodyOf(msg), tt.wantBody, tt.notInBody)
		})
	}
}

func TestParentMessage(t *testing.T) {
	base := deploy.Parent{
		DeploymentID:   deploytest.Deployment,
		TS:             deploytest.ParentTS,
		Channel:        deploytest.Channel,
		Service:        deploytest.Service,
		Version:        "v3",
		Actor:          "tamaco489",
		RunURL:         deploytest.RunURL,
		TaskDefinition: deploytest.TaskDef,
	}

	tests := map[string]struct {
		parent     func() deploy.Parent
		wantHeader string
		wantColor  string
		wantBody   []string
	}{
		"デプロイ中": {
			parent:     func() deploy.Parent { p := base; p.Status = deploy.StatusDeploying; return p },
			wantHeader: ":rocket: Deploying v3: prd-amber-api",
			wantColor:  slack.ColorInfo,
			wantBody: []string{
				"*Status*\nDeploying",
				"*Triggered by*\ntamaco489",
				"*Task definition*\nprd-amber-api:7",
				"<" + deploytest.RunURL + "|GitHub Actions run>",
			},
		},
		"バージョンがまだ分からない": {
			parent: func() deploy.Parent {
				return deploy.Parent{Service: deploytest.Service, Status: deploy.StatusDeploying}
			},
			wantHeader: ":rocket: Deploying: prd-amber-api",
			wantBody:   []string{"*Triggered by*\nnone", "GitHub Actions run: none"},
		},
		"完了": {
			parent: func() deploy.Parent {
				p := base
				p.Status = deploy.StatusCompleted
				p.CompletedAt = deploytest.Time
				return p
			},
			wantHeader: ":white_check_mark: Deployed v3: prd-amber-api",
			wantColor:  slack.ColorSuccess,
			wantBody:   []string{"*Status*\nCompleted", "*Completed at*\n2026-11-23 11:12:30 (JST)"},
		},
		"ロールバック中": {
			parent:     func() deploy.Parent { p := base; p.Status = deploy.StatusRollingBack; return p },
			wantHeader: ":rewind: Rolling back v3: prd-amber-api",
			wantColor:  slack.ColorDanger,
			wantBody:   []string{"*Status*\nRolling back"},
		},
		"失敗": {
			parent:     func() deploy.Parent { p := base; p.Status = deploy.StatusFailed; return p },
			wantHeader: ":x: Failed to deploy v3: prd-amber-api",
			wantColor:  slack.ColorDanger,
			wantBody:   []string{"*Status*\nFailed"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p := tt.parent()
			msg := slack.ParentMessage(&p)

			if got := slacktest.HeaderOf(msg); got != tt.wantHeader {
				t.Errorf("header = %q, want %q", got, tt.wantHeader)
			}
			if tt.wantColor != "" && slacktest.ColorOf(msg) != tt.wantColor {
				t.Errorf("color = %q, want %q", slacktest.ColorOf(msg), tt.wantColor)
			}
			assertBody(t, slacktest.BodyOf(msg), tt.wantBody, nil)
		})
	}
}

func assertBody(t *testing.T, body string, want, unwanted []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("body does not contain %q:\n%s", w, body)
		}
	}
	for _, u := range unwanted {
		if strings.Contains(body, u) {
			t.Errorf("body contains %q:\n%s", u, body)
		}
	}
}
