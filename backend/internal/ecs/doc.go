// Package ecs は、ECS の API のうち、デプロイの通知に使うものだけを呼ぶ
//   - ListServiceDeployments: ECS のイベントのデプロイから、サービスデプロイを引き、ECS のコンソールのそのデプロイのページの URL を組む
//
// サービスデプロイの ID は、ECS のイベントの deploymentId (ecs-svc/<数字>) とは別の ID で、ListServiceDeployments の応答は deploymentId を持たない。
// そのため、対象のサービスリビジョンの ARN の末尾が deploymentId の数字と同じサービスデプロイを、そのデプロイのものとみなす。
// この対応はドキュメントに書かれていないため、実際のデプロイで確かめる。
package ecs
