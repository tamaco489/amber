package deploy_test

import (
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

func TestParseEvent(t *testing.T) {
	tests := map[string]struct {
		raw        events.CloudWatchEvent
		wantKind   deploy.Kind
		wantPushed bool
	}{
		"開始": {
			raw:        evStarted,
			wantKind:   deploy.KindStarted,
			wantPushed: true,
		},
		"push しない開始": {
			raw:      startedEvent("ev-started", false),
			wantKind: deploy.KindStarted,
		},
		"進行中": {
			raw:      evInProgress,
			wantKind: deploy.KindInProgress,
		},
		"サーキットブレーカーによるロールバック": {
			raw:      evRollback,
			wantKind: deploy.KindRollback,
		},
		"アラームによるロールバック": {
			raw:      ecsEvent("ev-alarm", deploy.ECSInProgress, "ECS deployment alarms: Rolling back to deploymentId ecs-svc/111."),
			wantKind: deploy.KindRollback,
		},
		"完了": {
			raw:      evCompleted,
			wantKind: deploy.KindCompleted,
		},
		"失敗": {
			raw:      evFailed,
			wantKind: deploy.KindFailed,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ev, err := deploy.ParseEvent(tt.raw, testService)
			if err != nil {
				t.Fatalf("err = %v", err)
			}

			if ev.Kind != tt.wantKind {
				t.Errorf("kind = %s, want %s", ev.Kind, tt.wantKind)
			}
			if ev.Service != testService {
				t.Errorf("service = %q, want %q", ev.Service, testService)
			}
			if ev.Pushed != tt.wantPushed {
				t.Errorf("pushed = %v, want %v", ev.Pushed, tt.wantPushed)
			}
			if ev.DeploymentID != testDeployment {
				t.Errorf("deploymentId = %q, want %q", ev.DeploymentID, testDeployment)
			}
		})
	}
}

func TestParseEventError(t *testing.T) {
	tests := map[string]struct {
		raw             events.CloudWatchEvent
		wantUnsupported bool // 通知の対象でないイベント (ErrUnsupported) か
	}{
		"知らない ECS のイベント名": {
			raw:             ecsEvent("ev-stopped", "SERVICE_DEPLOYMENT_STOPPED", ""),
			wantUnsupported: true,
		},
		"知らない出どころ": {
			raw:             rawEvent("ev-ec2", "aws.ec2", "x", ""),
			wantUnsupported: true,
		},
		"ID がない": {
			raw: rawEvent("", deploy.ECSSource, deploy.ECSDetailType, ""),
		},
		"デプロイ ID がない": {
			raw: rawEvent("ev-no-id", deploy.ECSSource, deploy.ECSDetailType, `{"eventName":"SERVICE_DEPLOYMENT_COMPLETED"}`),
		},
		"壊れた detail": {
			raw: rawEvent("ev-broken", deploy.StartedSource, deploy.StartedDetailType, `{`),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ev, err := deploy.ParseEvent(tt.raw, testService)

			if err == nil {
				t.Fatalf("event = %+v, want an error", ev)
			}
			if got := errors.Is(err, deploy.ErrUnsupported); got != tt.wantUnsupported {
				t.Errorf("err = %v, unsupported = %v, want %v", err, got, tt.wantUnsupported)
			}
		})
	}
}
