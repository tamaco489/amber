package deploy_test

import (
	"testing"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

func TestParentApply(t *testing.T) {
	tests := map[string]struct {
		from        deploy.Status
		ev          events.CloudWatchEvent
		wantStatus  deploy.Status
		wantChanged bool
	}{
		"開始で値が埋まる": {
			from:        deploy.StatusDeploying,
			ev:          startedEvent("ev-started", true),
			wantStatus:  deploy.StatusDeploying,
			wantChanged: true,
		},
		"進行中では状態が変わらない": {
			from:       deploy.StatusDeploying,
			ev:         ecsEvent("ev-in-progress", deploy.ECSInProgress, "ECS deployment ecs-svc/1234567890 in progress."),
			wantStatus: deploy.StatusDeploying,
		},
		"ロールバックに進む": {
			from:        deploy.StatusDeploying,
			ev:          ecsEvent("ev-rollback", deploy.ECSInProgress, "ECS deployment circuit breaker: rolling back to deploymentId ecs-svc/111."),
			wantStatus:  deploy.StatusRollingBack,
			wantChanged: true,
		},
		"完了に進む": {
			from:        deploy.StatusDeploying,
			ev:          ecsEvent("ev-completed", deploy.ECSCompleted, "ECS deployment ecs-svc/1234567890 completed."),
			wantStatus:  deploy.StatusCompleted,
			wantChanged: true,
		},
		"遅れて届いたロールバックで、完了から戻らない": {
			from:       deploy.StatusCompleted,
			ev:         ecsEvent("ev-rollback", deploy.ECSInProgress, "ECS deployment circuit breaker: rolling back to deploymentId ecs-svc/111."),
			wantStatus: deploy.StatusCompleted,
		},
		"失敗のあとの完了では、失敗のまま": {
			from:       deploy.StatusFailed,
			ev:         ecsEvent("ev-completed", deploy.ECSCompleted, "ECS deployment ecs-svc/1234567890 completed."),
			wantStatus: deploy.StatusFailed,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p := deploy.Parent{DeploymentID: testDeployment, Service: testService, Status: tt.from}

			changed := p.Apply(parse(tt.ev))

			if p.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", p.Status, tt.wantStatus)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
		})
	}
}
