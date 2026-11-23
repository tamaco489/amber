package notifier

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

// Kind は、通知の種類
type Kind int

// 通知の種類 (届くイベントは 4 種類。ECS の進行中は、reason でロールバックかどうかを分ける)
const (
	KindStarted Kind = iota + 1
	KindInProgress
	KindRollback
	KindCompleted
	KindFailed
)

func (k Kind) String() string {
	switch k {
	case KindStarted:
		return "started"
	case KindInProgress:
		return "in_progress"
	case KindRollback:
		return "rollback"
	case KindCompleted:
		return "completed"
	case KindFailed:
		return "failed"
	}
	return "unknown"
}

// cd-backend が送る、デプロイを始めたことのイベント
//   - infra の modules/notification と、cd-backend の notify-deploy-started.sh と同じ値にする
const (
	startedSource     = "amber.cd-backend"
	startedDetailType = "Deployment Started"
)

// ECS のデプロイの状態の変化のイベント
const (
	ecsSource     = "aws.ecs"
	ecsDetailType = "ECS Deployment State Change"

	ecsInProgress = "SERVICE_DEPLOYMENT_IN_PROGRESS"
	ecsCompleted  = "SERVICE_DEPLOYMENT_COMPLETED"
	ecsFailed     = "SERVICE_DEPLOYMENT_FAILED"
)

// ロールバックを表す reason の文言
//   - ロールバックも SERVICE_DEPLOYMENT_IN_PROGRESS で届き、違いは reason だけ
//     (例: ECS deployment circuit breaker: rolling back to deploymentId ecs-svc/123.)
//   - サーキットブレーカーとアラームのどちらのロールバックも拾えるよう、前半ではなく "rolling back" で見分ける
//   - 大文字と小文字は区別しない
//
// DOC: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html
const rollbackReason = "rolling back"

// ErrUnsupported は、通知の対象ではないイベント
var ErrUnsupported = errors.New("unsupported event")

// Event は、届いたイベントを、通知に使う形に直したもの
type Event struct {
	// EventBridge のイベントの ID (再試行で同じイベントが届いたことを見分ける)
	ID           string
	Kind         Kind
	DeploymentID string
	Service      string
	Time         time.Time

	// ECS のイベントだけが持つ
	Reason string

	// Deployment Started だけが持つ
	Version        string
	Actor          string
	RunURL         string
	TaskDefinition string
	Pushed         bool
}

// cd-backend の notify-deploy-started.sh が送る detail
//   - pushed は "true" / "false" の文字列
type startedDetail struct {
	Version        string `json:"version"`
	DeploymentID   string `json:"deploymentId"`
	TaskDefinition string `json:"taskDefinition"`
	Pushed         string `json:"pushed"`
	Actor          string `json:"actor"`
	RunURL         string `json:"runUrl"`
}

// ECS の ECS Deployment State Change の detail
//
// DOC: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html
type ecsDetail struct {
	EventName    string `json:"eventName"`
	DeploymentID string `json:"deploymentId"`
	Reason       string `json:"reason"`
}

// ParseEvent は、EventBridge のイベントを Event に直す
//   - 通知の対象でないイベントは ErrUnsupported を返す
//   - Deployment Started はサービスの名前を持たないため、defaultService を使う
func ParseEvent(raw events.CloudWatchEvent, defaultService string) (Event, error) {
	ev := Event{ID: raw.ID, Time: raw.Time, Service: defaultService}
	if ev.ID == "" {
		return Event{}, errors.New("event has no id")
	}

	switch {
	case raw.Source == startedSource && raw.DetailType == startedDetailType:
		var d startedDetail
		if err := json.Unmarshal(raw.Detail, &d); err != nil {
			return Event{}, fmt.Errorf("decode %s detail: %w", raw.DetailType, err)
		}
		ev.Kind = KindStarted
		ev.DeploymentID = d.DeploymentID
		ev.Version = d.Version
		ev.Actor = d.Actor
		ev.RunURL = d.RunURL
		ev.TaskDefinition = d.TaskDefinition
		ev.Pushed = d.Pushed == "true"

	case raw.Source == ecsSource && raw.DetailType == ecsDetailType:
		var d ecsDetail
		if err := json.Unmarshal(raw.Detail, &d); err != nil {
			return Event{}, fmt.Errorf("decode %s detail: %w", raw.DetailType, err)
		}
		switch d.EventName {
		case ecsInProgress:
			ev.Kind = KindInProgress
			if strings.Contains(strings.ToLower(d.Reason), rollbackReason) {
				ev.Kind = KindRollback
			}
		case ecsCompleted:
			ev.Kind = KindCompleted
		case ecsFailed:
			ev.Kind = KindFailed
		default:
			return Event{}, fmt.Errorf("%w: %s", ErrUnsupported, d.EventName)
		}
		ev.DeploymentID = d.DeploymentID
		ev.Reason = d.Reason
		// resources[0] はサービスの ARN (arn:aws:ecs:<region>:<account>:service/<cluster>/<service>)
		if len(raw.Resources) > 0 {
			if i := strings.LastIndex(raw.Resources[0], "/"); i >= 0 && i < len(raw.Resources[0])-1 {
				ev.Service = raw.Resources[0][i+1:]
			}
		}

	default:
		return Event{}, fmt.Errorf("%w: %s / %s", ErrUnsupported, raw.Source, raw.DetailType)
	}

	if ev.DeploymentID == "" {
		return Event{}, fmt.Errorf("%s event %s has no deploymentId", ev.Kind, ev.ID)
	}
	return ev, nil
}
