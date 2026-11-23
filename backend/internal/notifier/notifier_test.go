package notifier

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/slack"
)

const (
	testChannel    = "C0TEST"
	testService    = "prd-amber-api"
	testDeployment = "ecs-svc/1234567890"
	testServiceARN = "arn:aws:ecs:ap-northeast-1:111122223333:service/prd-amber/prd-amber-api"
	testTaskDef    = "arn:aws:ecs:ap-northeast-1:111122223333:task-definition/prd-amber-api:7"
	testRunURL     = "https://github.com/tamaco489/amber/actions/runs/1"
)

var testTime = time.Date(2026, 11, 23, 2, 12, 30, 0, time.UTC)

func startedEvent(id string, pushed bool) events.CloudWatchEvent {
	detail, _ := json.Marshal(map[string]string{
		"version":        "v3",
		"deploymentId":   testDeployment,
		"taskDefinition": testTaskDef,
		"pushed":         map[bool]string{true: "true", false: "false"}[pushed],
		"actor":          "tamaco489",
		"runUrl":         testRunURL,
	})
	return events.CloudWatchEvent{
		ID: id, Source: startedSource, DetailType: startedDetailType, Time: testTime, Detail: detail,
	}
}

func ecsEvent(id, eventName, reason string) events.CloudWatchEvent {
	detail, _ := json.Marshal(map[string]string{
		"eventType":    "INFO",
		"eventName":    eventName,
		"deploymentId": testDeployment,
		"updatedAt":    testTime.Format(time.RFC3339),
		"reason":       reason,
	})
	return events.CloudWatchEvent{
		ID: id, Source: ecsSource, DetailType: ecsDetailType, Time: testTime,
		Resources: []string{testServiceARN}, Detail: detail,
	}
}

var (
	evStarted    = startedEvent("ev-started", true)
	evInProgress = ecsEvent("ev-in-progress", ecsInProgress, "ECS deployment ecs-svc/1234567890 in progress.")
	evRollback   = ecsEvent("ev-rollback", ecsInProgress, "ECS deployment circuit breaker: rolling back to deploymentId ecs-svc/111.")
	evCompleted  = ecsEvent("ev-completed", ecsCompleted, "ECS deployment ecs-svc/1234567890 completed.")
	evFailed     = ecsEvent("ev-failed", ecsFailed, "ECS deployment circuit breaker: task failed to start.")
)

func headerOf(m slack.Message) string {
	if len(m.Attachments) == 0 || len(m.Attachments[0].Blocks) == 0 || m.Attachments[0].Blocks[0].Text == nil {
		return ""
	}
	return m.Attachments[0].Blocks[0].Text.Text
}

func colorOf(m slack.Message) string {
	if len(m.Attachments) == 0 {
		return ""
	}
	return m.Attachments[0].Color
}

// bodyOf は、ブロックの mrkdwn の文をつなげたもの (項目の値を確かめるため)
func bodyOf(m slack.Message) string {
	var b strings.Builder
	for _, a := range m.Attachments {
		for _, blk := range a.Blocks {
			if blk.Text != nil {
				b.WriteString(blk.Text.Text + "\n")
			}
			for _, f := range blk.Fields {
				b.WriteString(f.Text + "\n")
			}
			for _, e := range blk.Elements {
				b.WriteString(e.Text + "\n")
			}
		}
	}
	return b.String()
}

func TestHandle(t *testing.T) {
	type call struct {
		ev      events.CloudWatchEvent
		wantErr bool
	}

	tests := []struct {
		name  string
		setup func(fs *fakeSlack, st *fakeStore)
		calls []call
		check func(t *testing.T, fs *fakeSlack, st *fakeStore)
	}{
		{
			name:  "started first creates the parent with actor and link",
			calls: []call{{ev: evStarted}, {ev: evInProgress}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				parents := fs.parents()
				if len(parents) != 1 {
					t.Fatalf("parents = %d, want 1", len(parents))
				}
				if got := headerOf(parents[0]); got != ":rocket: Deploying v3: prd-amber-api" {
					t.Errorf("parent header = %q", got)
				}
				body := bodyOf(parents[0])
				for _, want := range []string{"*Triggered by*\ntamaco489", "*Task definition*\nprd-amber-api:7", "<" + testRunURL + "|GitHub Actions run>", "*Status*\nDeploying"} {
					if !strings.Contains(body, want) {
						t.Errorf("parent body does not contain %q:\n%s", want, body)
					}
				}
				if got := colorOf(parents[0]); got != colorInfo {
					t.Errorf("parent color = %q, want %q", got, colorInfo)
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
		{
			name:  "in progress before started creates exactly one parent and fills it later",
			calls: []call{{ev: evInProgress}, {ev: evStarted}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				parents := fs.parents()
				if len(parents) != 1 {
					t.Fatalf("parents = %d, want 1", len(parents))
				}
				first := bodyOf(parents[0])
				if !strings.Contains(first, "*Triggered by*\nnone") || !strings.Contains(first, "GitHub Actions run: none") {
					t.Errorf("first parent should have no actor and no link:\n%s", first)
				}
				if got := headerOf(parents[0]); got != ":rocket: Deploying: prd-amber-api" {
					t.Errorf("first parent header = %q", got)
				}

				if len(fs.updates) != 1 {
					t.Fatalf("updates = %d, want 1", len(fs.updates))
				}
				updated := fs.updates[0]
				if updated.TS == "" {
					t.Errorf("update has no ts")
				}
				if got := headerOf(updated); got != ":rocket: Deploying v3: prd-amber-api" {
					t.Errorf("updated header = %q", got)
				}
				if !strings.Contains(bodyOf(updated), "*Triggered by*\ntamaco489") {
					t.Errorf("updated parent has no actor:\n%s", bodyOf(updated))
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
		{
			name:  "same event twice posts one reply",
			calls: []call{{ev: evInProgress}, {ev: evInProgress}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				if len(fs.parents()) != 1 {
					t.Errorf("parents = %d, want 1", len(fs.parents()))
				}
				if len(fs.replies()) != 1 {
					t.Errorf("replies = %d, want 1", len(fs.replies()))
				}
			},
		},
		{
			name: "retry after chat.update failed does not post the reply again",
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"chat.update": 1}
			},
			calls: []call{{ev: evStarted}, {ev: evCompleted, wantErr: true}, {ev: evCompleted}},
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
		{
			name: "retry after reactions.add failed does not update or reply again",
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"reactions.add": 1}
			},
			calls: []call{{ev: evStarted}, {ev: evFailed, wantErr: true}, {ev: evFailed}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				if len(fs.replies()) != 2 || len(fs.updates) != 1 || len(fs.reactions) != 1 {
					t.Errorf("replies = %d, updates = %d, reactions = %d, want 2, 1, 1", len(fs.replies()), len(fs.updates), len(fs.reactions))
				}
			},
		},
		{
			name: "retry after the parent post failed creates one parent",
			setup: func(fs *fakeSlack, _ *fakeStore) {
				fs.failures = map[string]int{"chat.postMessage": 1}
			},
			calls: []call{{ev: evInProgress, wantErr: true}, {ev: evInProgress}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				if len(fs.parents()) != 1 || len(fs.replies()) != 1 {
					t.Errorf("parents = %d, replies = %d, want 1, 1", len(fs.parents()), len(fs.replies()))
				}
			},
		},
		{
			name:  "completed turns the parent green and adds a check mark",
			calls: []call{{ev: evStarted}, {ev: evInProgress}, {ev: evCompleted}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				p := fs.lastParent()
				if got := headerOf(p); got != ":white_check_mark: Deployed v3: prd-amber-api" {
					t.Errorf("parent header = %q", got)
				}
				if colorOf(p) != colorSuccess {
					t.Errorf("parent color = %q, want %q", colorOf(p), colorSuccess)
				}
				body := bodyOf(p)
				if !strings.Contains(body, "*Status*\nCompleted") || !strings.Contains(body, "*Completed at*\n2026-11-23 11:12:30 (JST)") {
					t.Errorf("parent body:\n%s", body)
				}
				if len(fs.reactions) != 1 || fs.reactions[0].name != "white_check_mark" {
					t.Errorf("reactions = %+v, want one white_check_mark", fs.reactions)
				}
				replies := fs.replies()
				if got := headerOf(replies[len(replies)-1]); got != ":white_check_mark: Deployment completed" {
					t.Errorf("reply header = %q", got)
				}
			},
		},
		{
			name:  "rollback turns the parent red",
			calls: []call{{ev: evStarted}, {ev: evInProgress}, {ev: evRollback}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				p := fs.lastParent()
				if colorOf(p) != colorDanger || !strings.Contains(bodyOf(p), "*Status*\nRolling back") {
					t.Errorf("parent color = %q, body:\n%s", colorOf(p), bodyOf(p))
				}
				if got := headerOf(p); got != ":rewind: Rolling back v3: prd-amber-api" {
					t.Errorf("parent header = %q", got)
				}
				replies := fs.replies()
				last := replies[len(replies)-1]
				if headerOf(last) != ":rewind: Rollback started" || colorOf(last) != colorDanger {
					t.Errorf("rollback reply header = %q, color = %q", headerOf(last), colorOf(last))
				}
				if len(fs.reactions) != 0 {
					t.Errorf("reactions = %+v, want none", fs.reactions)
				}
			},
		},
		{
			name:  "failed turns the parent red and adds x",
			calls: []call{{ev: evStarted}, {ev: evRollback}, {ev: evFailed}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				p := fs.lastParent()
				if colorOf(p) != colorDanger || !strings.Contains(bodyOf(p), "*Status*\nFailed") {
					t.Errorf("parent color = %q, body:\n%s", colorOf(p), bodyOf(p))
				}
				if len(fs.reactions) != 1 || fs.reactions[0].name != "x" {
					t.Errorf("reactions = %+v, want one x", fs.reactions)
				}
			},
		},
		{
			name:  "late in progress does not move a completed parent back",
			calls: []call{{ev: evStarted}, {ev: evCompleted}, {ev: evRollback}},
			check: func(t *testing.T, _ *fakeSlack, st *fakeStore) {
				if got := st.parents[testDeployment].Status; got != StatusCompleted {
					t.Errorf("status = %q, want %q", got, StatusCompleted)
				}
			},
		},
		{
			name: "slack ok false is returned as an error",
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
		{
			name: "a step held by another invocation returns an error without posting",
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
		{
			name:  "unsupported event is skipped",
			calls: []call{{ev: events.CloudWatchEvent{ID: "x", Source: "aws.ecs", DetailType: "ECS Task State Change", Detail: []byte(`{}`)}}},
			check: func(t *testing.T, fs *fakeSlack, _ *fakeStore) {
				if len(fs.posts) != 0 {
					t.Errorf("posts = %d, want 0", len(fs.posts))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &fakeSlack{}
			st := newFakeStore()
			if tt.setup != nil {
				tt.setup(fs, st)
			}
			n := &Notifier{
				Slack:   fs,
				Store:   st,
				Channel: testChannel,
				Service: testService,
				Now:     func() time.Time { return testTime },
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
	st.steps["parent#"+testDeployment] = &fakeStep{leaseUntil: testTime.Add(time.Minute)}
	n := &Notifier{Slack: &fakeSlack{}, Store: st, Channel: testChannel, Service: testService, Now: func() time.Time { return testTime }}

	if err := n.Handle(t.Context(), evInProgress); !errors.Is(err, ErrBusy) {
		t.Errorf("err = %v, want ErrBusy", err)
	}
}
