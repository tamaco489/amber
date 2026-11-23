#!/usr/bin/env bash
# 通知: EventBridge のルール・接続・API 送信先、デッドレターキュー、接続のシークレット (置いておくだけなら無料)
set -uo pipefail
source "$(dirname "$0")/../lib.sh"
require_amber_account

section "EventBridge: ルール、接続、API 送信先"
query '[.Rules[] | {name: .Name, state: .State}] | sort_by(.name)' \
    aws events list-rules --name-prefix prd-amber-
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

section "Secrets Manager: prd-amber を含むシークレット (接続のもの 1 つだけ。料金は API 送信先に含まれる)"
query '[.SecretList[] | select(.Name | contains("prd-amber")) | {name: (.Name | sub("/[0-9a-f-]+$"; "/<id>")), owner: .OwningService}] | sort_by(.name)' \
    aws secretsmanager list-secrets

finish
