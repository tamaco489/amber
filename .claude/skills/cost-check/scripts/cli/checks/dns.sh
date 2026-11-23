#!/usr/bin/env bash
# DNS と証明書: Route 53 の ecs-lab のレコード、ACM の ecs-lab の証明書

# JMESPath のリテラル (`tamaco-blog.jp.`) を展開させないため、--query はわざとシングルクォートで囲む
# shellcheck disable=SC2016
set -uo pipefail
source "$(dirname "$0")/../lib.sh"
require_amber_account

section "Route 53: tamaco-blog.jp のゾーンの ecs-lab のレコード (ない)"
zone_id=$(aws route53 list-hosted-zones-by-name --dns-name tamaco-blog.jp \
    --query 'HostedZones[?Name==`tamaco-blog.jp.`].Id | [0]' --output text 2>&1)
if [ -z "$zone_id" ] || [ "$zone_id" = "None" ]; then
    echo "ERROR: ホストゾーン tamaco-blog.jp. が見つからない: ${zone_id}"
    FAILED=1
else
    query '[.ResourceRecordSets[] | select(.Name | contains("ecs-lab")) | {name: .Name, type: .Type}] | sort_by(.name, .type)' \
        aws route53 list-resource-record-sets --hosted-zone-id "$zone_id"
fi

section "ACM: ecs-lab の証明書 (ない)"
query '[.CertificateSummaryList[] | select(.DomainName | contains("ecs-lab")) | {domain: .DomainName, status: .Status}] | sort_by(.domain)' \
    aws acm list-certificates

finish
