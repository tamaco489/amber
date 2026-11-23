#!/usr/bin/env bash
# cost-check のレポートを、メインのセッションで検証する (読み取りだけ)
#   1. レポートに、雛形の「(記入: ...)」が残っていないか
#   2. 調査の時刻が、依頼の時刻より後で、今より前か (レポートのファイルの更新時刻も)
#   3. check.sh を流し直し、レポートの付録の check.sh の出力と、時刻を除いて一致するか
#
# 使い方: .claude/skills/cost-check/scripts/verify/verify.sh <レポートのパス> <依頼の時刻 'YYYY-MM-DD HH:MM:SS'>
#   リポジトリのルートで実行する。問題がなければ 0、あれば 1 で終わる
set -uo pipefail
export TZ=Asia/Tokyo

report=${1:?レポートのパスを渡す}
requested_at=${2:?依頼の時刻を渡す}
check_sh="$(dirname "$0")/../cli/check.sh"
problems=0

ng() {
    printf 'NG: %s\n' "$1"
    problems=1
}

printf '== 1. 雛形の記入漏れ\n'
# コメントの行 (<!-- ... -->) は対象から外す
if grep -n '(記入:' "$report" | grep -v '<!--'; then
    ng "記入されていない箇所がある"
else
    echo "OK"
fi

printf '\n== 2. 時刻\n'
now=$(date '+%F %T')
started=$(grep -E '^\| 調査を始めた時刻' "$report" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}' | head -1)
finished=$(grep -E '^\| 調査を終えた時刻' "$report" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}' | head -1)
modified=$(stat -f '%Sm' -t '%F %T' "$report")
printf 'requested=%s started=%s finished=%s modified=%s now=%s\n' "$requested_at" "$started" "$finished" "$modified" "$now"
# 同じ形の文字列は、辞書順で比べると時刻の順になる
[ -n "$started" ] || ng "調査を始めた時刻がない"
[ -n "$finished" ] || ng "調査を終えた時刻がない"
[[ "$started" > "$requested_at" || "$started" == "$requested_at" ]] || ng "調査を始めた時刻が、依頼より前"
[[ "$finished" > "$started" || "$finished" == "$started" ]] || ng "調査を終えた時刻が、始めた時刻より前"
[[ "$now" > "$finished" || "$now" == "$finished" ]] || ng "調査を終えた時刻が、今より後"
[[ "$modified" > "$requested_at" || "$modified" == "$requested_at" ]] || ng "レポートの更新が、依頼より前"
[ "$problems" -eq 0 ] && echo "OK"

printf '\n== 3. check.sh の流し直しと付録の突き合わせ (時刻を除く)\n'
strip_time() {
    sed -E 's/ \[[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\]$//'
}
reported=$(awk '/^### check.sh/{found=1; next} found && /^```text/{inside=1; next} inside && /^```/{exit} inside' "$report" | strip_time)
current_raw=$("$check_sh" 2>&1)
if [ $? -eq 2 ]; then
    printf '%s\n' "$current_raw" | grep AUTH_ERROR
    echo "VERIFY: AUTH_ERROR (認証の不備で検証できない。止めてユーザーに報告する)"
    exit 2
fi
current=$(printf '%s\n' "$current_raw" | strip_time)
if [ -z "$reported" ]; then
    ng "付録に check.sh の出力がない"
elif diff <(printf '%s\n' "$reported") <(printf '%s\n' "$current"); then
    echo "OK (一致した)"
else
    ng "付録の出力と、今の出力が違う (上の差分を見る。調査のあとに状態が変わった可能性もある)"
fi

printf '\n'
if [ "$problems" -ne 0 ]; then
    echo "VERIFY: NG"
    exit 1
fi
echo "VERIFY: OK"
