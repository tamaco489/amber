package dynamodb

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

// parentToItem は、親の投稿をテーブルのアイテムにする
func parentToItem(p *deploy.Parent, now time.Time) map[string]types.AttributeValue {
	item := parentKey(p.DeploymentID)
	str := func(name, v string) {
		if v == "" {
			return
		}
		item[name] = &types.AttributeValueMemberS{Value: v}
	}
	str("ts", p.TS)
	str("channel", p.Channel)
	str("service", p.Service)
	str("status", string(p.Status))
	str("version", p.Version)
	str("actor", p.Actor)
	str("run_url", p.RunURL)
	str("task_definition", p.TaskDefinition)
	str("ecs_deployment_url", p.ECSDeploymentURL)
	if !p.CompletedAt.IsZero() {
		str("completed_at", p.CompletedAt.UTC().Format(time.RFC3339))
	}
	item["rev"] = &types.AttributeValueMemberN{Value: strconv.Itoa(p.Rev)}
	item["expires_at"] = unix(now.Add(itemTTL))
	return item
}

// parentFromItem は、テーブルのアイテムから親の投稿を組み立てる
func parentFromItem(deploymentID string, item map[string]types.AttributeValue) (*deploy.Parent, error) {
	str := func(name string) string {
		v, ok := item[name].(*types.AttributeValueMemberS)
		if !ok {
			return ""
		}
		return v.Value
	}

	p := &deploy.Parent{
		DeploymentID:     deploymentID,
		TS:               str("ts"),
		Channel:          str("channel"),
		Service:          str("service"),
		Status:           deploy.Status(str("status")),
		Version:          str("version"),
		Actor:            str("actor"),
		RunURL:           str("run_url"),
		TaskDefinition:   str("task_definition"),
		ECSDeploymentURL: str("ecs_deployment_url"),
	}

	completedAt, err := parseTime(str("completed_at"))
	if err != nil {
		return nil, fmt.Errorf("parent %s: completed_at: %w", deploymentID, err)
	}
	p.CompletedAt = completedAt

	rev, err := parseRev(item["rev"])
	if err != nil {
		return nil, fmt.Errorf("parent %s: rev: %w", deploymentID, err)
	}
	p.Rev = rev
	return p, nil
}

// parseTime は、RFC 3339 の時刻を読む。空ならゼロ値
func parseTime(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, v)
}

// parseRev は、rev を読む。無ければ 0
func parseRev(v types.AttributeValue) (int, error) {
	n, ok := v.(*types.AttributeValueMemberN)
	if !ok {
		return 0, nil
	}
	return strconv.Atoi(n.Value)
}
