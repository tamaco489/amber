package deploy_test

import (
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

func TestParseEvent(t *testing.T) {
	tests := map[string]struct {
		raw             events.CloudWatchEvent
		wantKind        deploy.Kind
		wantService     string
		wantPushed      bool
		wantUnsupported bool
		wantErr         bool
	}{
		"開始": {
			raw:         startedEvent("1", true),
			wantKind:    deploy.KindStarted,
			wantService: testService,
			wantPushed:  true,
		},
		"push しない開始": {
			raw:         startedEvent("1", false),
			wantKind:    deploy.KindStarted,
			wantService: testService,
		},
		"進行中": {
			raw:         ecsEvent("ev-in-progress", deploy.ECSInProgress, "ECS deployment ecs-svc/1234567890 in progress."),
			wantKind:    deploy.KindInProgress,
			wantService: "prd-amber-api",
		},
		"サーキットブレーカーによるロールバック": {
			raw:         ecsEvent("ev-rollback", deploy.ECSInProgress, "ECS deployment circuit breaker: rolling back to deploymentId ecs-svc/111."),
			wantKind:    deploy.KindRollback,
			wantService: "prd-amber-api",
		},
		"アラームによるロールバック": {
			raw:         ecsEvent("1", deploy.ECSInProgress, "ECS deployment alarms: Rolling back to deploymentId ecs-svc/111."),
			wantKind:    deploy.KindRollback,
			wantService: "prd-amber-api",
		},
		"完了": {
			raw:         ecsEvent("ev-completed", deploy.ECSCompleted, "ECS deployment ecs-svc/1234567890 completed."),
			wantKind:    deploy.KindCompleted,
			wantService: "prd-amber-api",
		},
		"失敗": {
			raw:         ecsEvent("ev-failed", deploy.ECSFailed, "ECS deployment circuit breaker: task failed to start."),
			wantKind:    deploy.KindFailed,
			wantService: "prd-amber-api",
		},
		"知らない ECS のイベント名": {
			raw:             ecsEvent("1", "SERVICE_DEPLOYMENT_STOPPED", ""),
			wantUnsupported: true,
		},
		"知らない出どころ": {
			raw:             events.CloudWatchEvent{ID: "1", Source: "aws.ec2", DetailType: "x"},
			wantUnsupported: true,
		},
		"ID がない": {
			raw:     events.CloudWatchEvent{Source: deploy.ECSSource, DetailType: deploy.ECSDetailType},
			wantErr: true,
		},
		"デプロイ ID がない": {
			raw: events.CloudWatchEvent{
				ID:         "1",
				Source:     deploy.ECSSource,
				DetailType: deploy.ECSDetailType,
				Detail:     []byte(`{"eventName":"SERVICE_DEPLOYMENT_COMPLETED"}`),
			},
			wantErr: true,
		},
		"壊れた detail": {
			raw: events.CloudWatchEvent{
				ID:         "1",
				Source:     deploy.StartedSource,
				DetailType: deploy.StartedDetailType,
				Detail:     []byte(`{`),
			},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ev, err := deploy.ParseEvent(tt.raw, testService)

			if tt.wantUnsupported {
				if !errors.Is(err, deploy.ErrUnsupported) {
					t.Fatalf("err = %v, want ErrUnsupported", err)
				}
				return
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if errors.Is(err, deploy.ErrUnsupported) {
					t.Errorf("err = %v, want an error other than ErrUnsupported", err)
				}
				return
			}
			if ev.Kind != tt.wantKind {
				t.Errorf("kind = %s, want %s", ev.Kind, tt.wantKind)
			}
			if ev.Service != tt.wantService {
				t.Errorf("service = %q, want %q", ev.Service, tt.wantService)
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
