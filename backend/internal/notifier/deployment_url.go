package notifier

import (
	"context"
	"log/slog"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

// lookupDeploymentURL は、ECS のイベントのとき、ECS のデプロイのページの URL を引いて、イベントに入れる
//   - Deployment Started はサービスの ARN を持たないため引かない。親には、あとから届く ECS のイベントで入る
//   - 引けなくても通知は止めない。ログに残し、親のリンクは「ECS deployment: none」のままにする (次の ECS のイベントで引き直す)
func (n *Notifier) lookupDeploymentURL(ctx context.Context, ev *deploy.Event) {
	if ev.ServiceARN == "" {
		return
	}

	url, err := n.ECS.DeploymentURL(ctx, ev)
	if err != nil {
		slog.WarnContext(ctx, "failed to look up ecs deployment url", "id", ev.ID, "deployment_id", ev.DeploymentID, "error", err)
		return
	}
	ev.ECSDeploymentURL = url
}
