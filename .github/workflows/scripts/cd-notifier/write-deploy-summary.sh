#!/usr/bin/env bash
# 置いたコードの内容を、ジョブのサマリーに書く (cd-notifier から呼ぶ)
#
# 環境変数:
#   - FUNCTION_NAME: 通知の Lambda の関数名
#   - CODE_SHA256: 置いたコードのハッシュ (update-function-code の CodeSha256)
#   - GITHUB_SHA: ビルドしたコミット (GitHub Actions が渡す)
#   - GITHUB_STEP_SUMMARY: ジョブのサマリーを書くファイル (GitHub Actions が渡す)
set -euo pipefail

cat >> "$GITHUB_STEP_SUMMARY" <<SUMMARY
## Notifier deployed

| Item | Value |
| --- | --- |
| Function | \`${FUNCTION_NAME}\` |
| Commit | \`${GITHUB_SHA}\` |
| CodeSha256 | \`${CODE_SHA256}\` |
SUMMARY
