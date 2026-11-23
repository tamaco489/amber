#!/usr/bin/env bash
# 置いておくと課金されるもの: CloudWatch アラーム、ロググループ
#   - オートスケーリングのアラームは TargetTracking-service/prd-amber/... という名前のため、
#     prd-amber で始まるかではなく、含むかで探す
set -uo pipefail
source "$(dirname "$0")/../lib.sh"
require_amber_account

section "CloudWatch: prd-amber を含むアラーム (ない。1 個あたり月 \$0.10)"
query '[.MetricAlarms[], .CompositeAlarms[] | select(.AlarmName | contains("prd-amber")) | .AlarmName] | sort' \
    aws cloudwatch describe-alarms

section "CloudWatch Logs: prd-amber を含むロググループ (ない)"
query '[.logGroups[] | select(.logGroupName | contains("prd-amber")) | {name: .logGroupName, bytes: .storedBytes}] | sort_by(.name)' \
    aws logs describe-log-groups

finish
