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
			ev:          evStarted,
			wantStatus:  deploy.StatusDeploying,
			wantChanged: true,
		},
		"進行中では状態が変わらない": {
			from:       deploy.StatusDeploying,
			ev:         evInProgress,
			wantStatus: deploy.StatusDeploying,
		},
		"ロールバックに進む": {
			from:        deploy.StatusDeploying,
			ev:          evRollback,
			wantStatus:  deploy.StatusRollingBack,
			wantChanged: true,
		},
		"完了に進む": {
			from:        deploy.StatusDeploying,
			ev:          evCompleted,
			wantStatus:  deploy.StatusCompleted,
			wantChanged: true,
		},
		"遅れて届いたロールバックで、完了から戻らない": {
			from:       deploy.StatusCompleted,
			ev:         evRollback,
			wantStatus: deploy.StatusCompleted,
		},
		"失敗のあとの完了では、失敗のまま": {
			from:       deploy.StatusFailed,
			ev:         evCompleted,
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

func TestParentApplyECSDeploymentURL(t *testing.T) {
	const url = "https://ap-northeast-1.console.aws.amazon.com/ecs/v2/clusters/prd-amber/services/prd-amber-api/deployments/abc?region=ap-northeast-1"

	tests := map[string]struct {
		have        string // 親に入っている URL
		evURL       string // イベントで引けた URL
		wantURL     string
		wantChanged bool
	}{
		"引けた URL で埋める": {
			evURL:       url,
			wantURL:     url,
			wantChanged: true,
		},
		"引けなかったときは、入っている URL を消さない": {
			have:    url,
			wantURL: url,
		},
		"同じ URL なら変わらない": {
			have:    url,
			evURL:   url,
			wantURL: url,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			p := deploy.Parent{DeploymentID: testDeployment, Service: testService, Status: deploy.StatusDeploying, ECSDeploymentURL: tt.have}
			ev := &deploy.Event{Kind: deploy.KindInProgress, DeploymentID: testDeployment, Service: testService, ECSDeploymentURL: tt.evURL}

			changed := p.Apply(ev)

			if p.ECSDeploymentURL != tt.wantURL {
				t.Errorf("url = %q, want %q", p.ECSDeploymentURL, tt.wantURL)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
		})
	}
}
