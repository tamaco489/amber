package ecs

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

func TestDeploymentURL(t *testing.T) {
	const wantURL = "https://ap-northeast-1.console.aws.amazon.com/ecs/v2/clusters/prd-amber/services/prd-amber-api/deployments/sd-target?region=ap-northeast-1"

	tests := map[string]struct {
		pages     [][]types.ServiceDeploymentBrief
		wantURL   string
		wantCalls int
	}{
		"最初のページで見つかる": {
			pages:     [][]types.ServiceDeploymentBrief{{brief("111", "sd-other"), brief("1234567890", "sd-target")}},
			wantURL:   wantURL,
			wantCalls: 1,
		},
		"次のページで見つかる": {
			pages:     [][]types.ServiceDeploymentBrief{{brief("111", "sd-other")}, {brief("1234567890", "sd-target")}},
			wantURL:   wantURL,
			wantCalls: 2,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{pages: tt.pages}
			ev := &deploy.Event{DeploymentID: testDeployment, ServiceARN: testServiceARN}

			got, err := New(api).DeploymentURL(t.Context(), ev)
			if err != nil {
				t.Fatalf("err = %v", err)
			}

			if got != tt.wantURL {
				t.Errorf("url = %q, want %q", got, tt.wantURL)
			}
			if len(api.calls) != tt.wantCalls {
				t.Errorf("calls = %d, want %d", len(api.calls), tt.wantCalls)
			}
			in := api.calls[0]
			if aws.ToString(in.Cluster) != "prd-amber" || aws.ToString(in.Service) != testServiceARN {
				t.Errorf("cluster = %q, service = %q", aws.ToString(in.Cluster), aws.ToString(in.Service))
			}
		})
	}
}

func TestDeploymentURLError(t *testing.T) {
	manyPages := make([][]types.ServiceDeploymentBrief, maxPages+1)
	for i := range manyPages {
		manyPages[i] = []types.ServiceDeploymentBrief{brief("111", "sd-other")}
	}

	tests := map[string]struct {
		ev           *deploy.Event
		api          *fakeAPI
		wantNotFound bool // ErrNotFound か
		wantCalls    int
	}{
		"一覧に無い": {
			ev:           &deploy.Event{DeploymentID: testDeployment, ServiceARN: testServiceARN},
			api:          &fakeAPI{pages: [][]types.ServiceDeploymentBrief{{brief("111", "sd-other")}}},
			wantNotFound: true,
			wantCalls:    1,
		},
		"決めたページ数を超えては探さない": {
			ev:           &deploy.Event{DeploymentID: testDeployment, ServiceARN: testServiceARN},
			api:          &fakeAPI{pages: manyPages},
			wantNotFound: true,
			wantCalls:    maxPages,
		},
		"API のエラー": {
			ev:        &deploy.Event{DeploymentID: testDeployment, ServiceARN: testServiceARN},
			api:       &fakeAPI{err: errors.New("AccessDeniedException")},
			wantCalls: 1,
		},
		"サービスの ARN が無い (Deployment Started)": {
			ev:  &deploy.Event{DeploymentID: testDeployment},
			api: &fakeAPI{},
		},
		"デプロイ ID の形が違う": {
			ev:  &deploy.Event{DeploymentID: "d-123", ServiceARN: testServiceARN},
			api: &fakeAPI{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := New(tt.api).DeploymentURL(t.Context(), tt.ev)

			if err == nil {
				t.Fatalf("url = %q, want an error", got)
			}
			if got != "" {
				t.Errorf("url = %q, want empty", got)
			}
			if errors.Is(err, ErrNotFound) != tt.wantNotFound {
				t.Errorf("err = %v, want not found = %v", err, tt.wantNotFound)
			}
			if len(tt.api.calls) != tt.wantCalls {
				t.Errorf("calls = %d, want %d", len(tt.api.calls), tt.wantCalls)
			}
		})
	}
}
