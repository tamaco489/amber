package slack

import (
	"fmt"
	"strings"
	"time"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

// Slack の通知の色の帯 (Slack のブランドカラー。infra の modules/notification の slack_colors と同じ)
const (
	ColorInfo    = "#36C5F0"
	ColorSuccess = "#2EB67D"
	ColorWarning = "#ECB22E"
	ColorDanger  = "#E01E5A"
)

// 値が無いときに出す文言
const none = "none"

// 時刻は JST で出す
//   - Lambda の provided.al2023 にタイムゾーンのデータを持ち込まずに済むよう、固定の +09:00 を使う
var jst = time.FixedZone("JST", 9*60*60)

// FormatJST は、時刻を 2026-11-23 11:12:30 (JST) の形にする
func FormatJST(t time.Time) string {
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

func field(name, value string) Text {
	return Text{Type: "mrkdwn", Text: "*" + name + "*\n" + escape(value)}
}

func header(text string) Block {
	return Block{Type: "header", Text: &Text{Type: "plain_text", Text: text, Emoji: true}}
}

func message(channel, threadTS, text, color string, blocks ...Block) Message {
	return Message{
		Channel:     channel,
		ThreadTS:    threadTS,
		Text:        text,
		Attachments: []Attachment{{Color: color, Blocks: blocks}},
	}
}

// shortTaskDefinition は、タスク定義の ARN から <family>:<revision> を取り出す
func shortTaskDefinition(arn string) string {
	i := strings.LastIndex(arn, "task-definition/")
	if i < 0 {
		return arn
	}
	return arn[i+len("task-definition/"):]
}

// ================================================================================
// 親の投稿
// ================================================================================

// look は、帯の色と見出し
type look struct {
	color  string // 帯の色
	emoji  string // 見出しの先頭の絵文字
	verb   string // 見出しの動詞 (Deploying など)
	status string // Status の欄の文言
}

// 状態ごとの、親の帯の色と見出し
var parentLooks = map[deploy.Status]look{
	deploy.StatusDeploying:   {color: ColorInfo, emoji: ":rocket:", verb: "Deploying", status: "Deploying"},
	deploy.StatusRollingBack: {color: ColorDanger, emoji: ":rewind:", verb: "Rolling back", status: "Rolling back"},
	deploy.StatusCompleted:   {color: ColorSuccess, emoji: ":white_check_mark:", verb: "Deployed", status: "Completed"},
	deploy.StatusFailed:      {color: ColorDanger, emoji: ":x:", verb: "Failed to deploy", status: "Failed"},
}

// link は、親の投稿の下に並べるリンク
type link struct {
	label string // リンクの文言
	url   string // リンク先 (空なら「<文言>: none」と出す)
}

// parentLinks は、親の投稿に載せるリンクを返す
//   - URL が無いリンクは「<名前>: none」と出す (GitHub から始めていないデプロイなど)
//   - ECS のデプロイのページへのリンクは #55 で、ここの先頭に足す
func parentLinks(p *deploy.Parent) []link {
	return []link{
		{label: "GitHub Actions run", url: p.RunURL},
	}
}

// parentTitle は、親の見出し。進行中が先に届いたときは、バージョンが分からないため、見出しから外す
func parentTitle(l look, p *deploy.Parent) string {
	if p.Version == "" {
		return fmt.Sprintf("%s %s: %s", l.emoji, l.verb, p.Service)
	}
	return fmt.Sprintf("%s %s %s: %s", l.emoji, l.verb, p.Version, p.Service)
}

// parentFields は、親の 2 列の項目
func parentFields(l look, p *deploy.Parent) []Text {
	fields := []Text{
		field("Service", p.Service),
		field("Status", l.status),
		field("Triggered by", p.Actor),
		field("Task definition", shortTaskDefinition(p.TaskDefinition)),
	}
	if p.CompletedAt.IsZero() {
		return fields
	}
	return append(fields, field("Completed at", FormatJST(p.CompletedAt)))
}

// linkText は、リンクを mrkdwn にする
func linkText(l link) string {
	if l.url == "" {
		return l.label + ": " + none
	}
	return "<" + l.url + "|" + l.label + ">"
}

// ParentMessage は、親の投稿の本文を組み立てる (chat.postMessage と chat.update の両方で使う)
func ParentMessage(p *deploy.Parent) Message {
	l, ok := parentLooks[p.Status]
	if !ok {
		l = parentLooks[deploy.StatusDeploying]
	}
	title := parentTitle(l, p)

	var links []string
	for _, lk := range parentLinks(p) {
		links = append(links, linkText(lk))
	}

	msg := message(p.Channel, "", title, l.color,
		header(title),
		Block{Type: "section", Fields: parentFields(l, p)},
		Block{Type: "section", Text: &Text{Type: "mrkdwn", Text: strings.Join(links, "   ")}},
	)
	msg.TS = p.TS
	return msg
}

// ================================================================================
// スレッドへの返信
// ================================================================================

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
