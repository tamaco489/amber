package deploy

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-go/events"
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

// ECS の ECS Deployment State Change の detail
//
// DOC: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html
type ecsDetail struct {
	EventName    string `json:"eventName"`
	DeploymentID string `json:"deploymentId"`
	Reason       string `json:"reason"`
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
