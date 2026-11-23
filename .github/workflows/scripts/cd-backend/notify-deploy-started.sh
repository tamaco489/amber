#!/usr/bin/env bash
# デプロイを始めたことを、EventBridge の default のイベントバスに送る (cd-backend から呼ぶ)
#   - infra の modules/notification のルールが拾い、Slack に投稿する
#   - source と detail-type は modules/notification と同じ値にする。deploy 用のロールは、この組み合わせのイベントしか送れない
#   - 値はエスケープされずに Slack の本文に埋め込まれるため、" を含む値は送らない (タグ、ARN、ユーザー名、URL には含まれない)
#
# 環境変数:
#   - EVENT_SOURCE: イベントの source
#   - EVENT_DETAIL_TYPE: イベントの detail-type
#   - DEPLOYMENT_ID: 始めたデプロイの ID
#   - TASK_DEFINITION: デプロイするタスク定義の ARN
#   - PUSHED: この実行で push したか (true / false)
#   - GITHUB_REF_NAME、GITHUB_ACTOR、GITHUB_SERVER_URL、GITHUB_REPOSITORY、GITHUB_RUN_ID: GitHub Actions が渡す
set -euo pipefail

detail=$(jq -nc \
    --arg version "$GITHUB_REF_NAME" \
    --arg deploymentId "$DEPLOYMENT_ID" \
    --arg taskDefinition "$TASK_DEFINITION" \
    --arg pushed "$PUSHED" \
    --arg actor "$GITHUB_ACTOR" \
    --arg runUrl "${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}" \
    '{
        version: $version,
        deploymentId: $deploymentId,
        taskDefinition: $taskDefinition,
        pushed: $pushed,
        actor: $actor,
        runUrl: $runUrl
    }')

entries=$(jq -nc \
    --arg source "$EVENT_SOURCE" \
    --arg detailType "$EVENT_DETAIL_TYPE" \
    --arg detail "$detail" \
    '[
        {
            Source: $source,
            DetailType: $detailType,
            Detail: $detail
        }
    ]')

# PutEvents は、一部のイベントを受け付けなくても成功を返すため、FailedEntryCount を確かめる
result=$(aws events put-events --entries "$entries")
failed=$(jq -r '.FailedEntryCount' <<<"$result")

if [ "$failed" != "0" ]; then
    echo "::error::Failed to send the deploy started event: $(jq -c '.Entries' <<<"$result")"
    exit 1
fi

echo "sent ${EVENT_DETAIL_TYPE} (${EVENT_SOURCE}): $(jq -r '.Entries[0].EventId' <<<"$result")"
