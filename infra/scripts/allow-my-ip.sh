#!/usr/bin/env bash
# 今のグローバル IP を、テスト用リスナー (8443 番) への接続を許可するプレフィックスリストに反映する (just infra::allow-my-ip から呼ぶ)
#   - リストは Terraform の外で管理する (公開リポジトリに IP を書かないため。destroy しても残る)
#   - リストがなければ作り、あれば label の IP を今のものに置き換える (同じ IP なら何もしない)
#
# 使い方: scripts/allow-my-ip.sh <環境> <ラベル>
#   例: scripts/allow-my-ip.sh prd home
set -euo pipefail

if [ "$#" -ne 2 ]; then
    echo "usage: $0 <env> <label>" >&2
    exit 1
fi

env="$1"
label="$2"
name="${env}-amber-test-listener-allowed"
cidr="$(curl -fsS https://checkip.amazonaws.com | tr -d '[:space:]')/32"

id=$(aws ec2 describe-managed-prefix-lists --filters "Name=prefix-list-name,Values=${name}" \
    --query 'PrefixLists[0].PrefixListId' --output text)

if [ "${id}" = "None" ]; then
    aws ec2 create-managed-prefix-list --prefix-list-name "${name}" \
        --address-family IPv4 --max-entries 3 \
        --entries "Cidr=${cidr},Description=${label}" \
        --tag-specifications "ResourceType=prefix-list,Tags=[{Key=project,Value=amber},{Key=environment,Value=${env}},{Key=managed_by,Value=cli}]" \
        --query 'PrefixList.PrefixListId' --output text
    echo "created ${name} with ${cidr} (${label})"
    exit 0
fi

current=$(aws ec2 get-managed-prefix-list-entries --prefix-list-id "${id}" \
    --query "Entries[?Description=='${label}'].Cidr | [0]" --output text)
if [ "${current}" = "${cidr}" ]; then
    echo "${name} already allows ${cidr} (${label})"
    exit 0
fi

version=$(aws ec2 describe-managed-prefix-lists --prefix-list-ids "${id}" \
    --query 'PrefixLists[0].Version' --output text)
remove=()
if [ "${current}" != "None" ]; then
    remove=(--remove-entries "Cidr=${current}")
fi
aws ec2 modify-managed-prefix-list --prefix-list-id "${id}" --current-version "${version}" \
    --add-entries "Cidr=${cidr},Description=${label}" ${remove[@]+"${remove[@]}"} \
    --query 'PrefixList.State' --output text
echo "updated ${name}: ${current} -> ${cidr} (${label})"
