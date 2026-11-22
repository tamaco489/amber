#!/usr/bin/env bash
# デプロイを始めた内容を、ジョブのサマリーに書く (cd-backend から呼ぶ)
#
# 環境変数:
#   - GITHUB_REF_NAME: デプロイしたバージョン (Git のタグ)
#   - IMAGE: デプロイしたイメージ (<repository_url>@sha256:...)
#   - TASK_DEFINITION: 登録したタスク定義の ARN
#   - PUSHED: この実行で push したか (true / false)
#   - GITHUB_STEP_SUMMARY: ジョブのサマリーを書くファイル (GitHub Actions が渡す)
set -euo pipefail

cat >> "$GITHUB_STEP_SUMMARY" <<SUMMARY
## Deploy started

| Item | Value |
| --- | --- |
| Version | \`${GITHUB_REF_NAME}\` |
| Image | \`${IMAGE}\` |
| Pushed in this run | ${PUSHED} |
| Task definition | \`${TASK_DEFINITION}\` |

This workflow does not wait for the deployment to finish. Check the result in Slack or the ECS console.
SUMMARY
