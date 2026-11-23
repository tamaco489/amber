package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/dynamodb"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// ErrBusy は、同じ手順をほかの呼び出しが進めていること
//   - エラーとして返し、Lambda の再試行で、済んだかを確かめ直す
var ErrBusy = errors.New("step is in progress in another invocation")

// 親の書き換えで、ほかの呼び出しとぶつかったときに、読み直して書き直す回数
const maxParentWriteAttempts = 3

// 完了と失敗で、親に付けるスタンプ
var reactions = map[deploy.Kind]string{
	deploy.KindCompleted: "white_check_mark",
	deploy.KindFailed:    "x",
}

// Notifier は、イベントを受けて Slack に通知する
type Notifier struct {
	Slack   Slack            // Slack の Web API
	Store   Store            // 親の投稿と手順の記録を残す場所
	Channel string           // 投稿するチャンネルの ID
	Service string           // Deployment Started (サービスの名前を持たない) で使う、サービスの名前
	Now     func() time.Time // 今の時刻 (テストで差し替える。nil なら time.Now)
}

func (n *Notifier) now() time.Time {
	if n.Now == nil {
		return time.Now()
	}
	return n.Now()
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

// reply は、イベントをスレッドに返信する
func (n *Notifier) reply(ctx context.Context, ev *deploy.Event, parent *deploy.Parent) error {
	return n.step(ctx, ev.ID+"#reply", func() error {
		_, err := n.Slack.PostMessage(ctx, slack.ReplyMessage(ev, parent.Channel, parent.TS))
		return err
	})
}

// rewriteParent は、イベントを親に反映して書き換える
//   - このイベントで親を作ったときは、作るときに反映済みのため書き換えない
//   - ロールバックでない進行中は、親を変えない
func (n *Notifier) rewriteParent(ctx context.Context, ev *deploy.Event, created bool) error {
	if created || ev.Kind == deploy.KindInProgress {
		return nil
	}
	return n.step(ctx, ev.ID+"#parent", func() error {
		return n.updateParent(ctx, ev)
	})
}

// react は、完了と失敗のとき、親にスタンプを付ける
func (n *Notifier) react(ctx context.Context, ev *deploy.Event, parent *deploy.Parent) error {
	name, ok := reactions[ev.Kind]
	if !ok {
		return nil
	}
	return n.step(ctx, ev.ID+"#reaction", func() error {
		return n.Slack.AddReaction(ctx, parent.Channel, parent.TS, name)
	})
}

// step は、手順に印を付けてから fn を実行し、済んだ印を付ける
//   - 済んでいれば fn を実行しない (同じイベントの再配送や、途中まで成功したあとの再試行で、同じ投稿をしない)
//   - fn が失敗したら印を外し、次の再試行ですぐに進め直せるようにする
func (n *Notifier) step(ctx context.Context, key string, fn func() error) error {
	res, err := n.Store.Claim(ctx, key, n.now())
	if err != nil {
		return err
	}
	if res == dynamodb.ClaimDone {
		return nil
	}
	if res == dynamodb.ClaimBusy {
		return fmt.Errorf("%s: %w", key, ErrBusy)
	}

	if err := fn(); err != nil {
		n.release(ctx, key)
		return fmt.Errorf("%s: %w", key, err)
	}
	return n.Store.Done(ctx, key, n.now())
}

// release は、手順の印を外す。外せなくても、印の期限が切れれば進め直せるため、ログに残すだけにする
func (n *Notifier) release(ctx context.Context, key string) {
	if err := n.Store.Release(ctx, key); err != nil {
		slog.WarnContext(ctx, "failed to release step", "key", key, "error", err)
	}
}

// ensureParent は、デプロイの親の投稿を返す。まだ無ければ、このイベントから作って投稿する
//   - 親を作る手順はデプロイ ID ごとに 1 つで、印を付けられた呼び出しだけが投稿する (親が 2 つできないようにする)
//   - 作ったときは created を true で返す
func (n *Notifier) ensureParent(ctx context.Context, ev *deploy.Event) (*deploy.Parent, bool, error) {
	p, err := n.Store.GetParent(ctx, ev.DeploymentID)
	if err != nil {
		return nil, false, err
	}
	if p != nil {
		return p, false, nil
	}

	key := "parent#" + ev.DeploymentID
	res, err := n.Store.Claim(ctx, key, n.now())
	if err != nil {
		return nil, false, err
	}
	if res == dynamodb.ClaimBusy {
		return nil, false, fmt.Errorf("%s: %w", key, ErrBusy)
	}
	if res == dynamodb.ClaimDone {
		p, err := n.existingParent(ctx, ev.DeploymentID, key)
		return p, false, err
	}

	// 印を付けるまでのあいだに、ほかの呼び出しが作り終えていることがある
	p, err = n.Store.GetParent(ctx, ev.DeploymentID)
	if err != nil {
		return nil, false, err
	}
	if p != nil {
		return p, false, n.Store.Done(ctx, key, n.now())
	}

	p, err = n.postParent(ctx, ev, key)
	if err != nil {
		return nil, false, err
	}
	return p, true, nil
}

// existingParent は、親を作る手順が済んでいるときに、残っている親を読む
func (n *Notifier) existingParent(ctx context.Context, deploymentID, key string) (*deploy.Parent, error) {
	p, err := n.Store.GetParent(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("%s is done but the parent is missing", key)
	}
	return p, nil
}

// postParent は、このイベントから親を組み立てて投稿し、残す
func (n *Notifier) postParent(ctx context.Context, ev *deploy.Event, key string) (*deploy.Parent, error) {
	p := &deploy.Parent{
		DeploymentID: ev.DeploymentID,
		Channel:      n.Channel,
		Service:      ev.Service,
		Status:       deploy.StatusDeploying,
	}
	p.Apply(ev)

	ts, err := n.Slack.PostMessage(ctx, slack.ParentMessage(p))
	if err != nil {
		n.release(ctx, key)
		return nil, fmt.Errorf("%s: %w", key, err)
	}

	p.TS = ts
	p.Rev = 1
	if err := n.Store.PutParent(ctx, p, n.now()); err != nil {
		return nil, err
	}
	if err := n.Store.Done(ctx, key, n.now()); err != nil {
		return nil, err
	}
	return p, nil
}

// updateParent は、イベントを親の状態に反映して保存し、親の投稿を書き換える
//   - 保存したあと chat.update の前に止まっても、再試行で同じ状態のまま chat.update をやり直す
//   - ほかの呼び出しと書き込みがぶつかったら、読み直して書き直す
func (n *Notifier) updateParent(ctx context.Context, ev *deploy.Event) error {
	for range maxParentWriteAttempts {
		next, err := n.saveParent(ctx, ev)
		if errors.Is(err, dynamodb.ErrConflict) {
			continue
		}
		if err != nil {
			return err
		}
		return n.Slack.UpdateMessage(ctx, slack.ParentMessage(next))
	}
	return fmt.Errorf("update parent of %s: %w", ev.DeploymentID, dynamodb.ErrConflict)
}

// saveParent は、今の親にイベントを反映し、変わっていれば保存する。反映したあとの親を返す
func (n *Notifier) saveParent(ctx context.Context, ev *deploy.Event) (*deploy.Parent, error) {
	cur, err := n.Store.GetParent(ctx, ev.DeploymentID)
	if err != nil {
		return nil, err
	}
	if cur == nil {
		return nil, fmt.Errorf("parent of %s is missing", ev.DeploymentID)
	}

	next := *cur
	if !next.Apply(ev) {
		return &next, nil
	}
	next.Rev = cur.Rev + 1
	if err := n.Store.PutParent(ctx, &next, n.now()); err != nil {
		return nil, err
	}
	return &next, nil
}
