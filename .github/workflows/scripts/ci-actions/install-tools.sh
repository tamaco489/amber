#!/usr/bin/env bash
# actionlint と shellcheck の、バージョンを固定したものを入れ、PATH の先頭に置く (ci-actions から呼ぶ)
#   - ubuntu-latest に入っている shellcheck はバージョンが決まらないため、リリースから取る
#   - 取ったファイルは、sha256 のチェックサムを確かめてから展開する
#
# 環境変数:
#   - ACTIONLINT_VERSION / ACTIONLINT_SHA256: actionlint のバージョンと、linux_amd64 の tar.gz のチェックサム
#   - SHELLCHECK_VERSION / SHELLCHECK_SHA256: shellcheck のバージョンと、linux.x86_64 の tar.xz のチェックサム
#   - RUNNER_TEMP: 作業用のディレクトリ (GitHub Actions が渡す)
#   - GITHUB_PATH: 次のステップから PATH に足すディレクトリを書くファイル (GitHub Actions が渡す)
set -euo pipefail

bin="${RUNNER_TEMP}/bin"
mkdir -p "$bin"
cd "$RUNNER_TEMP"

actionlint_tar="actionlint_${ACTIONLINT_VERSION}_linux_amd64.tar.gz"
curl -fsSLO "https://github.com/rhysd/actionlint/releases/download/v${ACTIONLINT_VERSION}/${actionlint_tar}"
echo "${ACTIONLINT_SHA256}  ${actionlint_tar}" | sha256sum -c -
tar -xzf "$actionlint_tar" -C "$bin" actionlint

shellcheck_tar="shellcheck-v${SHELLCHECK_VERSION}.linux.x86_64.tar.xz"
curl -fsSLO "https://github.com/koalaman/shellcheck/releases/download/v${SHELLCHECK_VERSION}/${shellcheck_tar}"
echo "${SHELLCHECK_SHA256}  ${shellcheck_tar}" | sha256sum -c -
tar -xJf "$shellcheck_tar" --strip-components=1 -C "$bin" "shellcheck-v${SHELLCHECK_VERSION}/shellcheck"

echo "$bin" >> "$GITHUB_PATH"
