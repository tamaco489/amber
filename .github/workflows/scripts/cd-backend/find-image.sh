#!/usr/bin/env bash
# ECR に push 済みのイメージのダイジェストを、GITHUB_OUTPUT の digest に書く (cd-backend から呼ぶ)
#   - タグがまだなければ、digest は空にする (このあとのステップでビルドして push する)
#   - タグがない以外のエラー (権限が足りないなど) は、そのまま失敗させる
#
# 環境変数:
#   - ECR_REPOSITORY: ECR のリポジトリ名
#   - GITHUB_REF_NAME: 探すイメージのタグ (Git のタグ)
#   - GITHUB_OUTPUT: ステップの出力を書くファイル (GitHub Actions が渡す)
set -euo pipefail

if out=$(aws ecr describe-images \
    --repository-name "$ECR_REPOSITORY" \
    --image-ids "imageTag=$GITHUB_REF_NAME" \
    --query 'imageDetails[0].imageDigest' --output text 2>&1); then
    digest="$out"
    echo "found ${ECR_REPOSITORY}:${GITHUB_REF_NAME} (${digest})"
elif grep -q ImageNotFoundException <<<"$out"; then
    digest=""
    echo "${ECR_REPOSITORY}:${GITHUB_REF_NAME} is not pushed yet"
else
    echo "$out" >&2
    exit 1
fi

echo "digest=${digest}" >> "$GITHUB_OUTPUT"
