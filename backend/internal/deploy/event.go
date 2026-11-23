// Package deploy は、デプロイの通知で扱う値 (届いたイベントと、親の投稿が表す状態) を持つ
//   - AWS の SDK と Slack には依存しない
package deploy

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

var kindNames = map[Kind]string{
	KindStarted:    "started",
	KindInProgress: "in_progress",
	KindRollback:   "rollback",
	KindCompleted:  "completed",
	KindFailed:     "failed",
}

func (k Kind) String() string {
	name, ok := kindNames[k]
	if !ok {
		return "unknown"
	}
	return name
}

// cd-backend が送る、デプロイを始めたことのイベント
//   - infra の modules/notification と、cd-backend の notify-deploy-started.sh と同じ値にする
const (
	StartedSource     = "amber.cd-backend"
	StartedDetailType = "Deployment Started"
)

// ECS のデプロイの状態の変化のイベント
//
// DOC: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html
const (
	ECSSource     = "aws.ecs"
	ECSDetailType = "ECS Deployment State Change"

	ECSInProgress = "SERVICE_DEPLOYMENT_IN_PROGRESS"
	ECSCompleted  = "SERVICE_DEPLOYMENT_COMPLETED"
	ECSFailed     = "SERVICE_DEPLOYMENT_FAILED"
)

// ロールバックを表す reason の文言
//   - ロールバックも SERVICE_DEPLOYMENT_IN_PROGRESS で届き、違いは reason だけ
//     (例: ECS deployment circuit breaker: rolling back to deploymentId ecs-svc/123.)
//   - サーキットブレーカーとアラームのどちらのロールバックも拾えるよう、前半ではなく "rolling back" で見分ける
//   - 大文字と小文字は区別しない
//
// DOC: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html
const rollbackReason = "rolling back"

// ECS のイベントの名前ごとの、通知の種類 (進行中は、reason でロールバックかを見分け直す)
var ecsKinds = map[string]Kind{
	ECSInProgress: KindInProgress,
	ECSCompleted:  KindCompleted,
	ECSFailed:     KindFailed,
}

// ErrUnsupported は、通知の対象ではないイベント
var ErrUnsupported = errors.New("unsupported event")

// Event は、届いたイベントを、通知に使う形に直したもの
type Event struct {
	ID             string    // EventBridge のイベントの ID (再試行で同じイベントが届いたことを見分ける)
	Kind           Kind      // 通知の種類
	DeploymentID   string    // デプロイ ID (ecs-svc/...)
	Service        string    // サービスの名前
	Time           time.Time // イベントの時刻
	Reason         string    // ECS のイベントだけが持つ
	Version        string    // Deployment Started だけが持つ (以下も同じ)
	Actor          string    // 起動した人
	RunURL         string    // GitHub Actions の実行の URL
	TaskDefinition string    // タスク定義の ARN
	Pushed         bool      // この実行でイメージを push したか (false なら、同じタグの流し直し)
}

// cd-backend の notify-deploy-started.sh が送る detail
type startedDetail struct {
	Version        string `json:"version"`
	DeploymentID   string `json:"deploymentId"`
	TaskDefinition string `json:"taskDefinition"`
	Pushed         string `json:"pushed"` // "true" / "false" の文字列
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
func ParseEvent(raw events.CloudWatchEvent, defaultService string) (*Event, error) {
	if raw.ID == "" {
		return nil, errors.New("event has no id")
	}

	ev, err := parseByType(raw, &Event{ID: raw.ID, Time: raw.Time, Service: defaultService})
	if err != nil {
		return nil, err
	}
	if ev.DeploymentID == "" {
		return nil, fmt.Errorf("%s event %s has no deploymentId", ev.Kind, ev.ID)
	}
	return ev, nil
}

// parseByType は、イベントの出どころと種類ごとに、detail を読む
func parseByType(raw events.CloudWatchEvent, ev *Event) (*Event, error) {
	switch {
	case raw.Source == StartedSource && raw.DetailType == StartedDetailType:
		return parseStarted(raw, ev)
	case raw.Source == ECSSource && raw.DetailType == ECSDetailType:
		return parseECS(raw, ev)
	default:
		return nil, fmt.Errorf("%w: %s / %s", ErrUnsupported, raw.Source, raw.DetailType)
	}
}

// parseStarted は、cd-backend の Deployment Started を読む
func parseStarted(raw events.CloudWatchEvent, ev *Event) (*Event, error) {
	var d startedDetail
	if err := json.Unmarshal(raw.Detail, &d); err != nil {
		return nil, fmt.Errorf("decode %s detail: %w", raw.DetailType, err)
	}

	ev.Kind = KindStarted
	ev.DeploymentID = d.DeploymentID
	ev.Version = d.Version
	ev.Actor = d.Actor
	ev.RunURL = d.RunURL
	ev.TaskDefinition = d.TaskDefinition
	ev.Pushed = d.Pushed == "true"
	return ev, nil
}

// parseECS は、ECS の ECS Deployment State Change を読む
func parseECS(raw events.CloudWatchEvent, ev *Event) (*Event, error) {
	var d ecsDetail
	if err := json.Unmarshal(raw.Detail, &d); err != nil {
		return nil, fmt.Errorf("decode %s detail: %w", raw.DetailType, err)
	}

	kind, ok := ecsKinds[d.EventName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, d.EventName)
	}

	ev.Kind = ecsKind(kind, d.Reason)
	ev.DeploymentID = d.DeploymentID
	ev.Reason = d.Reason
	if service := serviceFromARN(raw.Resources); service != "" {
		ev.Service = service
	}
	return ev, nil
}

// ecsKind は、進行中のうち、reason がロールバックを表すものをロールバックにする
func ecsKind(kind Kind, reason string) Kind {
	if kind != KindInProgress {
		return kind
	}
	if !strings.Contains(strings.ToLower(reason), rollbackReason) {
		return kind
	}
	return KindRollback
}

// serviceFromARN は、resources[0] のサービスの ARN (arn:aws:ecs:<region>:<account>:service/<cluster>/<service>) から、サービスの名前を取り出す
func serviceFromARN(resources []string) string {
	if len(resources) == 0 {
		return ""
	}
	arn := resources[0]
	i := strings.LastIndex(arn, "/")
	if i < 0 || i == len(arn)-1 {
		return ""
	}
	return arn[i+1:]
}
