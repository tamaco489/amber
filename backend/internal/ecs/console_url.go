package ecs

import (
	"net/url"
	"strings"
)

// consoleURL は、ECS のコンソールの、サービスデプロイのページの URL を組む
//   - 形式: https://<region>.console.aws.amazon.com/ecs/v2/clusters/<cluster>/services/<service>/deployments/<サービスデプロイの ID>?region=<region>
//   - サービスデプロイの ID は、サービスデプロイの ARN (arn:aws:ecs:<region>:<account>:service-deployment/<cluster>/<service>/<ID>) の末尾
//   - コンソールの URL の形式は、AWS のドキュメントに書かれていない。コンソールで開くページの URL に合わせている (実際のデプロイで確かめる)
//
// DOC: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-deployment.html
func consoleURL(svc *serviceARN, serviceDeploymentID string) string {
	u := url.URL{
		Scheme: "https",
		Host:   svc.region + ".console.aws.amazon.com",
		Path: strings.Join([]string{
			"/ecs/v2/clusters", svc.cluster,
			"services", svc.service,
			"deployments", serviceDeploymentID,
		}, "/"),
		RawQuery: url.Values{"region": {svc.region}}.Encode(),
	}
	return u.String()
}
