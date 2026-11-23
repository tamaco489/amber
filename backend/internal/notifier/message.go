package notifier

import (
	"fmt"
	"strings"
	"time"

	"github.com/tamaco489/amber/backend/internal/slack"
)

// Slack の通知の色の帯 (Slack のブランドカラー。infra の modules/notification の slack_colors と同じ)
const (
	colorInfo    = "#36C5F0"
	colorSuccess = "#2EB67D"
	colorWarning = "#ECB22E"
	colorDanger  = "#E01E5A"
)

// 値が無いときに出す文言
const none = "none"

// 時刻は JST で出す
//   - Lambda の provided.al2023 にタイムゾーンのデータを持ち込まずに済むよう、固定の +09:00 を使う
var jst = time.FixedZone("JST", 9*60*60)

// formatJST は、時刻を 2026-11-23 11:12:30 (JST) の形にする
func formatJST(t time.Time) string {
	return t.In(jst).Format("2006-01-02 15:04:05") + " (JST)"
}

// mrkdwn の本文に埋め込む値を、Slack の決まりどおりにエスケープする (&、<、> だけ)
//
// DOC: https://docs.slack.dev/messaging/formatting-message-text#escaping
var mrkdwnEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func escape(s string) string {
	if s == "" {
		return none
	}
	return mrkdwnEscaper.Replace(s)
}

func field(name, value string) slack.Text {
	return slack.Text{Type: "mrkdwn", Text: "*" + name + "*\n" + escape(value)}
}

func header(text string) slack.Block {
	return slack.Block{Type: "header", Text: &slack.Text{Type: "plain_text", Text: text, Emoji: true}}
}

func message(channel, threadTS, text, color string, blocks ...slack.Block) slack.Message {
	return slack.Message{
		Channel:     channel,
		ThreadTS:    threadTS,
		Text:        text,
		Attachments: []slack.Attachment{{Color: color, Blocks: blocks}},
	}
}

// shortTaskDefinition は、タスク定義の ARN から <family>:<revision> を取り出す
func shortTaskDefinition(arn string) string {
	if i := strings.LastIndex(arn, "task-definition/"); i >= 0 {
		return arn[i+len("task-definition/"):]
	}
	return arn
}

// ================================================================================
// 親の投稿
// ================================================================================

// 状態ごとの、親の帯の色と見出し
var parentLooks = map[Status]struct {
	color  string
	emoji  string
	verb   string
	status string
}{
	StatusDeploying:   {color: colorInfo, emoji: ":rocket:", verb: "Deploying", status: "Deploying"},
	StatusRollingBack: {color: colorDanger, emoji: ":rewind:", verb: "Rolling back", status: "Rolling back"},
	StatusCompleted:   {color: colorSuccess, emoji: ":white_check_mark:", verb: "Deployed", status: "Completed"},
	StatusFailed:      {color: colorDanger, emoji: ":x:", verb: "Failed to deploy", status: "Failed"},
}

// link は、親の投稿の下に並べるリンク
type link struct {
	label string
	url   string
}

// parentLinks は、親の投稿に載せるリンクを返す
//   - URL が無いリンクは「<名前>: none」と出す (GitHub から始めていないデプロイなど)
//   - ECS のデプロイのページへのリンクは #55 で、ここの先頭に足す
func parentLinks(p Parent) []link {
	return []link{
		{label: "GitHub Actions run", url: p.RunURL},
	}
}

// parentMessage は、親の投稿の本文を組み立てる (chat.postMessage と chat.update の両方で使う)
func parentMessage(p Parent) slack.Message {
	look, ok := parentLooks[p.Status]
	if !ok {
		look = parentLooks[StatusDeploying]
	}

	// 進行中が先に届いたときは、バージョンが分からないため、見出しから外す
	title := fmt.Sprintf("%s %s: %s", look.emoji, look.verb, p.Service)
	if p.Version != "" {
		title = fmt.Sprintf("%s %s %s: %s", look.emoji, look.verb, p.Version, p.Service)
	}

	fields := []slack.Text{
		field("Service", p.Service),
		field("Status", look.status),
		field("Triggered by", p.Actor),
		field("Task definition", shortTaskDefinition(p.TaskDefinition)),
	}
	if !p.CompletedAt.IsZero() {
		fields = append(fields, field("Completed at", formatJST(p.CompletedAt)))
	}

	var links []string
	for _, l := range parentLinks(p) {
		if l.url == "" {
			links = append(links, l.label+": "+none)
			continue
		}
		links = append(links, "<"+l.url+"|"+l.label+">")
	}

	msg := message(p.Channel, "", title, look.color,
		header(title),
		slack.Block{Type: "section", Fields: fields},
		slack.Block{Type: "section", Text: &slack.Text{Type: "mrkdwn", Text: strings.Join(links, "   ")}},
	)
	msg.TS = p.TS
	return msg
}

// ================================================================================
// スレッドへの返信
// ================================================================================

// replyMessage は、イベントごとのスレッドへの返信を組み立てる
func replyMessage(ev Event, channel, threadTS string) slack.Message {
	if ev.Kind == KindStarted {
		return startedReply(ev, channel, threadTS)
	}
	return ecsReply(ev, channel, threadTS)
}

// startedReply は、デプロイを始めたことの返信
//   - 同じタグを流し直したとき (push していないとき) だけ、見出しで示す。古いバージョンに戻したことに気づけるようにするため
func startedReply(ev Event, channel, threadTS string) slack.Message {
	title := ":rocket: Deploy started: " + ev.Version
	if !ev.Pushed {
		title = ":rocket: Redeploy started: " + ev.Version + " (existing image)"
	}

	return message(channel, threadTS, title, colorInfo,
		header(title),
		slack.Block{Type: "section", Fields: []slack.Text{field("Triggered by", ev.Actor)}},
		slack.Block{Type: "section", Fields: []slack.Text{
			field("Deployment ID", ev.DeploymentID),
			field("Task definition", ev.TaskDefinition),
		}},
		slack.Block{Type: "section", Text: &slack.Text{Type: "mrkdwn", Text: "*Run*\n" + escape(ev.RunURL)}},
	)
}

// ECS のイベントの種類ごとの、返信の帯の色と見出し
//   - ロールバックは進行中と同じイベントだが、黄の進行中に紛れないよう赤にする
var ecsLooks = map[Kind]struct {
	color string
	title string
}{
	KindInProgress: {color: colorWarning, title: ":hourglass_flowing_sand: Deployment in progress"},
	KindRollback:   {color: colorDanger, title: ":rewind: Rollback started"},
	KindCompleted:  {color: colorSuccess, title: ":white_check_mark: Deployment completed"},
	KindFailed:     {color: colorDanger, title: ":x: Deployment failed"},
}

// ecsReply は、ECS のデプロイの状態の変化の返信
//   - reason は省かずにそのまま載せる (失敗の原因を追う手がかりになるため)
func ecsReply(ev Event, channel, threadTS string) slack.Message {
	look := ecsLooks[ev.Kind]

	return message(channel, threadTS, look.title+": "+ev.Service, look.color,
		header(look.title),
		slack.Block{Type: "section", Fields: []slack.Text{
			field("Service", ev.Service),
			field("Deployment ID", ev.DeploymentID),
		}},
		slack.Block{Type: "section", Text: &slack.Text{Type: "mrkdwn", Text: "*Reason*\n" + escape(ev.Reason)}},
		slack.Block{Type: "context", Elements: []slack.Text{{Type: "mrkdwn", Text: formatJST(ev.Time)}}},
	)
}
