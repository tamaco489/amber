package notifier

import (
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

// TestHandleECSDeploymentURL は、ECS のデプロイのページの URL を引く流れ (引く回数、親を書き換える回数、親に残る URL) を確かめる
func TestHandleECSDeploymentURL(t *testing.T) {
	tests := map[string]struct {
		ecs         *fakeECS                 // ECS の差し替え
		events      []events.CloudWatchEvent // 順に渡すイベント
		wantCalls   int                      // URL を引いた回数
		wantUpdates int                      // 親を書き換えた回数
		wantURL     string                   // 親に残る URL
	}{
		"開始が先に届いた親に、進行中で URL を足す": {
			ecs:         &fakeECS{url: testECSURL},
			events:      []events.CloudWatchEvent{evStarted, evInProgress},
			wantCalls:   1,
			wantUpdates: 1,
			wantURL:     testECSURL,
		},
		"進行中が先に届くと、URL の入った親を作る": {
			ecs:       &fakeECS{url: testECSURL},
			events:    []events.CloudWatchEvent{evInProgress},
			wantCalls: 1,
			wantURL:   testECSURL,
		},
		"同じイベントの再試行では、URL を足す書き換えは 1 回だけ": {
			ecs:         &fakeECS{url: testECSURL},
			events:      []events.CloudWatchEvent{evStarted, evInProgress, evInProgress},
			wantCalls:   2,
			wantUpdates: 1,
			wantURL:     testECSURL,
		},
		"引けなくても、通知は続け、URL は空のまま": {
			ecs:       &fakeECS{err: errors.New("AccessDeniedException")},
			events:    []events.CloudWatchEvent{evStarted, evInProgress},
			wantCalls: 1,
		},
		"引けなかった URL を、完了で引き直して埋める": {
			ecs:         &fakeECS{url: testECSURL},
			events:      []events.CloudWatchEvent{evStarted, evCompleted},
			wantCalls:   1,
			wantUpdates: 1,
			wantURL:     testECSURL,
		},
		"開始 (サービスの ARN を持たない) では引かない": {
			ecs:    &fakeECS{url: testECSURL},
			events: []events.CloudWatchEvent{evStarted},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fs := &fakeSlack{}
			st := newFakeStore()
			n := newTestNotifier(fs, st, tt.ecs)

			for _, ev := range tt.events {
				if err := n.Handle(t.Context(), ev); err != nil {
					t.Fatalf("handle %s: %v", ev.ID, err)
				}
			}

			if tt.ecs.calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", tt.ecs.calls, tt.wantCalls)
			}
			if len(fs.updates) != tt.wantUpdates {
				t.Errorf("updates = %d, want %d", len(fs.updates), tt.wantUpdates)
			}
			if got := st.parents[testDeployment].ECSDeploymentURL; got != tt.wantURL {
				t.Errorf("url = %q, want %q", got, tt.wantURL)
			}
		})
	}
}
