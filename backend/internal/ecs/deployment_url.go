package ecs

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

// ErrNotFound は、イベントのデプロイに当たるサービスデプロイが見つからないこと
var ErrNotFound = errors.New("service deployment not found")

// サービスデプロイの一覧の引き方
//   - 1 ページの件数は、上限の 100 件にする
//   - 並び順がドキュメントに書かれていないため、決めたページ数まで探す
//
// DOC: https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListServiceDeployments.html
const (
	pageSize = 100
	maxPages = 5
)

// ECS のイベントの deploymentId の先頭
const deploymentIDPrefix = "ecs-svc/"

// DeploymentURL は、ECS のイベントのデプロイの、ECS のコンソールのデプロイのページの URL を返す
//   - サービスの ARN (ECS のイベントの resources[0]) から、クラスターを取り出して ListServiceDeployments を呼ぶ
//   - 見つからなければ ErrNotFound を返す
//
// DOC: https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListServiceDeployments.html
func (c *Client) DeploymentURL(ctx context.Context, ev *deploy.Event) (string, error) {
	svc, err := parseServiceARN(ev.ServiceARN)
	if err != nil {
		return "", err
	}
	revisionID, ok := strings.CutPrefix(ev.DeploymentID, deploymentIDPrefix)
	if !ok || revisionID == "" {
		return "", fmt.Errorf("unexpected deployment id: %q", ev.DeploymentID)
	}

	in := &awsecs.ListServiceDeploymentsInput{
		Cluster:    aws.String(svc.cluster),
		Service:    aws.String(svc.arn),
		MaxResults: aws.Int32(pageSize),
	}
	for range maxPages {
		out, err := c.api.ListServiceDeployments(ctx, in)
		if err != nil {
			return "", fmt.Errorf("list service deployments of %s: %w", svc.arn, err)
		}
		if id := findServiceDeployment(out.ServiceDeployments, revisionID); id != "" {
			return consoleURL(svc, id), nil
		}
		if out.NextToken == nil {
			break
		}
		in.NextToken = out.NextToken
	}
	return "", fmt.Errorf("%s of %s: %w", ev.DeploymentID, svc.arn, ErrNotFound)
}

// findServiceDeployment は、対象のサービスリビジョンの ID が revisionID のサービスデプロイを探し、その ID を返す。無ければ空
//   - サービスリビジョンの ARN: arn:aws:ecs:<region>:<account>:service-revision/<cluster>/<service>/<ID>
//   - サービスデプロイの ARN: arn:aws:ecs:<region>:<account>:service-deployment/<cluster>/<service>/<ID>
//
// DOC: https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ServiceDeploymentBrief.html
func findServiceDeployment(deployments []types.ServiceDeploymentBrief, revisionID string) string {
	for _, d := range deployments {
		if lastSegment(aws.ToString(d.TargetServiceRevisionArn)) != revisionID {
			continue
		}
		return lastSegment(aws.ToString(d.ServiceDeploymentArn))
	}
	return ""
}

// lastSegment は、ARN の最後の / より後ろを返す
func lastSegment(arn string) string {
	i := strings.LastIndex(arn, "/")
	return arn[i+1:]
}
