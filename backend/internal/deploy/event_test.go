package deploy_test

import (
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/deploy/deploytest"
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
			raw:         deploytest.Started("1", true),
			wantKind:    deploy.KindStarted,
			wantService: deploytest.Service,
			wantPushed:  true,
		},
		"push しない開始": {
			raw:         deploytest.Started("1", false),
			wantKind:    deploy.KindStarted,
			wantService: deploytest.Service,
		},
		"進行中": {
			raw:         deploytest.EvInProgress,
			wantKind:    deploy.KindInProgress,
			wantService: "prd-amber-api",
		},
		"サーキットブレーカーによるロールバック": {
			raw:         deploytest.EvRollback,
			wantKind:    deploy.KindRollback,
			wantService: "prd-amber-api",
		},
		"アラームによるロールバック": {
			raw:         deploytest.ECS("1", deploy.ECSInProgress, "ECS deployment alarms: Rolling back to deploymentId ecs-svc/111."),
			wantKind:    deploy.KindRollback,
			wantService: "prd-amber-api",
		},
		"完了": {
			raw:         deploytest.EvCompleted,
			wantKind:    deploy.KindCompleted,
			wantService: "prd-amber-api",
		},
		"失敗": {
			raw:         deploytest.EvFailed,
			wantKind:    deploy.KindFailed,
			wantService: "prd-amber-api",
		},
		"知らない ECS のイベント名": {
			raw:             deploytest.ECS("1", "SERVICE_DEPLOYMENT_STOPPED", ""),
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
			ev, err := deploy.ParseEvent(tt.raw, deploytest.Service)

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
			if ev.DeploymentID != deploytest.Deployment {
				t.Errorf("deploymentId = %q, want %q", ev.DeploymentID, deploytest.Deployment)
			}
		})
	}
}
