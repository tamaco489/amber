#!/usr/bin/env bash
# gopls の解析を、backend の Go のファイルすべてに流す
#   - golangci-lint に入っていない解析 (writestring など) を拾うため
#   - gopls check は警告があっても 0 で終わるため、何か出力されたら失敗にする
set -euo pipefail

cd "$(dirname "$0")/.."

out="$(find . -name '*.go' -not -path './dist/*' -print0 | xargs -0 gopls check -severity=warning)"
if [[ -n "$out" ]]; then
    printf '%s\n' "$out"
    exit 1
fi
