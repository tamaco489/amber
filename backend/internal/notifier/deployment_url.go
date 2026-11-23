package notifier

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/ecs"
)

// ECS のデプロイのページの URL を、見つからないときに引き直す回数と間隔
//   - 進行中のイベントは、サービスデプロイが ListServiceDeployments に出るより先に届く (検証では 2〜6 秒先)
//   - 最大で 9 秒待つ。Lambda の時間切れ (30 秒) には十分に余裕がある
const (
	deploymentURLAttempts = 4
	deploymentURLInterval = 3 * time.Second
)

// lookupDeploymentURL は、ECS のイベントのとき、ECS のデプロイのページの URL を引いて、イベントに入れる
//   - Deployment Started はサービスの ARN を持たないため引かない。親には、あとから届く ECS のイベントで入る
//   - 引けなくても通知は止めない。ログに残し、親のリンクは「ECS deployment: none」のままにする (次の ECS のイベントで引き直す)
func (n *Notifier) lookupDeploymentURL(ctx context.Context, ev *deploy.Event) {
	if ev.ServiceARN == "" {
		return
	}

	url, err := n.findDeploymentURL(ctx, ev)
	if err != nil {
		slog.WarnContext(ctx, "failed to look up ecs deployment url", "id", ev.ID, "deployment_id", ev.DeploymentID, "error", err)
		return
	}
	ev.ECSDeploymentURL = url
}

// findDeploymentURL は、URL を引く。見つからない (ecs.ErrNotFound) 間だけ、間を空けて引き直す
//   - 見つからない以外のエラー (権限が無いなど) は、引き直しても変わらないため、すぐに返す
func (n *Notifier) findDeploymentURL(ctx context.Context, ev *deploy.Event) (string, error) {
	url, err := n.ECS.DeploymentURL(ctx, ev)
	for range deploymentURLAttempts - 1 {
		if !errors.Is(err, ecs.ErrNotFound) {
			return url, err
		}
		if err := n.sleep(ctx, deploymentURLInterval); err != nil {
			return "", err
		}
		url, err = n.ECS.DeploymentURL(ctx, ev)
	}
	return url, err
}
