package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/tamaco489/amber/backend/internal/ecs"
	"github.com/tamaco489/amber/backend/internal/notifier"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// TestNotifyMention は、対応表の JSON を読み、生のイベントを順に渡して、Slack に届く本文 (text) のメンションを確かめる
func TestNotifyMention(t *testing.T) {
	tests := map[string]struct {
		userIDs     string                   // 対応表の JSON (SSM のパラメータの値)
		events      []events.CloudWatchEvent // 順に渡すイベント
		wantParent  string                   // 親の投稿 (chat.postMessage) の text
		wantReplies []string                 // 返信の text (届いた順)
		wantUpdates []string                 // 親の書き換え (chat.update) の text (届いた順)
	}{
		"開始が先に届くと、親の投稿の先頭でメンションする": {
			userIDs:    `{"tamaco489": "U0TEST"}`,
			events:     []events.CloudWatchEvent{evStarted, evInProgress, evCompleted},
			wantParent: "<@U0TEST> *:rocket: Deploying v3: prd-amber-api*",
			wantReplies: []string{
				"*:rocket: Deploy started: v3*",
				"*:hourglass_flowing_sand: Deployment in progress: prd-amber-api*",
				"*:white_check_mark: Deployment completed: prd-amber-api*",
			},
			wantUpdates: []string{
				"*:rocket: Deploying v3: prd-amber-api*",
				"*:white_check_mark: Deployed v3: prd-amber-api*",
			},
		},
		"進行中が先に届くと、開始の返信の先頭でメンションする": {
			userIDs:    `{"tamaco489": "U0TEST"}`,
			events:     []events.CloudWatchEvent{evInProgress, evStarted},
			wantParent: "*:rocket: Deploying: prd-amber-api*",
			wantReplies: []string{
				"*:hourglass_flowing_sand: Deployment in progress: prd-amber-api*",
				"<@U0TEST> *:rocket: Deploy started: v3*",
			},
			wantUpdates: []string{"*:rocket: Deploying v3: prd-amber-api*"},
		},
		"対応表に無い人には、メンションしない": {
			userIDs:     `{}`,
			events:      []events.CloudWatchEvent{evStarted},
			wantParent:  "*:rocket: Deploying v3: prd-amber-api*",
			wantReplies: []string{"*:rocket: Deploy started: v3*"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ids, err := slack.ParseUserIDs(tt.userIDs)
			if err != nil {
				t.Fatalf("parse user ids: %v", err)
			}
			fs := &fakeSlack{}
			n := &notifier.Notifier{
				Slack:   fs,
				Store:   newFakeStore(),
				ECS:     ecs.New(&fakeECSAPI{deployments: []types.ServiceDeploymentBrief{testServiceDeployment}}),
				Channel: testChannel,
				Service: testService,
				UserIDs: ids,
				Now:     func() time.Time { return testTime },
				Sleep:   func(context.Context, time.Duration) error { return nil },
			}

			for _, ev := range tt.events {
				if err := n.Handle(t.Context(), ev); err != nil {
					t.Fatalf("handle %s: %v", ev.ID, err)
				}
			}

			parents := fs.parents()
			if len(parents) != 1 {
				t.Fatalf("parents = %d, want 1", len(parents))
			}
			if got := parents[0].Text; got != tt.wantParent {
				t.Errorf("parent text = %q, want %q", got, tt.wantParent)
			}
			assertTexts(t, "reply", fs.replies(), tt.wantReplies)
			assertTexts(t, "update", fs.updates, tt.wantUpdates)
		})
	}
}
