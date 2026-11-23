#!/usr/bin/env bash
# 通知: EventBridge のルール、デッドレターキュー、消した接続・API 送信先・接続のシークレットが残っていないこと、
#       通知の Lambda の関数と状態のテーブルとボットトークンのパラメータ (置いておくだけなら無料)
set -uo pipefail
source "$(dirname "$0")/../lib.sh"
require_amber_account

section "EventBridge: ルール (4 本)"
query '[.Rules[] | {name: .Name, state: .State}] | sort_by(.name)' \
    aws events list-rules --name-prefix prd-amber-

section "EventBridge: 接続、API 送信先 (どちらもなし。通知を Lambda に切り替えたときに消した)"
query '[.Connections[] | {name: .Name, state: .ConnectionState}] | sort_by(.name)' \
    aws events list-connections --name-prefix prd-amber-
query '[.ApiDestinations[] | {name: .Name, state: .ApiDestinationState}] | sort_by(.name)' \
    aws events list-api-destinations --name-prefix prd-amber-

section "SQS: デッドレターキューのメッセージ数 (0 件)"
dlq_url=$(aws sqs get-queue-url --queue-name prd-amber-notification-dlq --query QueueUrl --output text 2>&1)
if [[ "$dlq_url" != https://* ]]; then
    echo "ERROR: キューが見つからない: ${dlq_url}"
    FAILED=1
else
    query '{messages: .Attributes.ApproximateNumberOfMessages}' \
        aws sqs get-queue-attributes --queue-url "$dlq_url" --attribute-names ApproximateNumberOfMessages
fi

section "Secrets Manager: prd-amber を含むシークレット (なし。消した接続の events!connection/... が残っていない)"
query '[.SecretList[] | select(.Name | contains("prd-amber")) | {name: (.Name | sub("/[0-9a-f-]+$"; "/<id>")), owner: .OwningService}] | sort_by(.name)' \
    aws secretsmanager list-secrets

section "DynamoDB: prd-amber を含むテーブル (通知の状態のテーブル 1 つだけ。オンデマンド)"
query '[.TableNames[] | select(contains("prd-amber"))]' \
    aws dynamodb list-tables
for table in $(aws dynamodb list-tables --output json 2>/dev/null | jq -r '.TableNames[] | select(contains("prd-amber"))'); do
    query '{name: .Table.TableName, billing: .Table.BillingModeSummary.BillingMode}' \
        aws dynamodb describe-table --table-name "$table"
done

section "Lambda: prd-amber を含む関数 (通知の関数 1 つだけ。provided.al2023、arm64)"
query '[.Functions[] | select(.FunctionName | contains("prd-amber")) | {name: .FunctionName, runtime: .Runtime, architectures: .Architectures}] | sort_by(.name)' \
    aws lambda list-functions

section "SSM: /prd-amber/ のパラメータ (ボットトークン 1 つだけ。標準)"
query '[.Parameters[] | {name: .Name, type: .Type, tier: .Tier}] | sort_by(.name)' \
    aws ssm describe-parameters --parameter-filters Key=Name,Option=BeginsWith,Values=/prd-amber/

finish
