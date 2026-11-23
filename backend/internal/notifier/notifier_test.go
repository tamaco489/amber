package notifier

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/deploy/deploytest"
	"github.com/tamaco489/amber/backend/internal/slack"
	"github.com/tamaco489/amber/backend/internal/slack/slacktest"
)

func TestHandle(t *testing.T) {
	type call struct {
		ev      events.CloudWatchEvent // 渡すイベント
		wantErr bool                   // エラーになるか
	}

	tests := map[string]struct {
		setup func(fs *fakeSlack, st *fakeStore)               // 前もって失敗や印を仕込む
		calls []call                                           // 順に渡すイベント
		check func(t *testing.T, fs *fakeSlack, st *fakeStore) // 結果を確かめる
	}{
		"開始が先に届くと、起動した人とリンク付きの親を作る": {
			calls: []call{{ev: deploytest.EvStarted}, {ev: deploytest.EvInProgress}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				parents := fs.parents()
				if len(parents) != 1 {
					t.Fatalf("parents = %d, want 1", len(parents))
				}
				if got := slacktest.HeaderOf(parents[0]); got != ":rocket: Deploying v3: prd-amber-api" {
					t.Errorf("parent header = %q", got)
				}
				body := slacktest.BodyOf(parents[0])
				for _, want := range []string{"*Triggered by*\ntamaco489", "*Task definition*\nprd-amber-api:7", "<" + deploytest.RunURL + "|GitHub Actions run>", "*Status*\nDeploying"} {
					if !strings.Contains(body, want) {
						t.Errorf("parent body does not contain %q:\n%s", want, body)
					}
				}
				if got := slacktest.ColorOf(parents[0]); got != slack.ColorInfo {
					t.Errorf("parent color = %q, want %q", got, slack.ColorInfo)
				}
				if len(fs.replies()) != 2 {
					t.Errorf("replies = %d, want 2", len(fs.replies()))
				}
				// 進行中は親を書き換えない
				if len(fs.updates) != 0 {
					t.Errorf("updates = %d, want 0", len(fs.updates))
				}
			},
		},
		"進行中が開始より先に届いても、親は 1 つだけで、あとから埋まる": {
			calls: []call{{ev: deploytest.EvInProgress}, {ev: deploytest.EvStarted}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				parents := fs.parents()
				if len(parents) != 1 {
					t.Fatalf("parents = %d, want 1", len(parents))
				}
				first := slacktest.BodyOf(parents[0])
				if !strings.Contains(first, "*Triggered by*\nnone") || !strings.Contains(first, "GitHub Actions run: none") {
					t.Errorf("first parent should have no actor and no link:\n%s", first)
				}
				if got := slacktest.HeaderOf(parents[0]); got != ":rocket: Deploying: prd-amber-api" {
					t.Errorf("first parent header = %q", got)
				}

				if len(fs.updates) != 1 {
					t.Fatalf("updates = %d, want 1", len(fs.updates))
				}
				updated := fs.updates[0]
				if updated.TS == "" {
					t.Errorf("update has no ts")
				}
				if got := slacktest.HeaderOf(updated); got != ":rocket: Deploying v3: prd-amber-api" {
					t.Errorf("updated header = %q", got)
				}
				if !strings.Contains(slacktest.BodyOf(updated), "*Triggered by*\ntamaco489") {
					t.Errorf("updated parent has no actor:\n%s", slacktest.BodyOf(updated))
				}

				// どちらの返信も、同じ親のスレッドに付く
				replies := fs.replies()
				if len(replies) != 2 {
					t.Fatalf("replies = %d, want 2", len(replies))
				}
				for _, r := range replies {
					if r.ThreadTS != updated.TS {
						t.Errorf("reply thread_ts = %q, want %q", r.ThreadTS, updated.TS)
					}
				}
			},
		},
		"同じイベントが 2 回届いても、返信は 1 件": {
			calls: []call{{ev: deploytest.EvInProgress}, {ev: deploytest.EvInProgress}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				if len(fs.parents()) != 1 {
					t.Errorf("parents = %d, want 1", len(fs.parents()))
				}
				if len(fs.replies()) != 1 {
					t.Errorf("replies = %d, want 1", len(fs.replies()))
				}
			},
		},
		"chat.update の失敗のあとに流し直しても、返信は重ならない": {
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"chat.update": 1}
			},
			calls: []call{{ev: deploytest.EvStarted}, {ev: deploytest.EvCompleted, wantErr: true}, {ev: deploytest.EvCompleted}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				if len(fs.replies()) != 2 {
					t.Errorf("replies = %d, want 2 (started, completed)", len(fs.replies()))
				}
				if len(fs.updates) != 1 {
					t.Errorf("updates = %d, want 1", len(fs.updates))
				}
				if len(fs.reactions) != 1 {
					t.Errorf("reactions = %d, want 1", len(fs.reactions))
				}
			},
		},
		"reactions.add の失敗のあとに流し直しても、書き換えと返信は重ならない": {
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"reactions.add": 1}
			},
			calls: []call{{ev: deploytest.EvStarted}, {ev: deploytest.EvFailed, wantErr: true}, {ev: deploytest.EvFailed}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				if len(fs.replies()) != 2 || len(fs.updates) != 1 || len(fs.reactions) != 1 {
					t.Errorf("replies = %d, updates = %d, reactions = %d, want 2, 1, 1", len(fs.replies()), len(fs.updates), len(fs.reactions))
				}
			},
		},
		"親の投稿の失敗のあとに流し直しても、親は 1 つだけ": {
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"chat.postMessage": 1}
			},
			calls: []call{{ev: deploytest.EvInProgress, wantErr: true}, {ev: deploytest.EvInProgress}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				if len(fs.parents()) != 1 || len(fs.replies()) != 1 {
					t.Errorf("parents = %d, replies = %d, want 1, 1", len(fs.parents()), len(fs.replies()))
				}
			},
		},
		"完了で、親が緑になり、チェックのスタンプが付く": {
			calls: []call{{ev: deploytest.EvStarted}, {ev: deploytest.EvInProgress}, {ev: deploytest.EvCompleted}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				p := fs.lastParent()
				if got := slacktest.HeaderOf(p); got != ":white_check_mark: Deployed v3: prd-amber-api" {
					t.Errorf("parent header = %q", got)
				}
				if slacktest.ColorOf(p) != slack.ColorSuccess {
					t.Errorf("parent color = %q, want %q", slacktest.ColorOf(p), slack.ColorSuccess)
				}
				body := slacktest.BodyOf(p)
				if !strings.Contains(body, "*Status*\nCompleted") || !strings.Contains(body, "*Completed at*\n2026-11-23 11:12:30 (JST)") {
					t.Errorf("parent body:\n%s", body)
				}
				if len(fs.reactions) != 1 || fs.reactions[0].name != "white_check_mark" {
					t.Errorf("reactions = %+v, want one white_check_mark", fs.reactions)
				}
				replies := fs.replies()
				if got := slacktest.HeaderOf(replies[len(replies)-1]); got != ":white_check_mark: Deployment completed" {
					t.Errorf("reply header = %q", got)
				}
			},
		},
		"ロールバックで、親が赤になる": {
			calls: []call{{ev: deploytest.EvStarted}, {ev: deploytest.EvInProgress}, {ev: deploytest.EvRollback}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				p := fs.lastParent()
				if slacktest.ColorOf(p) != slack.ColorDanger || !strings.Contains(slacktest.BodyOf(p), "*Status*\nRolling back") {
					t.Errorf("parent color = %q, body:\n%s", slacktest.ColorOf(p), slacktest.BodyOf(p))
				}
				if got := slacktest.HeaderOf(p); got != ":rewind: Rolling back v3: prd-amber-api" {
					t.Errorf("parent header = %q", got)
				}
				replies := fs.replies()
				last := replies[len(replies)-1]
				if slacktest.HeaderOf(last) != ":rewind: Rollback started" || slacktest.ColorOf(last) != slack.ColorDanger {
					t.Errorf("rollback reply header = %q, color = %q", slacktest.HeaderOf(last), slacktest.ColorOf(last))
				}
				if len(fs.reactions) != 0 {
					t.Errorf("reactions = %+v, want none", fs.reactions)
				}
			},
		},
		"失敗で、親が赤になり、x のスタンプが付く": {
			calls: []call{{ev: deploytest.EvStarted}, {ev: deploytest.EvRollback}, {ev: deploytest.EvFailed}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				p := fs.lastParent()
				if slacktest.ColorOf(p) != slack.ColorDanger || !strings.Contains(slacktest.BodyOf(p), "*Status*\nFailed") {
					t.Errorf("parent color = %q, body:\n%s", slacktest.ColorOf(p), slacktest.BodyOf(p))
				}
				if len(fs.reactions) != 1 || fs.reactions[0].name != "x" {
					t.Errorf("reactions = %+v, want one x", fs.reactions)
				}
			},
		},
		"遅れて届いたロールバックで、完了した親が戻らない": {
			calls: []call{{ev: deploytest.EvStarted}, {ev: deploytest.EvCompleted}, {ev: deploytest.EvRollback}},
			check: func(t *testing.T, _ *fakeSlack, st *fakeStore) {
				if got := st.parents[deploytest.Deployment].Status; got != deploy.StatusCompleted {
					t.Errorf("status = %q, want %q", got, deploy.StatusCompleted)
				}
			},
		},
		"Slack の ok: false はエラーとして返す": {
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"chat.postMessage": 1}
				fs.failWith = &slack.APIError{Method: "chat.postMessage", Code: "not_in_channel"}
			},
			calls: []call{{ev: deploytest.EvStarted, wantErr: true}},
			check: func(t *testing.T, fs *fakeSlack, st *fakeStore) {
				if len(fs.posts) != 0 || len(st.parents) != 0 {
					t.Errorf("posts = %d, parents = %d, want 0, 0", len(fs.posts), len(st.parents))
				}
			},
		},
		"ほかの呼び出しが進めている手順は、投稿せずにエラーを返す": {
			setup: func(_ *fakeSlack, st *fakeStore) {
				st.steps["ev-started#reply"] = &fakeStep{leaseUntil: deploytest.Time.Add(time.Minute)}
			},
			calls: []call{{ev: deploytest.EvStarted, wantErr: true}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				if len(fs.replies()) != 0 {
					t.Errorf("replies = %d, want 0", len(fs.replies()))
				}
			},
		},
		"通知の対象でないイベントは飛ばす": {
			calls: []call{{ev: events.CloudWatchEvent{ID: "x", Source: "aws.ecs", DetailType: "ECS Task State Change", Detail: []byte(`{}`)}}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				if len(fs.posts) != 0 {
					t.Errorf("posts = %d, want 0", len(fs.posts))
				}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fs := &fakeSlack{}
			st := newFakeStore()
			if tt.setup != nil {
				tt.setup(fs, st)
			}
			n := &Notifier{
				Slack:   fs,
				Store:   st,
				Channel: deploytest.Channel,
				Service: deploytest.Service,
				Now:     func() time.Time { return deploytest.Time },
			}

			for i, c := range tt.calls {
				err := n.Handle(t.Context(), c.ev)
				if (err != nil) != c.wantErr {
					t.Fatalf("call %d (%s): err = %v, wantErr %v", i, c.ev.ID, err, c.wantErr)
				}
			}
			tt.check(t, fs, st)
		})
	}
}

func TestHandleBusyIsErrBusy(t *testing.T) {
	st := newFakeStore()
	st.steps["parent#"+deploytest.Deployment] = &fakeStep{leaseUntil: deploytest.Time.Add(time.Minute)}
	n := &Notifier{Slack: &fakeSlack{}, Store: st, Channel: deploytest.Channel, Service: deploytest.Service, Now: func() time.Time { return deploytest.Time }}

	if err := n.Handle(t.Context(), deploytest.EvInProgress); !errors.Is(err, ErrBusy) {
		t.Errorf("err = %v, want ErrBusy", err)
	}
}
