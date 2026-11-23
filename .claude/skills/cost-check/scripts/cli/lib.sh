#!/usr/bin/env bash
# cost-check のスクリプトで共通に使う設定と関数 (各スクリプトから source する)
#
# 何回流しても同じ状態なら同じ出力になるように、次を守る
#   - プロファイルとリージョンは固定する (呼び出し元の AWS_PROFILE を使わない)
#   - 失敗と「空」を区別する。コマンドが失敗したら ERROR と出し、最後に 0 以外で終わる
#   - 出力は jq -c で 1 行にし、並び順は sort で固定する。空なら [] と出す
#   - 時刻は JST の '%F %T' で出す
#
# 認証の不備 (SSO の期限切れ、権限の不足など) で調べられないときは、その場で止める
#   - 終了コード 2 と AUTH_ERROR を出す。check.sh は残りを流さずに止まる
#   - 調べられないまま、推測で結論を出さないため

export AWS_PROFILE=tamaco489
export AWS_REGION=ap-northeast-1
export AWS_PAGER=""
export TZ=Asia/Tokyo

FAILED=0

# 認証の不備を表す、AWS CLI のエラーの文言
AUTH_ERROR_PATTERN='ExpiredToken|InvalidClientTokenId|UnrecognizedClientException|AccessDenied|UnauthorizedOperation|AuthFailure|SignatureDoesNotMatch|Token has expired|SSO (session|token)|sso login|config profile .* could not be found|Unable to locate credentials|NoCredentialProviders'

# 認証の不備で止める (終了コード 2)
auth_error() {
    printf 'AUTH_ERROR: %s\n' "$1"
    printf '認証の不備で調べられない。調査を止める (aws sso login --profile %s のあと、やり直す)\n' "$AWS_PROFILE"
    exit 2
}

# 見出しを出す (確かめた時刻を付ける。レポートの行ごとの時刻は、ここから写す)
section() {
    printf '\n== %s [%s]\n' "$1" "$(date '+%F %T')"
}

# AWS CLI などを流し、標準出力の JSON を jq で整えて出す
#   使い方: query '<jq のフィルター>' aws ...
#   - コマンドが失敗したら、標準エラーの内容と一緒に ERROR と出す (空の出力と区別するため)
query() {
    local filter=$1
    shift
    local err out
    err=$(mktemp)
    if ! out=$("$@" 2>"$err"); then
        local message
        message=$(tr '\n' ' ' <"$err")
        rm -f "$err"
        if grep -qiE "$AUTH_ERROR_PATTERN" <<<"$message"; then
            auth_error "$message"
        fi
        printf 'ERROR: %s\n' "$message"
        FAILED=1
        return
    fi
    rm -f "$err"
    if ! jq -c "$filter" <<<"$out"; then
        printf 'ERROR: jq failed: %s\n' "$filter"
        FAILED=1
    fi
}

# 調べている AWS アカウントが amber のものかを確かめる
#   - アカウント ID は公開のリポジトリに書かないため、tfstate のバケットの持ち主と一致するかで確かめる
#   - 一致しなければ、以降を調べずに止める
require_amber_account() {
    local account
    if ! account=$(aws sts get-caller-identity --query Account --output text 2>&1); then
        auth_error "$(tr '\n' ' ' <<<"$account")"
    fi
    if ! aws s3api head-bucket --bucket prd-amber-tfstate --expected-bucket-owner "$account" >/dev/null 2>&1; then
        auth_error "このアカウントは prd-amber-tfstate の持ち主ではない (amber のアカウントではない)"
    fi
}

# amber の VPC の ID (タグ project=amber で引く)
amber_vpc_id() {
    aws ec2 describe-vpcs --filters Name=tag:project,Values=amber \
        --query 'Vpcs[0].VpcId' --output text
}

# 最後に、失敗したコマンドがあったかを出して終わる
finish() {
    if [ "$FAILED" -ne 0 ]; then
        printf '\nRESULT: FAILED (失敗したコマンドがある。ERROR の行を見る)\n'
        exit 1
    fi
    printf '\nRESULT: OK (すべてのコマンドが成功した)\n'
}
