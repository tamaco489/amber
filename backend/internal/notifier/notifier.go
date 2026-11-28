package notifier

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

// Notifier は、イベントを受けて Slack に通知する
type Notifier struct {
	Slack   Slack                                            // Slack の Web API
	Store   Store                                            // 親の投稿と手順の記録を残す場所
	ECS     ECS                                              // ECS の API (デプロイのページの URL を引く)
	Channel string                                           // 投稿するチャンネルの ID
	Service string                                           // Deployment Started (サービスの名前を持たない) で使う、サービスの名前
	Now     func() time.Time                                 // 今の時刻 (テストで差し替える。nil なら time.Now)
	Sleep   func(ctx context.Context, d time.Duration) error // 待つ (テストで差し替える。nil なら d だけ待つ)
}

func (n *Notifier) now() time.Time {
	if n.Now == nil {
		return time.Now()
	}
	return n.Now()
}

// sleep は、d だけ待つ。待つ間に ctx が終わったら、そのエラーを返す
func (n *Notifier) sleep(ctx context.Context, d time.Duration) error {
	if n.Sleep != nil {
		return n.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Handle は、Lambda のハンドラー。1 回の呼び出しで 1 つのイベントを処理する
//   - 通知の対象でないイベントは、ログに残して成功として返す (再試行しても変わらないため)
//   - それ以外の失敗はエラーとして返し、Lambda の非同期呼び出しの再試行と、失敗時の送り先に乗せる
func (n *Notifier) Handle(ctx context.Context, raw events.CloudWatchEvent) error {
	ev, err := deploy.ParseEvent(raw, n.Service)
	if errors.Is(err, deploy.ErrUnsupported) {
		slog.WarnContext(ctx, "skip unsupported event", "id", raw.ID, "error", err)
		return nil
	}
	if err != nil {
		return err
	}

	log := slog.With("id", ev.ID, "kind", ev.Kind.String(), "deployment_id", ev.DeploymentID)
	log.InfoContext(ctx, "handle event")

	// 0. ECS のデプロイのページの URL (引けなくても通知は続ける)
	n.lookupDeploymentURL(ctx, ev)

	// 1. 親の投稿
	parent, created, err := n.ensureParent(ctx, ev)
	if err != nil {
		return err
	}

	// 2. スレッドへの返信
	if err := n.reply(ctx, ev, parent); err != nil {
		return err
	}

	// 3. 親の書き換え
	if err := n.rewriteParent(ctx, ev, created); err != nil {
		return err
	}

	// 4. スタンプ
	if err := n.react(ctx, ev, parent); err != nil {
		return err
	}

	log.InfoContext(ctx, "handled event", "parent_ts", parent.TS, "parent_created", created)
	return nil
}
