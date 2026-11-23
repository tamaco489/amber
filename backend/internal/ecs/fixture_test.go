package ecs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// テストで使う値
const (
	testServiceARN = "arn:aws:ecs:ap-northeast-1:111122223333:service/prd-amber/prd-amber-api"
	testDeployment = "ecs-svc/1234567890"
)

// brief は、対象のサービスリビジョンの ID が revisionID、ID が id のサービスデプロイを作る
func brief(revisionID, id string) types.ServiceDeploymentBrief {
	return types.ServiceDeploymentBrief{
		ServiceDeploymentArn:     aws.String("arn:aws:ecs:ap-northeast-1:111122223333:service-deployment/prd-amber/prd-amber-api/" + id),
		TargetServiceRevisionArn: aws.String("arn:aws:ecs:ap-northeast-1:111122223333:service-revision/prd-amber/prd-amber-api/" + revisionID),
	}
}
