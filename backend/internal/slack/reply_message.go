package slack

import "github.com/tamaco489/amber/backend/internal/deploy"

// ReplyMessage は、イベントごとのスレッドへの返信を組み立てる
func ReplyMessage(ev *deploy.Event, channel, threadTS string) Message {
	if ev.Kind == deploy.KindStarted {
		return startedReply(ev, channel, threadTS)
	}
	return ecsReply(ev, channel, threadTS)
}

// startedTitle は、デプロイを始めたことの返信の見出し
//   - 同じタグを流し直したとき (push していないとき) だけ、見出しで示す。古いバージョンに戻したことに気づけるようにするため
func startedTitle(ev *deploy.Event) string {
	if ev.Pushed {
		return ":rocket: Deploy started: " + ev.Version
	}
	return ":rocket: Redeploy started: " + ev.Version + " (existing image)"
}

// startedReply は、デプロイを始めたことの返信
func startedReply(ev *deploy.Event, channel, threadTS string) Message {
	title := startedTitle(ev)
	return message(channel, threadTS, title, ColorInfo,
		header(title),
		Block{Type: "section", Fields: []Text{field("Triggered by", ev.Actor)}},
		Block{Type: "section", Fields: []Text{
			field("Deployment ID", ev.DeploymentID),
			field("Task definition", ev.TaskDefinition),
		}},
		Block{Type: "section", Text: &Text{Type: "mrkdwn", Text: "*Run*\n" + escape(ev.RunURL)}},
	)
}

// replyLook は、返信の帯の色と見出し
type replyLook struct {
	color string // 帯の色
	title string // 見出し
}

// ECS のイベントの種類ごとの、返信の帯の色と見出し
//   - ロールバックは進行中と同じイベントだが、黄の進行中に紛れないよう赤にする
var ecsLooks = map[deploy.Kind]replyLook{
	deploy.KindInProgress: {color: ColorWarning, title: ":hourglass_flowing_sand: Deployment in progress"},
	deploy.KindRollback:   {color: ColorDanger, title: ":rewind: Rollback started"},
	deploy.KindCompleted:  {color: ColorSuccess, title: ":white_check_mark: Deployment completed"},
	deploy.KindFailed:     {color: ColorDanger, title: ":x: Deployment failed"},
}

// ecsReply は、ECS のデプロイの状態の変化の返信
//   - reason は省かずにそのまま載せる (失敗の原因を追う手がかりになるため)
func ecsReply(ev *deploy.Event, channel, threadTS string) Message {
	l := ecsLooks[ev.Kind]
	return message(channel, threadTS, l.title+": "+ev.Service, l.color,
		header(l.title),
		Block{Type: "section", Fields: []Text{
			field("Service", ev.Service),
			field("Deployment ID", ev.DeploymentID),
		}},
		Block{Type: "section", Text: &Text{Type: "mrkdwn", Text: "*Reason*\n" + escape(ev.Reason)}},
		Block{Type: "context", Elements: []Text{{Type: "mrkdwn", Text: FormatJST(ev.Time)}}},
	)
}
