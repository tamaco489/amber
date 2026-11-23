package notifier

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// TestHandle は、処理の流れ (親の数、返信の数とスレッド、書き換え、スタンプ、再試行での重複) を確かめる
//   - Slack に届く本文の中身 (見出し、色、項目) は、slack の単体テストと integration の結合テストで確かめる
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
		"開始が先に届くと、起動した人の入った親を作る": {
			calls: []call{{ev: evStarted}, {ev: evInProgress}},
			check: func(t *testing.T, fs *fakeSlack, st *fakeStore) {
				assertCounts(t, fs, 1, 2, 0, 0)
				p := st.parents[testDeployment]
				if p.Actor != "tamaco489" || p.Status != deploy.StatusDeploying {
					t.Errorf("parent = %+v", p)
				}
			},
		},
		"進行中が開始より先に届いても、親は 1 つだけで、あとから埋まる": {
			calls: []call{{ev: evInProgress}, {ev: evStarted}},
			check: func(t *testing.T, fs *fakeSlack, st *fakeStore) {
				assertCounts(t, fs, 1, 2, 1, 0)
				parentTS := st.parents[testDeployment].TS
				for _, r := range fs.replies() {
					if r.ThreadTS != parentTS {
						t.Errorf("reply thread_ts = %q, want %q", r.ThreadTS, parentTS)
					}
				}
				if fs.updates[0].TS != parentTS {
					t.Errorf("update ts = %q, want %q", fs.updates[0].TS, parentTS)
				}
				if got := st.parents[testDeployment].Actor; got != "tamaco489" {
					t.Errorf("actor = %q, want tamaco489", got)
				}
			},
		},
		"同じイベントが 2 回届いても、返信は 1 件": {
			calls: []call{{ev: evInProgress}, {ev: evInProgress}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				assertCounts(t, fs, 1, 1, 0, 0)
			},
		},
		"chat.update の失敗のあとに流し直しても、返信は重ならない": {
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"chat.update": 1}
			},
			calls: []call{{ev: evStarted}, {ev: evCompleted, wantErr: true}, {ev: evCompleted}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				assertCounts(t, fs, 1, 2, 1, 1)
			},
		},
		"reactions.add の失敗のあとに流し直しても、書き換えと返信は重ならない": {
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"reactions.add": 1}
			},
			calls: []call{{ev: evStarted}, {ev: evFailed, wantErr: true}, {ev: evFailed}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				assertCounts(t, fs, 1, 2, 1, 1)
			},
		},
		"親の投稿の失敗のあとに流し直しても、親は 1 つだけ": {
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"chat.postMessage": 1}
			},
			calls: []call{{ev: evInProgress, wantErr: true}, {ev: evInProgress}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				assertCounts(t, fs, 1, 1, 0, 0)
			},
		},
		"完了で、親を完了にし、親にチェックのスタンプを付ける": {
			calls: []call{{ev: evStarted}, {ev: evInProgress}, {ev: evCompleted}},
			check: func(t *testing.T, fs *fakeSlack, st *fakeStore) {
				assertCounts(t, fs, 1, 3, 1, 1)
				p := st.parents[testDeployment]
				if p.Status != deploy.StatusCompleted || !p.CompletedAt.Equal(testTime) {
					t.Errorf("parent = %+v", p)
				}
				if r := fs.reactions[0]; r.name != "white_check_mark" || r.ts != p.TS {
					t.Errorf("reaction = %+v, want white_check_mark on %s", r, p.TS)
				}
			},
		},
		"ロールバックで、親をロールバック中にし、スタンプは付けない": {
			calls: []call{{ev: evStarted}, {ev: evInProgress}, {ev: evRollback}},
			check: func(t *testing.T, fs *fakeSlack, st *fakeStore) {
				assertCounts(t, fs, 1, 3, 1, 0)
				assertStatus(t, st, deploy.StatusRollingBack)
			},
		},
		"失敗で、親を失敗にし、x のスタンプを付ける": {
			calls: []call{{ev: evStarted}, {ev: evRollback}, {ev: evFailed}},
			check: func(t *testing.T, fs *fakeSlack, st *fakeStore) {
				assertCounts(t, fs, 1, 3, 2, 1)
				assertStatus(t, st, deploy.StatusFailed)
				if fs.reactions[0].name != "x" {
					t.Errorf("reaction = %+v, want x", fs.reactions[0])
				}
			},
		},
		"遅れて届いたロールバックで、完了した親が戻らない": {
			calls: []call{{ev: evStarted}, {ev: evCompleted}, {ev: evRollback}},
			check: func(t *testing.T, _ *fakeSlack, st *fakeStore) {
				assertStatus(t, st, deploy.StatusCompleted)
			},
		},
		"Slack の ok: false はエラーとして返す": {
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"chat.postMessage": 1}
				fs.failWith = &slack.APIError{Method: "chat.postMessage", Code: "not_in_channel"}
			},
			calls: []call{{ev: evStarted, wantErr: true}},
			check: func(t *testing.T, fs *fakeSlack, st *fakeStore) {
				if len(fs.posts) != 0 || len(st.parents) != 0 {
					t.Errorf("posts = %d, parents = %d, want 0, 0", len(fs.posts), len(st.parents))
				}
			},
		},
		"ほかの呼び出しが進めている手順は、投稿せずにエラーを返す": {
			setup: func(_ *fakeSlack, st *fakeStore) {
				st.steps["ev-started#reply"] = &fakeStep{leaseUntil: testTime.Add(time.Minute)}
			},
			calls: []call{{ev: evStarted, wantErr: true}},
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
			n := newTestNotifier(fs, st, &fakeECS{})

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
	st.steps["parent#"+testDeployment] = &fakeStep{leaseUntil: testTime.Add(time.Minute)}
	n := newTestNotifier(&fakeSlack{}, st, &fakeECS{})

	if err := n.Handle(t.Context(), evInProgress); !errors.Is(err, ErrBusy) {
		t.Errorf("err = %v, want ErrBusy", err)
	}
}

// assertCounts は、親の投稿、返信、書き換え、スタンプの数を確かめる
func assertCounts(t *testing.T, fs *fakeSlack, parents, replies, updates, reactions int) {
	t.Helper()
	got := []int{len(fs.parents()), len(fs.replies()), len(fs.updates), len(fs.reactions)}
	want := []int{parents, replies, updates, reactions}
	for i, name := range []string{"parents", "replies", "updates", "reactions"} {
		if got[i] != want[i] {
			t.Errorf("%s = %d, want %d", name, got[i], want[i])
		}
	}
}

// newTestNotifier は、差し替えを使い、時刻を testTime に止めた Notifier を作る
func newTestNotifier(fs *fakeSlack, st *fakeStore, fe *fakeECS) *Notifier {
	return &Notifier{
		Slack:   fs,
		Store:   st,
		ECS:     fe,
		Channel: testChannel,
		Service: testService,
		Now:     func() time.Time { return testTime },
	}
}

// assertStatus は、残っている親の状態を確かめる
func assertStatus(t *testing.T, st *fakeStore, want deploy.Status) {
	t.Helper()
	if got := st.parents[testDeployment].Status; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
}
