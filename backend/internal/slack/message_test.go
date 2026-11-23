package slack_test

import (
	"strings"
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
				"*Task definition*\n" + testTaskDef,
				"*Run*\n" + testRunURL,
			},
			notInBody: []string{"Pushed"},
		},
		"同じタグの流し直し": {
			ev:         startedEvent(false),
			wantHeader: ":rocket: Redeploy started: v3 (existing image)",
			wantColor:  slack.ColorInfo,
		},
		"進行中": {
			ev:         ecsEvent(deploy.KindInProgress, "ECS deployment ecs-svc/1234567890 in progress."),
			wantHeader: ":hourglass_flowing_sand: Deployment in progress",
			wantColor:  slack.ColorWarning,
			wantBody:   []string{"*Service*\nprd-amber-api", "*Reason*\nECS deployment", "2026-11-23 11:12:30 (JST)"},
		},
		"ロールバック": {
			ev:         ecsEvent(deploy.KindRollback, "ECS deployment circuit breaker: rolling back to deploymentId ecs-svc/111."),
			wantHeader: ":rewind: Rollback started",
			wantColor:  slack.ColorDanger,
		},
		"完了": {
			ev:         ecsEvent(deploy.KindCompleted, "ECS deployment ecs-svc/1234567890 completed."),
			wantHeader: ":white_check_mark: Deployment completed",
			wantColor:  slack.ColorSuccess,
		},
		"失敗": {
			ev:         ecsEvent(deploy.KindFailed, "ECS deployment circuit breaker: task failed to start."),
			wantHeader: ":x: Deployment failed",
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
			if tt.wantHeader != "" && headerOf(msg) != tt.wantHeader {
				t.Errorf("header = %q, want %q", headerOf(msg), tt.wantHeader)
			}
			if tt.wantColor != "" && colorOf(msg) != tt.wantColor {
				t.Errorf("color = %q, want %q", colorOf(msg), tt.wantColor)
			}
			assertBody(t, bodyOf(msg), tt.wantBody, tt.notInBody)
		})
	}
}

func TestParentMessage(t *testing.T) {
	base := deploy.Parent{
		DeploymentID:   testDeployment,
		TS:             testParentTS,
		Channel:        testChannel,
		Service:        testService,
		Version:        "v3",
		Actor:          "tamaco489",
		RunURL:         testRunURL,
		TaskDefinition: testTaskDef,
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
				"<" + testRunURL + "|GitHub Actions run>",
			},
		},
		"バージョンがまだ分からない": {
			parent: func() deploy.Parent {
				return deploy.Parent{Service: testService, Status: deploy.StatusDeploying}
			},
			wantHeader: ":rocket: Deploying: prd-amber-api",
			wantBody:   []string{"*Triggered by*\nnone", "GitHub Actions run: none"},
		},
		"完了": {
			parent: func() deploy.Parent {
				p := base
				p.Status = deploy.StatusCompleted
				p.CompletedAt = testTime
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

			if got := headerOf(msg); got != tt.wantHeader {
				t.Errorf("header = %q, want %q", got, tt.wantHeader)
			}
			if tt.wantColor != "" && colorOf(msg) != tt.wantColor {
				t.Errorf("color = %q, want %q", colorOf(msg), tt.wantColor)
			}
			assertBody(t, bodyOf(msg), tt.wantBody, nil)
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
