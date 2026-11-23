#!/usr/bin/env bash
# amber の AWS のリソースのうち、料金がかかるものが残っていないかを、読み取りだけで確かめる (cost-check スキルから呼ぶ)
#   - AWS を変更する操作はしない (describe / list / get と terraform plan -lock=false だけ)
#   - 確認は責務ごとに checks/ に分けている。1 つだけ流すときは checks/<名前>.sh を直接呼ぶ
#   - 1 つでも失敗したら、残りは流したうえで、0 以外で終わる
#   - 認証の不備 (終了コード 2) なら、残りを流さずに止まる。推測で結論を出させないため
#
# 使い方: .claude/skills/cost-check/scripts/cli/check.sh (リポジトリのルートで実行する)
set -uo pipefail

checks_dir="$(dirname "$0")/checks"
status=0

for name in terraform compute monitoring dns notification storage; do
    printf '\n######## %s\n' "$name"
    "${checks_dir}/${name}.sh"
    code=$?
    if [ "$code" -eq 2 ]; then
        printf '\nCHECK: AUTH_ERROR (認証の不備で止めた。調査を中断し、ユーザーに報告する)\n'
        exit 2
    fi
    [ "$code" -eq 0 ] || status=1
done

if [ "$status" -ne 0 ]; then
    printf '\nCHECK: FAILED (失敗したスクリプトがある)\n'
else
    printf '\nCHECK: OK (すべてのスクリプトが成功した)\n'
fi
exit "$status"
