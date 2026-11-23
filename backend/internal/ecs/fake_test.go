package ecs

import (
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsecs "github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// fakeAPI は、決めたページを順に返す API (呼び出しの中身を残す)
type fakeAPI struct {
	pages [][]types.ServiceDeploymentBrief      // 返すページ (最後のページのほかは nextToken を付ける)
	err   error                                 // 呼び出しが返すエラー
	calls []*awsecs.ListServiceDeploymentsInput // 受け取った入力
}

func (f *fakeAPI) ListServiceDeployments(_ context.Context, in *awsecs.ListServiceDeploymentsInput, _ ...func(*awsecs.Options)) (*awsecs.ListServiceDeploymentsOutput, error) {
	f.calls = append(f.calls, in)
	if f.err != nil {
		return nil, f.err
	}

	page := 0
	if in.NextToken != nil {
		n, err := strconv.Atoi(*in.NextToken)
		if err != nil {
			return nil, err
		}
		page = n
	}
	if page >= len(f.pages) {
		return &awsecs.ListServiceDeploymentsOutput{}, nil
	}

	out := &awsecs.ListServiceDeploymentsOutput{ServiceDeployments: f.pages[page]}
	if page+1 < len(f.pages) {
		out.NextToken = aws.String(strconv.Itoa(page + 1))
	}
	return out, nil
}
