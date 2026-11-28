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

// TestHandleMention は、起動した人へのメンションを付ける投稿を確かめる
//   - 親の投稿か開始の返信のどちらか 1 回だけでメンションし、親の書き換えでは、親に起動した人が入っていればメンションを付け直す
func TestHandleMention(t *testing.T) {
	type call struct {
		ev      events.CloudWatchEvent // 渡すイベント
		fail    string                 // このイベントで 1 回だけ失敗させる Slack のメソッド (空なら失敗させない)
		wantErr bool                   // エラーになるか
	}

	tests := map[string]struct {
		userIDs            slack.UserIDs       // 対応表
		ecsURL             string              // ECS のデプロイのページの URL (空なら、ロールバックでない進行中で親を書き換えない)
		setup              func(st *fakeStore) // 前もって親を仕込む
		calls              []call              // 順に渡すイベント
		wantParent         int                 // メンションした親の投稿の数
		wantReplies        int                 // メンションした返信の数
		wantUpdateMentions []bool              // 親の書き換え (届いた順) ごとに、メンションを付けたか
	}{
		"開始が先に届くと、親でメンションし、書き換えでも付け直し、返信ではメンションしない": {
			userIDs:            slack.UserIDs{"tamaco489": testSlackUserID},
			ecsURL:             testECSURL,
			calls:              []call{{ev: evStarted}, {ev: evInProgress}, {ev: evCompleted}},
			wantParent:         1,
			wantUpdateMentions: []bool{true, true},
		},
		"進行中が先に届くと、親ではなく開始の返信でメンションし、そのあとの書き換えで親にも付ける": {
			userIDs:            slack.UserIDs{"tamaco489": testSlackUserID},
			ecsURL:             testECSURL,
			calls:              []call{{ev: evInProgress}, {ev: evStarted}, {ev: evCompleted}},
			wantReplies:        1,
			wantUpdateMentions: []bool{true, true},
		},
		"開始の返信の失敗のあとに流し直しても、返信のメンションは 1 回": {
			userIDs:            slack.UserIDs{"tamaco489": testSlackUserID},
			calls:              []call{{ev: evInProgress}, {ev: evStarted, fail: "chat.postMessage", wantErr: true}, {ev: evStarted}},
			wantReplies:        1,
			wantUpdateMentions: []bool{true},
		},
		"親の書き換えの失敗のあとに流し直しても、書き換えでメンションを付け直す": {
			userIDs:            slack.UserIDs{"tamaco489": testSlackUserID},
			calls:              []call{{ev: evStarted}, {ev: evCompleted, fail: "chat.update", wantErr: true}, {ev: evCompleted}},
			wantParent:         1,
			wantUpdateMentions: []bool{true},
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
			calls:              []call{{ev: evStarted}},
			wantUpdateMentions: []bool{true},
		},
		"対応表に無い人には、親の投稿でも書き換えでもメンションしない": {
			userIDs:            slack.UserIDs{"someone": testSlackUserID},
			ecsURL:             testECSURL,
			calls:              []call{{ev: evStarted}, {ev: evInProgress}},
			wantUpdateMentions: []bool{false},
		},
		"対応表が無ければ、返信でも書き換えでもメンションしない": {
			calls:              []call{{ev: evInProgress}, {ev: evStarted}},
			wantUpdateMentions: []bool{false},
		},
		"起動した人が分からないデプロイでは、親の投稿でも書き換えでもメンションしない": {
			userIDs:            slack.UserIDs{"tamaco489": testSlackUserID},
			calls:              []call{{ev: evInProgress}, {ev: evCompleted}},
			wantUpdateMentions: []bool{false},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fs := &fakeSlack{}
			st := newFakeStore()
			if tt.setup != nil {
				tt.setup(st)
			}
			n := newTestNotifier(fs, st, &fakeECS{url: tt.ecsURL})
			n.UserIDs = tt.userIDs

			for i, c := range tt.calls {
				if c.fail != "" {
					fs.failures = map[string]int{c.fail: 1}
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
			if len(fs.updates) != len(tt.wantUpdateMentions) {
				t.Fatalf("updates = %d, want %d", len(fs.updates), len(tt.wantUpdateMentions))
			}
			for i, want := range tt.wantUpdateMentions {
				if got := mentioned(fs.updates[i]); got != want {
					t.Errorf("update %d mentioned = %t, want %t", i, got, want)
				}
			}
		})
	}
}

// countMentions は、起動した人にメンションした投稿の数
func countMentions(msgs []slack.Message) int {
	count := 0
	for _, m := range msgs {
		if mentioned(m) {
			count++
		}
	}
	return count
}

// mentioned は、起動した人にメンションした投稿か
func mentioned(m slack.Message) bool {
	return strings.Contains(m.Text, "<@"+testSlackUserID+">")
}
