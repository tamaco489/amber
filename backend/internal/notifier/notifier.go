package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/tamaco489/amber/backend/internal/slack"
)

// ErrBusy は、同じ手順をほかの呼び出しが進めていること
//   - エラーとして返し、Lambda の再試行で、済んだかを確かめ直す
var ErrBusy = errors.New("step is in progress in another invocation")

// 親の書き換えで、ほかの呼び出しとぶつかったときに、読み直して書き直す回数
const maxParentWriteAttempts = 3

// Slack は、Notifier が使う Slack の Web API (テストで差し替える)
type Slack interface {
	PostMessage(ctx context.Context, msg slack.Message) (string, error)
	UpdateMessage(ctx context.Context, msg slack.Message) error
	AddReaction(ctx context.Context, channel, ts, name string) error
}

// Notifier は、イベントを受けて Slack に通知する
type Notifier struct {
	Slack Slack
	Store Store

	// 投稿するチャンネルの ID
	Channel string
	// Deployment Started (サービスの名前を持たない) で使う、サービスの名前
	Service string

	// 今の時刻 (テストで差し替える)
	Now func() time.Time
}

func (n *Notifier) now() time.Time {
	if n.Now != nil {
		return n.Now()
	}
	return time.Now()
}

// Handle は、Lambda のハンドラー。1 回の呼び出しで 1 つのイベントを処理する
//   - 通知の対象でないイベントは、ログに残して成功として返す (再試行しても変わらないため)
//   - それ以外の失敗はエラーとして返し、Lambda の非同期呼び出しの再試行と、失敗時の送り先に乗せる
func (n *Notifier) Handle(ctx context.Context, raw events.CloudWatchEvent) error {
	ev, err := ParseEvent(raw, n.Service)
	if errors.Is(err, ErrUnsupported) {
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
	err = n.step(ctx, ev.ID+"#reply", func() error {
		_, err := n.Slack.PostMessage(ctx, replyMessage(ev, parent.Channel, parent.TS))
		return err
	})
	if err != nil {
		return err
	}

	// 3. 親の書き換え
	//   - このイベントで親を作ったときは、作るときに反映済みのため書き換えない
	//   - ロールバックでない進行中は、親を変えない
	if !created && ev.Kind != KindInProgress {
		err = n.step(ctx, ev.ID+"#parent", func() error {
			return n.updateParent(ctx, ev)
		})
		if err != nil {
			return err
		}
	}

	// 4. スタンプ
	if name := reactionFor(ev.Kind); name != "" {
		err = n.step(ctx, ev.ID+"#reaction", func() error {
			return n.Slack.AddReaction(ctx, parent.Channel, parent.TS, name)
		})
		if err != nil {
			return err
		}
	}

	log.InfoContext(ctx, "handled event", "parent_ts", parent.TS, "parent_created", created)
	return nil
}

// reactionFor は、親に付けるスタンプ (完了と失敗だけ)
func reactionFor(k Kind) string {
	switch k {
	case KindCompleted:
		return "white_check_mark"
	case KindFailed:
		return "x"
	}
	return ""
}

// step は、手順に印を付けてから fn を実行し、済んだ印を付ける
//   - 済んでいれば fn を実行しない (同じイベントの再配送や、途中まで成功したあとの再試行で、同じ投稿をしない)
//   - fn が失敗したら印を外し、次の再試行ですぐに進め直せるようにする
func (n *Notifier) step(ctx context.Context, key string, fn func() error) error {
	res, err := n.Store.Claim(ctx, key, n.now())
	if err != nil {
		return err
	}
	switch res {
	case ClaimDone:
		return nil
	case ClaimBusy:
		return fmt.Errorf("%s: %w", key, ErrBusy)
	}

	if err := fn(); err != nil {
		if rerr := n.Store.Release(ctx, key); rerr != nil {
			slog.WarnContext(ctx, "failed to release step", "key", key, "error", rerr)
		}
		return fmt.Errorf("%s: %w", key, err)
	}
	return n.Store.Done(ctx, key, n.now())
}

// ensureParent は、デプロイの親の投稿を返す。まだ無ければ、このイベントから作って投稿する
//   - 親を作る手順はデプロイ ID ごとに 1 つで、印を付けられた呼び出しだけが投稿する (親が 2 つできないようにする)
//   - 作ったときは created を true で返す
func (n *Notifier) ensureParent(ctx context.Context, ev Event) (Parent, bool, error) {
	p, ok, err := n.Store.GetParent(ctx, ev.DeploymentID)
	if err != nil || ok {
		return p, false, err
	}

	key := "parent#" + ev.DeploymentID
	res, err := n.Store.Claim(ctx, key, n.now())
	if err != nil {
		return Parent{}, false, err
	}
	switch res {
	case ClaimBusy:
		return Parent{}, false, fmt.Errorf("%s: %w", key, ErrBusy)
	case ClaimDone:
		p, ok, err := n.Store.GetParent(ctx, ev.DeploymentID)
		if err == nil && !ok {
			err = fmt.Errorf("%s is done but the parent is missing", key)
		}
		return p, false, err
	}

	// 印を付けるまでのあいだに、ほかの呼び出しが作り終えていることがある
	if p, ok, err := n.Store.GetParent(ctx, ev.DeploymentID); err != nil || ok {
		if ok {
			err = n.Store.Done(ctx, key, n.now())
		}
		return p, false, err
	}

	p, _ = applyEvent(Parent{
		DeploymentID: ev.DeploymentID,
		Channel:      n.Channel,
		Service:      ev.Service,
		Status:       StatusDeploying,
	}, ev)

	ts, err := n.Slack.PostMessage(ctx, parentMessage(p))
	if err != nil {
		if rerr := n.Store.Release(ctx, key); rerr != nil {
			slog.WarnContext(ctx, "failed to release step", "key", key, "error", rerr)
		}
		return Parent{}, false, fmt.Errorf("%s: %w", key, err)
	}

	p.TS = ts
	p.Rev = 1
	if err := n.Store.PutParent(ctx, p, n.now()); err != nil {
		return Parent{}, false, err
	}
	if err := n.Store.Done(ctx, key, n.now()); err != nil {
		return Parent{}, false, err
	}
	return p, true, nil
}

// updateParent は、イベントを親の状態に反映して保存し、親の投稿を書き換える
//   - 保存したあと chat.update の前に止まっても、再試行で同じ状態のまま chat.update をやり直す
func (n *Notifier) updateParent(ctx context.Context, ev Event) error {
	for range maxParentWriteAttempts {
		cur, ok, err := n.Store.GetParent(ctx, ev.DeploymentID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("parent of %s is missing", ev.DeploymentID)
		}

		next, changed := applyEvent(cur, ev)
		if changed {
			next.Rev = cur.Rev + 1
			err := n.Store.PutParent(ctx, next, n.now())
			if errors.Is(err, ErrConflict) {
				continue
			}
			if err != nil {
				return err
			}
		}
		return n.Slack.UpdateMessage(ctx, parentMessage(next))
	}
	return fmt.Errorf("update parent of %s: %w", ev.DeploymentID, ErrConflict)
}

// applyEvent は、イベントを親の状態に反映する。変わったかどうかも返す
//   - 開始: バージョン、起動した人、リンク、タスク定義を埋める
//   - ロールバック・完了・失敗: 状態を進める (戻す方向には変えない)
func applyEvent(p Parent, ev Event) (Parent, bool) {
	changed := false
	set := func(dst *string, v string) {
		if v != "" && *dst != v {
			*dst = v
			changed = true
		}
	}
	advance := func(s Status) bool {
		if s.rank() > p.Status.rank() {
			p.Status = s
			changed = true
			return true
		}
		return false
	}

	set(&p.Service, ev.Service)
	switch ev.Kind {
	case KindStarted:
		set(&p.Version, ev.Version)
		set(&p.Actor, ev.Actor)
		set(&p.RunURL, ev.RunURL)
		set(&p.TaskDefinition, ev.TaskDefinition)
	case KindRollback:
		advance(StatusRollingBack)
	case KindCompleted:
		if advance(StatusCompleted) {
			p.CompletedAt = ev.Time.UTC().Truncate(time.Second)
		}
	case KindFailed:
		advance(StatusFailed)
	case KindInProgress:
	}
	return p, changed
}
