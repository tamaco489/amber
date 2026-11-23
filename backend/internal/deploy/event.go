package deploy

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

// ErrUnsupported は、通知の対象ではないイベント
var ErrUnsupported = errors.New("unsupported event")

// Event は、届いたイベントを、通知に使う形に直したもの
type Event struct {
	ID               string    // EventBridge のイベントの ID (再試行で同じイベントが届いたことを見分ける)
	Kind             Kind      // 通知の種類
	DeploymentID     string    // デプロイ ID (ecs-svc/...)
	Service          string    // サービスの名前
	Time             time.Time // イベントの時刻
	Reason           string    // ECS のイベントだけが持つ (ServiceARN も同じ)
	ServiceARN       string    // サービスの ARN (resources[0])
	ECSDeploymentURL string    // ECS のデプロイのページの URL (ECS のイベントで、notifier が ECS の API で引いて入れる。引けなければ空)
	Version          string    // Deployment Started だけが持つ (以下も同じ)
	Actor            string    // 起動した人
	RunURL           string    // GitHub Actions の実行の URL
	TaskDefinition   string    // タスク定義の ARN
	Pushed           bool      // この実行でイメージを push したか (false なら、同じタグの流し直し)
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
