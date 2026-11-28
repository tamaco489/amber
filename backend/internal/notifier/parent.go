package notifier

import (
	"context"
	"errors"
	"fmt"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/dynamodb"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// 親の書き換えで、ほかの呼び出しとぶつかったときに、読み直して書き直す回数
const maxParentWriteAttempts = 3

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

	ts, err := n.Slack.PostMessage(ctx, slack.WithMention(slack.ParentMessage(p), n.parentMention(p)))
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
