package notifier

import (
	"context"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// 完了と失敗で、親に付けるスタンプ
var reactions = map[deploy.Kind]string{
	deploy.KindCompleted: "white_check_mark",
	deploy.KindFailed:    "x",
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
