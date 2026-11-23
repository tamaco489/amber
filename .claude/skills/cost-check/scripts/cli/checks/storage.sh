#!/usr/bin/env bash
# ほぼ $0 で残るもの: ECR のイメージ、タスク定義、tfstate のバケット
set -uo pipefail
source "$(dirname "$0")/../lib.sh"
require_amber_account

section "ECR: prd-amber-api のイメージ (合計 100 MB 未満なら OK。1 GB あたり月 \$0.10)"
query '{images: ([.imageDetails[] | {tags: (.imageTags // [] | sort), mb: (.imageSizeInBytes / 1048576 * 10 | floor / 10)}] | sort_by(.tags)), totalMb: ([.imageDetails[].imageSizeInBytes] | add // 0 | . / 1048576 * 10 | floor / 10)}' \
    aws ecr describe-images --repository-name prd-amber-api

section "ECS: ACTIVE なタスク定義 (無料)"
query '[.taskDefinitionArns[] | split("/")[1]] | sort_by(split(":")[1] | tonumber)' \
    aws ecs list-task-definitions --family-prefix prd-amber-api --status ACTIVE

section "S3: tfstate のバケットの key ごとの古い版の数 (10 個以下なら OK、超えていたら NG。料金はほぼ \$0 で、止める操作は要らない)"
query '[(.Versions // [])[], (.DeleteMarkers // [])[] | select(.IsLatest | not) | .Key] | group_by(.) | map({key: .[0], noncurrent: length})' \
    aws s3api list-object-versions --bucket prd-amber-tfstate

finish
