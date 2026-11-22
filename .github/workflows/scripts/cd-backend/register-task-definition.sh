#!/usr/bin/env bash
# サービスが現在使っているタスク定義を複製し、コンテナのイメージだけを差し替えた新しいリビジョンとして登録する (cd-backend から呼ぶ)
#   - CPU、メモリ、ロール、ログなど、イメージ以外の設定はそのまま引き継ぐ (最初のリビジョンは Terraform が作る)
#   - 登録したリビジョンの ARN を、GITHUB_OUTPUT の arn に書く
#
# 環境変数:
#   - ECS_CLUSTER: ECS のクラスター名
#   - ECS_SERVICE: ECS のサービス名
#   - CONTAINER_NAME: イメージを差し替えるコンテナの名前
#   - IMAGE: 新しいイメージ (<repository_url>@sha256:...)
#   - GITHUB_OUTPUT: ステップの出力を書くファイル (GitHub Actions が渡す)
set -euo pipefail

read -r status current < <(aws ecs describe-services \
    --cluster "$ECS_CLUSTER" --services "$ECS_SERVICE" \
    --query 'services[0].[status,taskDefinition]' --output text)

if [ "$status" != "ACTIVE" ]; then
    echo "::error::Service ${ECS_SERVICE} is not active (status: ${status}). Is enable_runtime set to true?"
    exit 1
fi

workdir=$(mktemp -d)
aws ecs describe-task-definition --task-definition "$current" \
    --query taskDefinition > "${workdir}/current.json"

# 登録の入力に使えない、ECS が付けた項目は消す
jq --arg name "$CONTAINER_NAME" --arg image "$IMAGE" '
    .containerDefinitions |= map(if .name == $name then .image = $image else . end)
    | del(.taskDefinitionArn, .revision, .status, .requiresAttributes, .compatibilities,
        .registeredAt, .registeredBy, .deregisteredAt)
' "${workdir}/current.json" > "${workdir}/new.json"

arn=$(aws ecs register-task-definition --cli-input-json "file://${workdir}/new.json" \
    --query 'taskDefinition.taskDefinitionArn' --output text)

echo "registered ${arn} (copied from ${current})"
echo "arn=${arn}" >> "$GITHUB_OUTPUT"
