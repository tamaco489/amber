package notifier

import "github.com/tamaco489/amber/backend/internal/deploy"

// 起動した人へのメンションの通知は、1 回のデプロイで 1 回だけにする (通知が何度も届かないようにするため)
//   - 親を作る時点で起動した人が分かれば (開始が先に届いたとき)、親の投稿でメンションする
//   - 分からなければ (進行中が先に届いたとき)、開始の返信でメンションする。あとから chat.update で親に起動した人を埋めても、メンションの通知は届かないため
//   - 親を書き換える (chat.update) ときも、親に起動した人が入っていれば、本文の先頭のメンションを付け直す。あとから見て、誰のデプロイか分かるようにするため
//   - ほかの返信ではメンションしない
//   - 対応表に無い人と、起動した人が分からないデプロイ (GitHub から始めていないもの) は、メンションしない

// parentMention は、親の投稿 (投稿と書き換えの両方) でメンションする Slack のユーザー ID を返す。メンションしないときは空
func (n *Notifier) parentMention(p *deploy.Parent) string {
	return n.UserIDs.Lookup(p.Actor)
}

// replyMention は、返信でメンションする Slack のユーザー ID を返す。メンションしないときは空
//   - 開始の返信は、親の書き換えより先に送る。そのため、親に起動した人が入っていなければ、親は起動した人が分かる前に作られている (親ではメンションしていない)
func (n *Notifier) replyMention(ev *deploy.Event, parent *deploy.Parent) string {
	if ev.Kind != deploy.KindStarted || parent.Actor != "" {
		return ""
	}
	return n.UserIDs.Lookup(ev.Actor)
}
