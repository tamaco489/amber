#!/usr/bin/env bash
# 通知の Lambda のコードを zip で置き換え、置き換えが終わるまで待つ (cd-notifier から呼ぶ)
#   - 関数は Terraform が仮のコードで作り、コードの変更は無視している (infra の modules/notification)
#   - 置き換えの直後は LastUpdateStatus が InProgress になる。Successful になるまで待ち、失敗したらジョブを失敗にする
#   - 置き換えたコードのハッシュ (CodeSha256) を、GITHUB_OUTPUT の code_sha256 に書く
#
# 環境変数:
#   - FUNCTION_NAME: 通知の Lambda の関数名
#   - ZIP_PATH: 置くコードの zip のパス
#   - GITHUB_OUTPUT: ステップの出力を書くファイル (GitHub Actions が渡す)
set -euo pipefail

code_sha256=$(aws lambda update-function-code \
    --function-name "$FUNCTION_NAME" \
    --zip-file "fileb://${ZIP_PATH}" \
    --query CodeSha256 --output text)

echo "updated ${FUNCTION_NAME} (CodeSha256: ${code_sha256}). Waiting for the update to finish"

# GetFunctionConfiguration の LastUpdateStatus を 5 秒ごとに読み、Successful になるまで待つ (Failed なら失敗で終わる)
aws lambda wait function-updated --function-name "$FUNCTION_NAME"

status=$(aws lambda get-function-configuration \
    --function-name "$FUNCTION_NAME" \
    --query '{state: State, lastUpdateStatus: LastUpdateStatus, codeSha256: CodeSha256}' \
    --output json)
echo "finished: $(jq -c . <<<"$status")"

echo "code_sha256=${code_sha256}" >> "$GITHUB_OUTPUT"
