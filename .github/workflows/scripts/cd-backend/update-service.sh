#!/usr/bin/env bash
# 登録したリビジョンでデプロイを始め、新しいデプロイの ID を GITHUB_OUTPUT の deployment_id に書く (cd-backend から呼ぶ)
#   - 方式 (ローリング、ブルー/グリーンなど) は Terraform のサービスの設定に従う
#   - デプロイの完了は待たない
#
# 環境変数:
#   - ECS_CLUSTER: ECS のクラスター名
#   - ECS_SERVICE: ECS のサービス名
#   - TASK_DEFINITION: デプロイするタスク定義の ARN
#   - GITHUB_OUTPUT: ステップの出力を書くファイル (GitHub Actions が渡す)
set -euo pipefail

# JMESPath の `PRIMARY` (リテラル) を展開させないため、--query はわざとシングルクォートで囲む
# shellcheck disable=SC2016
deployment_id=$(aws ecs update-service --cluster "$ECS_CLUSTER" --service "$ECS_SERVICE" \
    --task-definition "$TASK_DEFINITION" \
    --query 'service.deployments[?status==`PRIMARY`] | [0].id' --output text)

echo "started ${deployment_id} (${TASK_DEFINITION})"
echo "deployment_id=${deployment_id}" >> "$GITHUB_OUTPUT"
