#!/usr/bin/env bash
# ファミリーの最新の ACTIVE なタスク定義を複製し、コンテナのイメージだけを差し替えた新しいリビジョンとして登録する (cd-backend から呼ぶ)
#   - CPU、メモリ、ロール、ログなど、イメージ以外の設定はそのまま引き継ぐ
#   - 複製元をサービスが今使っているリビジョンではなくファミリーの最新にするのは、Terraform で変えた設定を次のデプロイで届けるため
#     (サービスは ignore_changes = [task_definition] のため、Terraform が作ったリビジョンには切り替わらない)
#   - Terraform が作るリビジョンのイメージは bootstrap のため、イメージはここで必ず差し替える
#   - 登録したリビジョンの ARN を、GITHUB_OUTPUT の arn に書く
#
# 環境変数:
#   - ECS_CLUSTER: ECS のクラスター名
#   - ECS_SERVICE: ECS のサービス名 (ACTIVE かの確認に使う)
#   - TASK_DEFINITION_FAMILY: 複製元のタスク定義のファミリー名 (番号を付けずに渡し、最新の ACTIVE なリビジョンを取る)
#   - CONTAINER_NAME: イメージを差し替えるコンテナの名前
#   - IMAGE: 新しいイメージ (<repository_url>@sha256:...)
#   - GITHUB_OUTPUT: ステップの出力を書くファイル (GitHub Actions が渡す)
set -euo pipefail

status=$(aws ecs describe-services \
    --cluster "$ECS_CLUSTER" --services "$ECS_SERVICE" \
    --query 'services[0].status' --output text)

if [ "$status" != "ACTIVE" ]; then
    echo "::error::Service ${ECS_SERVICE} is not active (status: ${status}). Is enable_runtime set to true?"
    exit 1
fi

workdir=$(mktemp -d)
aws ecs describe-task-definition --task-definition "$TASK_DEFINITION_FAMILY" \
    --query taskDefinition > "${workdir}/source.json"
source_arn=$(jq -r '.taskDefinitionArn' "${workdir}/source.json")

# 登録の入力に使えない、ECS が付けた項目は消す
jq \
    --arg name "$CONTAINER_NAME" \
    --arg image "$IMAGE" \
    '
    .containerDefinitions |= map(
        if .name == $name then
            .image = $image
        else
            .
        end
    )
    | del(
        .taskDefinitionArn,
        .revision,
        .status,
        .requiresAttributes,
        .compatibilities,
        .registeredAt,
        .registeredBy,
        .deregisteredAt
    )
    ' "${workdir}/source.json" > "${workdir}/new.json"

arn=$(aws ecs register-task-definition --cli-input-json "file://${workdir}/new.json" \
    --query 'taskDefinition.taskDefinitionArn' --output text)

echo "registered ${arn} (copied from ${source_arn})"
echo "arn=${arn}" >> "$GITHUB_OUTPUT"
