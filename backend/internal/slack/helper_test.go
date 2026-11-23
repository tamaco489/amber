package slack_test

import (
	"strings"
	"time"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// headerOf は、先頭のブロック (見出し) の文
func headerOf(m slack.Message) string {
	if len(m.Attachments) == 0 || len(m.Attachments[0].Blocks) == 0 || m.Attachments[0].Blocks[0].Text == nil {
		return ""
	}
	return m.Attachments[0].Blocks[0].Text.Text
}

// colorOf は、帯の色
func colorOf(m slack.Message) string {
	if len(m.Attachments) == 0 {
		return ""
	}
	return m.Attachments[0].Color
}

// bodyOf は、ブロックの mrkdwn の文をつなげたもの (項目の値を確かめるため)
func bodyOf(m slack.Message) string {
	var b strings.Builder
	for _, a := range m.Attachments {
		for _, blk := range a.Blocks {
			writeBlock(&b, blk)
		}
	}
	return b.String()
}

func writeBlock(b *strings.Builder, blk slack.Block) {
	if blk.Text != nil {
		writeLine(b, blk.Text.Text)
	}
	for _, f := range blk.Fields {
		writeLine(b, f.Text)
	}
	for _, e := range blk.Elements {
		writeLine(b, e.Text)
	}
}

func writeLine(b *strings.Builder, s string) {
	b.WriteString(s)
	b.WriteByte('\n')
}

// テストで使う値
const (
	testChannel    = "C0TEST"
	testService    = "prd-amber-api"
	testDeployment = "ecs-svc/1234567890"
	testParentTS   = "1700000000.000001"
	testTaskDef    = "arn:aws:ecs:ap-northeast-1:111122223333:task-definition/prd-amber-api:7"
	testRunURL     = "https://github.com/tamaco489/amber/actions/runs/1"
)

// testTime は、テストの時刻 (JST では 2026-11-23 11:12:30)
var testTime = time.Date(2026, 11, 23, 2, 12, 30, 0, time.UTC)

// startedEvent は、開始のイベントの値
func startedEvent(pushed bool) *deploy.Event {
	return &deploy.Event{
		ID:             "ev-started",
		Kind:           deploy.KindStarted,
		DeploymentID:   testDeployment,
		Service:        testService,
		Time:           testTime,
		Version:        "v3",
		Actor:          "tamaco489",
		RunURL:         testRunURL,
		TaskDefinition: testTaskDef,
		Pushed:         pushed,
	}
}

// ecsEvent は、ECS のイベントの値
func ecsEvent(kind deploy.Kind, reason string) *deploy.Event {
	return &deploy.Event{
		ID:           "ev-ecs",
		Kind:         kind,
		DeploymentID: testDeployment,
		Service:      testService,
		Time:         testTime,
		Reason:       reason,
	}
}
