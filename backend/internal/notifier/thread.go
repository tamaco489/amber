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
//   - ロールバックでない進行中は、ECS のデプロイのページの URL を引けたときだけ書き換える (開始が先に届いて作った親に、URL を足す)
//     親に URL が入っていても書き換える。保存したあと chat.update の前に止まったときに、再試行で chat.update をやり直せるようにするため
func (n *Notifier) rewriteParent(ctx context.Context, ev *deploy.Event, created bool) error {
	if created {
		return nil
	}
	if ev.Kind == deploy.KindInProgress && ev.ECSDeploymentURL == "" {
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
