#!/usr/bin/env bash
# ビルド済みのイメージを ECR に push し、ダイジェストを表示する (just backend::push から呼ぶ)
#   - 同じ版を 2 回 push すると、タグの上書き禁止で ECR が拒否する
#   - 最初のタスク定義のイメージは、bootstrap のタグで push する (v* のタグは CD が push する)
#   - Terraform は bootstrap のタグからダイジェストを引くため、tfvars に書き写す必要はない
#
# 使い方: scripts/push.sh <ローカルのイメージ名> <版> <環境>
#   例: scripts/push.sh amber-api v1 prd
set -euo pipefail

if [ "$#" -ne 3 ]; then
    echo "usage: $0 <image> <version> <env>" >&2
    exit 1
fi

image="$1"
version="$2"
env="$3"
repo="${env}-amber-api"

url=$(aws ecr describe-repositories --repository-names "${repo}" \
    --query 'repositories[0].repositoryUri' --output text)

aws ecr get-login-password | docker login --username AWS --password-stdin "${url%%/*}"
docker tag "${image}:${version}" "${url}:${version}"
docker push "${url}:${version}"

digest=$(aws ecr describe-images --repository-name "${repo}" --image-ids "imageTag=${version}" \
    --query 'imageDetails[0].imageDigest' --output text)
echo "pushed ${url}:${version}"
echo "digest: ${digest}"
