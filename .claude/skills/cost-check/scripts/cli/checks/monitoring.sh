#!/usr/bin/env bash
# 置いておくと課金されるもの: CloudWatch アラーム、ロググループ
#   - オートスケーリングのアラームは TargetTracking-service/prd-amber/... という名前のため、
#     prd-amber で始まるかではなく、含むかで探す
#   - 通知の Lambda のロググループは、enable_runtime で消さずに常に置く (保存期間 3 日)。
#     ほかのロググループと分けて、保存期間と保存量を出す
set -uo pipefail
source "$(dirname "$0")/../lib.sh"
require_amber_account

notifier_log_group="/aws/lambda/prd-amber-notification"

section "CloudWatch: prd-amber を含むアラーム (ない。1 個あたり月 \$0.10)"
query '[.MetricAlarms[], .CompositeAlarms[] | select(.AlarmName | contains("prd-amber")) | .AlarmName] | sort' \
    aws cloudwatch describe-alarms

section "CloudWatch Logs: prd-amber を含むロググループ (通知の Lambda のもの以外はない)"
query "[.logGroups[] | select((.logGroupName | contains(\"prd-amber\")) and .logGroupName != \"${notifier_log_group}\") | {name: .logGroupName, bytes: .storedBytes}] | sort_by(.name)" \
    aws logs describe-log-groups

section "CloudWatch Logs: 通知の Lambda のロググループ (残ってよい。保存期間 3 日)"
query "[.logGroups[] | select(.logGroupName == \"${notifier_log_group}\") | {name: .logGroupName, retention_days: .retentionInDays, bytes: .storedBytes}]" \
    aws logs describe-log-groups --log-group-name-prefix "$notifier_log_group"

finish
