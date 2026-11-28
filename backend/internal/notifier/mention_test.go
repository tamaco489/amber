package notifier

import (
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// テストで使う、起動した人 (tamaco489) の Slack のユーザー ID
const testSlackUserID = "U0TEST"

// TestHandleMention は、起動した人にメンションする投稿 (親か開始の返信のどちらか 1 回だけ) を確かめる
func TestHandleMention(t *testing.T) {
	type call struct {
		ev       events.CloudWatchEvent // 渡すイベント
		failPost bool                   // このイベントの最初の chat.postMessage を失敗させるか
		wantErr  bool                   // エラーになるか
	}

	tests := map[string]struct {
		userIDs     slack.UserIDs       // 対応表
		setup       func(st *fakeStore) // 前もって親を仕込む
		calls       []call              // 順に渡すイベント
		wantParent  int                 // メンションした親の投稿の数
		wantReplies int                 // メンションした返信の数
	}{
		"開始が先に届くと、親でメンションし、返信と書き換えではメンションしない": {
			userIDs:    slack.UserIDs{"tamaco489": testSlackUserID},
			calls:      []call{{ev: evStarted}, {ev: evInProgress}, {ev: evCompleted}},
			wantParent: 1,
		},
		"進行中が先に届くと、親ではなく開始の返信でメンションする": {
			userIDs:     slack.UserIDs{"tamaco489": testSlackUserID},
			calls:       []call{{ev: evInProgress}, {ev: evStarted}, {ev: evCompleted}},
			wantReplies: 1,
		},
		"開始の返信の失敗のあとに流し直しても、メンションは 1 回": {
			userIDs:     slack.UserIDs{"tamaco489": testSlackUserID},
			calls:       []call{{ev: evInProgress}, {ev: evStarted, failPost: true, wantErr: true}, {ev: evStarted}},
			wantReplies: 1,
		},
		"開始で作った親があるときに開始を流し直しても、返信ではメンションしない": {
			userIDs: slack.UserIDs{"tamaco489": testSlackUserID},
			setup: func(st *fakeStore) {
				st.parents[testDeployment] = deploy.Parent{
					DeploymentID: testDeployment,
					TS:           testParentTS,
					Channel:      testChannel,
					Service:      testService,
					Status:       deploy.StatusDeploying,
					Version:      "v3",
					Actor:        "tamaco489",
					Rev:          1,
				}
				st.steps["parent#"+testDeployment] = &fakeStep{done: true}
			},
			calls: []call{{ev: evStarted}},
		},
		"対応表に無い人には、メンションしない": {
			userIDs: slack.UserIDs{"someone": testSlackUserID},
			calls:   []call{{ev: evStarted}, {ev: evInProgress}},
		},
		"対応表が無ければ、メンションしない": {
			calls: []call{{ev: evInProgress}, {ev: evStarted}},
		},
		"起動した人が分からないデプロイでは、メンションしない": {
			userIDs: slack.UserIDs{"tamaco489": testSlackUserID},
			calls:   []call{{ev: evInProgress}, {ev: evCompleted}},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fs := &fakeSlack{}
			st := newFakeStore()
			if tt.setup != nil {
				tt.setup(st)
			}
			n := newTestNotifier(fs, st, &fakeECS{})
			n.UserIDs = tt.userIDs

			for i, c := range tt.calls {
				if c.failPost {
					fs.failures = map[string]int{"chat.postMessage": 1}
				}
				err := n.Handle(t.Context(), c.ev)
				if (err != nil) != c.wantErr {
					t.Fatalf("call %d (%s): err = %v, wantErr %v", i, c.ev.ID, err, c.wantErr)
				}
			}

			if got := countMentions(fs.parents()); got != tt.wantParent {
				t.Errorf("mentioned parents = %d, want %d", got, tt.wantParent)
			}
			if got := countMentions(fs.replies()); got != tt.wantReplies {
				t.Errorf("mentioned replies = %d, want %d", got, tt.wantReplies)
			}
			if got := countMentions(fs.updates); got != 0 {
				t.Errorf("mentioned updates = %d, want 0", got)
			}
		})
	}
}

// countMentions は、起動した人にメンションした投稿の数
func countMentions(msgs []slack.Message) int {
	count := 0
	for _, m := range msgs {
		if strings.Contains(m.Text, "<@"+testSlackUserID+">") {
			count++
		}
	}
	return count
}
