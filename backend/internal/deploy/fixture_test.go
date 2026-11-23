package deploy_test

import (
	"encoding/json"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

// テストで使う値
const (
	testService    = "prd-amber-api"
	testDeployment = "ecs-svc/1234567890"
	testServiceARN = "arn:aws:ecs:ap-northeast-1:111122223333:service/prd-amber/prd-amber-api"
)

// testTime は、テストのイベントの時刻
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
			"taskDefinition": "arn:aws:ecs:ap-northeast-1:111122223333:task-definition/prd-amber-api:7",
			"pushed":         pushedText,
			"actor":          "tamaco489",
			"runUrl":         "https://github.com/tamaco489/amber/actions/runs/1",
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
			"eventName":    eventName,
			"deploymentId": testDeployment,
			"reason":       reason,
		}),
	}
}

// parse は、イベントを読む。失敗したら panic する (テストの前提が壊れているため)
func parse(raw events.CloudWatchEvent) *deploy.Event {
	ev, err := deploy.ParseEvent(raw, testService)
	if err != nil {
		panic(err)
	}
	return ev
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
