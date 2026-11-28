package ecs

import (
	"context"

	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
)

// API は、Client が使う ECS の操作 (テストで差し替える)
type API interface {
	// ListServiceDeployments は、サービスのサービスデプロイを一覧する
	ListServiceDeployments(ctx context.Context, in *awsecs.ListServiceDeploymentsInput, optFns ...func(*awsecs.Options)) (*awsecs.ListServiceDeploymentsOutput, error)
}
