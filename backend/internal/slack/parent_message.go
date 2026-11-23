package slack

import (
	"fmt"
	"strings"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

// look は、帯の色と見出し
type look struct {
	color  string // 帯の色
	emoji  string // 見出しの先頭の絵文字
	verb   string // 見出しの動詞 (Deploying など)
	status string // Status の欄の文言
}

// 状態ごとの、親の帯の色と見出し
var parentLooks = map[deploy.Status]look{
	deploy.StatusDeploying:   {color: ColorNeutral, emoji: ":rocket:", verb: "Deploying", status: "Deploying"},
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
//   - ECS のデプロイのページを先に、GitHub Actions の実行を 2 つ目に並べる (ECS のリンクは、GitHub から始めていないデプロイでも付くため)
//   - URL が無いリンクは「<名前>: none」と出す (GitHub から始めていないデプロイ、ECS の URL を引けなかったときなど)
func parentLinks(p *deploy.Parent) []link {
	return []link{
		{label: "ECS deployment", url: p.ECSDeploymentURL},
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
		Block{Type: "section", Fields: parentFields(l, p)},
		Block{Type: "section", Text: &Text{Type: "mrkdwn", Text: strings.Join(links, "   ")}},
	)
	msg.TS = p.TS
	return msg
}
