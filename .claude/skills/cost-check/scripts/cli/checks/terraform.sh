#!/usr/bin/env bash
# Terraform の状態: enable_runtime の値、apply の途中か、plan の差分、state に残っている時間課金のモジュール
#   - plan は -lock=false で流す。ロックを取ると、そのたびに tfstate のバケットに .tflock の版が増えるため
#   - apply の途中かは、.tflock があるかを読み取りで確かめる (あれば、ほかの誰かがロックを持っている)
set -uo pipefail
source "$(dirname "$0")/../lib.sh"
require_amber_account

state_dir=infra/envs/prd

section "tfvars の enable_runtime"
grep '^enable_runtime' "${state_dir}/terraform.tfvars" || { echo "ERROR: enable_runtime が見つからない"; FAILED=1; }

section "state のロック (NOT_LOCKED なら apply の途中ではない)"
if aws s3api head-object --bucket prd-amber-tfstate --key envs/prd/terraform.tfstate.tflock >/dev/null 2>&1; then
    echo "LOCKED (apply の途中の可能性がある。結論を出さない)"
else
    echo "NOT_LOCKED"
fi

section "terraform plan (No changes なら、コードと AWS が一致している)"
if ! plan=$(terraform -chdir="$state_dir" plan -no-color -input=false -lock=false 2>&1); then
    if grep -qiE "$AUTH_ERROR_PATTERN|No valid credential sources" <<<"$plan"; then
        auth_error "$(grep -E 'Error' <<<"$plan" | tr '\n' ' ')"
    fi
    printf 'ERROR: terraform plan failed\n%s\n' "$(grep -E 'Error' <<<"$plan")"
    FAILED=1
else
    grep -E 'Plan:|No changes|will be (created|destroyed|updated|replaced)' <<<"$plan" | sort
fi

section "state に残っている時間課金のモジュール (false なら module.network.aws_vpc_endpoint.s3 だけ)"
if ! state=$(terraform -chdir="$state_dir" state list 2>&1); then
    echo "ERROR: terraform state list failed"
    FAILED=1
else
    grep -E 'module\.(alb|ecs|dns)|aws_vpc_endpoint' <<<"$state" | sort
fi

finish
