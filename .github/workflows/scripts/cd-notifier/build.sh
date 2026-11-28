#!/usr/bin/env bash
# notifier (デプロイの通知の Lambda) を linux/arm64 でビルドし、zip にまとめる (cd-notifier から呼ぶ)
#   - ビルドの条件は、backend の justfile の build-notifier と同じにする (CI には just がないため、ここに同じコマンドを書く)
#     - ランタイムは provided.al2023 のため、実行ファイルの名前は bootstrap にする
#     - lambda.norpc は、provided のランタイムで使わない RPC のコードを外すタグ
#   - zip の中には bootstrap だけを、ディレクトリを付けずに入れる (Lambda は zip の直下の bootstrap を起動する)
#   - zip のパスを、GITHUB_OUTPUT の zip に書く
#
# 環境変数:
#   - GITHUB_OUTPUT: ステップの出力を書くファイル (GitHub Actions が渡す)
set -euo pipefail

cd backend

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -trimpath \
    -ldflags "-s -w" \
    -tags lambda.norpc \
    -o dist/notifier/bootstrap \
    ./cmd/notifier

zip_path="${PWD}/dist/notifier/notifier.zip"
rm -f "$zip_path"
zip -j -X "$zip_path" dist/notifier/bootstrap

echo "built ${zip_path} ($(wc -c < "$zip_path" | tr -d " ") bytes)"
echo "zip=${zip_path}" >> "$GITHUB_OUTPUT"
