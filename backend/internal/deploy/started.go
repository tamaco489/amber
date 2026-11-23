package deploy

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

// cd-backend が送る、デプロイを始めたことのイベント
//   - infra の modules/notification と、cd-backend の notify-deploy-started.sh と同じ値にする
const (
	StartedSource     = "amber.cd-backend"
	StartedDetailType = "Deployment Started"
)

// cd-backend の notify-deploy-started.sh が送る detail
type startedDetail struct {
	Version        string `json:"version"`
	DeploymentID   string `json:"deploymentId"`
	TaskDefinition string `json:"taskDefinition"`
	Pushed         string `json:"pushed"` // "true" / "false" の文字列
	Actor          string `json:"actor"`
	RunURL         string `json:"runUrl"`
}

// parseStarted は、cd-backend の Deployment Started を読む
func parseStarted(raw events.CloudWatchEvent, ev *Event) (*Event, error) {
	var d startedDetail
	if err := json.Unmarshal(raw.Detail, &d); err != nil {
		return nil, fmt.Errorf("decode %s detail: %w", raw.DetailType, err)
	}

	ev.Kind = KindStarted
	ev.DeploymentID = d.DeploymentID
	ev.Version = d.Version
	ev.Actor = d.Actor
	ev.RunURL = d.RunURL
	ev.TaskDefinition = d.TaskDefinition
	ev.Pushed = d.Pushed == "true"
	return ev, nil
}
