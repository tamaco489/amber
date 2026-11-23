package integration_test

import (
	"encoding/json"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

// テストで使う値
const (
	testChannel    = "C0TEST"
	testService    = "prd-amber-api"
	testDeployment = "ecs-svc/1234567890"
	testServiceARN = "arn:aws:ecs:ap-northeast-1:111122223333:service/prd-amber/prd-amber-api"
	testTaskDef    = "arn:aws:ecs:ap-northeast-1:111122223333:task-definition/prd-amber-api:7"
	testRunURL     = "https://github.com/tamaco489/amber/actions/runs/1"
	testParentTS   = "1700000000.000001"
	testECSURL     = "https://ap-northeast-1.console.aws.amazon.com/ecs/v2/clusters/prd-amber/services/prd-amber-api/deployments/abc?region=ap-northeast-1"
)

// testServiceDeployment は、testDeployment のサービスデプロイ (対象のサービスリビジョンの ID が、デプロイ ID の数字と同じ)
var testServiceDeployment = types.ServiceDeploymentBrief{
	ServiceDeploymentArn:     aws.String("arn:aws:ecs:ap-northeast-1:111122223333:service-deployment/prd-amber/prd-amber-api/abc"),
	TargetServiceRevisionArn: aws.String("arn:aws:ecs:ap-northeast-1:111122223333:service-revision/prd-amber/prd-amber-api/1234567890"),
}

// testTime は、テストのイベントの時刻 (JST では 2026-11-23 11:12:30)
var testTime = time.Date(2026, 11, 23, 2, 12, 30, 0, time.UTC)

// startedEvent は、cd-backend の Deployment Started のイベントを作る
func startedEvent(id string, pushed bool) events.CloudWatchEvent {
	pushedText := "false"
	if pushed {
		pushedText = "true"
	}
	return events.CloudWatchEvent{
		ID:         id,
		Source:     deploy.StartedSource,
		DetailType: deploy.StartedDetailType,
		Time:       testTime,
		Detail: mustJSON(map[string]string{
			"version":        "v3",
			"deploymentId":   testDeployment,
			"taskDefinition": testTaskDef,
			"pushed":         pushedText,
			"actor":          "tamaco489",
			"runUrl":         testRunURL,
		}),
	}
}

// ecsEvent は、ECS の ECS Deployment State Change のイベントを作る
func ecsEvent(id, eventName, reason string) events.CloudWatchEvent {
	return events.CloudWatchEvent{
		ID:         id,
		Source:     deploy.ECSSource,
		DetailType: deploy.ECSDetailType,
		Time:       testTime,
		Resources:  []string{testServiceARN},
		Detail: mustJSON(map[string]string{
			"eventType":    "INFO",
			"eventName":    eventName,
			"deploymentId": testDeployment,
			"updatedAt":    testTime.Format(time.RFC3339),
			"reason":       reason,
		}),
	}
}

// よく使うイベント
var (
	evStarted    = startedEvent("ev-started", true)
	evInProgress = ecsEvent("ev-in-progress", deploy.ECSInProgress, "ECS deployment ecs-svc/1234567890 in progress.")
	evRollback   = ecsEvent("ev-rollback", deploy.ECSInProgress, "ECS deployment circuit breaker: rolling back to deploymentId ecs-svc/111.")
	evCompleted  = ecsEvent("ev-completed", deploy.ECSCompleted, "ECS deployment ecs-svc/1234567890 completed.")
	evFailed     = ecsEvent("ev-failed", deploy.ECSFailed, "ECS deployment circuit breaker: task failed to start.")
)

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
