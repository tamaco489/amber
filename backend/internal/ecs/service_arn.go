package ecs

import (
	"fmt"
	"strings"
)

// serviceARN は、サービスの ARN を分けたもの
//   - 形式: arn:aws:ecs:<region>:<account>:service/<cluster>/<service>
//
// DOC: https://docs.aws.amazon.com/service-authorization/latest/reference/list_amazonelasticcontainerservice.html#amazonelasticcontainerservice-resources-for-iam-policies
type serviceARN struct {
	arn     string // ARN のそのまま
	region  string // リージョン
	cluster string // クラスターの名前
	service string // サービスの名前
}

// parseServiceARN は、サービスの ARN を分ける。形式が違えばエラーを返す
func parseServiceARN(arn string) (*serviceARN, error) {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) != 6 || parts[0] != "arn" || parts[2] != "ecs" || parts[3] == "" {
		return nil, fmt.Errorf("not an ecs service arn: %q", arn)
	}

	path := strings.Split(parts[5], "/")
	if len(path) != 3 || path[0] != "service" || path[1] == "" || path[2] == "" {
		return nil, fmt.Errorf("not an ecs service arn (want service/<cluster>/<service>): %q", arn)
	}
	return &serviceARN{arn: arn, region: parts[3], cluster: path[1], service: path[2]}, nil
}
