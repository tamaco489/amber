// Package deploytest は、テストで使うデプロイのイベントを作る
package deploytest

import (
	"encoding/json"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

// テストで使う値
const (
	Channel    = "C0TEST"
	Service    = "prd-amber-api"
	Deployment = "ecs-svc/1234567890"
	ServiceARN = "arn:aws:ecs:ap-northeast-1:111122223333:service/prd-amber/prd-amber-api"
	TaskDef    = "arn:aws:ecs:ap-northeast-1:111122223333:task-definition/prd-amber-api:7"
	RunURL     = "https://github.com/tamaco489/amber/actions/runs/1"
	ParentTS   = "1700000000.000001"
)

// Time は、テストのイベントの時刻 (JST では 2026-11-23 11:12:30)
var Time = time.Date(2026, 11, 23, 2, 12, 30, 0, time.UTC)

// Started は、cd-backend の Deployment Started のイベントを作る
func Started(id string, pushed bool) events.CloudWatchEvent {
	pushedText := "false"
	if pushed {
		pushedText = "true"
	}
	return events.CloudWatchEvent{
		ID:         id,
		Source:     deploy.StartedSource,
		DetailType: deploy.StartedDetailType,
		Time:       Time,
		Detail: mustJSON(map[string]string{
			"version":        "v3",
			"deploymentId":   Deployment,
			"taskDefinition": TaskDef,
			"pushed":         pushedText,
			"actor":          "tamaco489",
			"runUrl":         RunURL,
		}),
	}
}

// ECS は、ECS の ECS Deployment State Change のイベントを作る
func ECS(id, eventName, reason string) events.CloudWatchEvent {
	return events.CloudWatchEvent{
		ID:         id,
		Source:     deploy.ECSSource,
		DetailType: deploy.ECSDetailType,
		Time:       Time,
		Resources:  []string{ServiceARN},
		Detail: mustJSON(map[string]string{
			"eventType":    "INFO",
			"eventName":    eventName,
			"deploymentId": Deployment,
			"updatedAt":    Time.Format(time.RFC3339),
			"reason":       reason,
		}),
	}
}

// よく使うイベント
var (
	EvStarted    = Started("ev-started", true)
	EvInProgress = ECS("ev-in-progress", deploy.ECSInProgress, "ECS deployment ecs-svc/1234567890 in progress.")
	EvRollback   = ECS("ev-rollback", deploy.ECSInProgress, "ECS deployment circuit breaker: rolling back to deploymentId ecs-svc/111.")
	EvCompleted  = ECS("ev-completed", deploy.ECSCompleted, "ECS deployment ecs-svc/1234567890 completed.")
	EvFailed     = ECS("ev-failed", deploy.ECSFailed, "ECS deployment circuit breaker: task failed to start.")
)

// Parse は、イベントを deploy.Event に直す。失敗したら panic する (テストの前提が壊れているため)
func Parse(raw events.CloudWatchEvent) *deploy.Event {
	ev, err := deploy.ParseEvent(raw, Service)
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
