#!/usr/bin/env bash
# ほぼ $0 で残るもの: ECR のイメージ、タスク定義、tfstate のバケット
set -uo pipefail
source "$(dirname "$0")/../lib.sh"
require_amber_account

section "ECR: prd-amber-api のイメージ (合計 100 MB 未満なら OK。1 GB あたり月 \$0.10)"
query '{images: ([.imageDetails[] | {tags: (.imageTags // [] | sort), mb: (.imageSizeInBytes / 1048576 * 10 | floor / 10)}] | sort_by(.tags)), totalMb: ([.imageDetails[].imageSizeInBytes] | add // 0 | . / 1048576 * 10 | floor / 10)}' \
    aws ecr describe-images --repository-name prd-amber-api

section "ECS: ACTIVE なタスク定義 (無料)"
query '[.taskDefinitionArns[] | split("/")[1]] | sort_by(split(":")[1] | tonumber)' \
    aws ecs list-task-definitions --family-prefix prd-amber-api --status ACTIVE

# tfstate のバケットの古い版が、ライフサイクルどおりに消えているかを、key ごとに確かめる
#   - S3 は、NoncurrentDays と NewerNoncurrentVersions の両方を満たした古い版だけを消す
#   - 日数の条件: 古い版になった時刻 (1 つ新しい版の LastModified) に NoncurrentDays を足し、次の UTC 0 時に切り上げた時刻に満たす
#   - 数の条件: 新しいほうから数えて NewerNoncurrentVersions 番目の古い版が、古い版になった時刻に満たす
#     (それより新しい古い版が NewerNoncurrentVersions 個そろった時刻)
#   - 2 つを満たした時刻 (eligibleAtJst) から grace_days 日 (deadlineJst) を過ぎても残っている版があれば NG。
#     ライフサイクルは 1 日に 1 回、非同期で動き、消える時刻は保証されないため、待つ日数を置く
#   - 削除マーカーも、古い版の 1 つとして並べる (.tflock は、ロックを外すたびに削除マーカーが残る)
#   - 出力に今の時刻は含めない。今の時刻で変わるのは、待つ日数を過ぎたか (overdue、verdict) だけ
#   - verdict: OK (古い版が NewerNoncurrentVersions 個以下) / OK_PENDING (超えているが、どれも待つ日数の内) / NG (待つ日数を過ぎて残る版がある)
#   - excessVersions は、新しい NewerNoncurrentVersions 個より古い版を、新しい順に並べる
#   - 値は scripts/mcp/check.py の tfstateVersions と同じにする
# DOC: https://docs.aws.amazon.com/AmazonS3/latest/userguide/intro-lifecycle-rules.html
# DOC: https://repost.aws/knowledge-center/s3-lifecycle-rule-delay
section "S3: tfstate のバケットの key ごとの古い版 (消える条件を満たしてから 3 日を過ぎて残る版がなければ OK、あれば NG。料金はほぼ \$0 で、止める操作は要らない)"
# $ は jq の変数のため、シェルで展開させない
# shellcheck disable=SC2016
query '
def noncurrent_days: 1;           # infra/tfstate/lifecycle.json の NoncurrentDays
def newer_noncurrent_versions: 10; # infra/tfstate/lifecycle.json の NewerNoncurrentVersions
def grace_days: 3;                # 消える条件を満たしてから、待つ日数
def epoch: sub("\\+00:00$"; "Z") | fromdateiso8601;
def jst: . + 32400 | strftime("%Y-%m-%d %H:%M:%S");
def next_utc_midnight: (. / 86400 | floor) + 1 | . * 86400;
now as $now
| [(.Versions // [])[], (.DeleteMarkers // [])[] | {key: .Key, latest: .IsLatest, t: (.LastModified | epoch)}]
| group_by(.key)
| map(
    # 今の版を先頭に、古い版を新しい順に並べる。古い版 n 番目 ($all[n]) は、$all[n - 1] が作られた時刻に古い版になった
    ([.[] | select(.latest)] + ([.[] | select(.latest | not)] | sort_by(-.t))) as $all
    | [range(newer_noncurrent_versions + 1; $all | length) as $n
        | ([($all[$n - 1].t + noncurrent_days * 86400 | next_utc_midnight), $all[$n - newer_noncurrent_versions - 1].t] | max) as $eligible
        | {
            noncurrentSinceJst: ($all[$n - 1].t | jst),
            eligibleAtJst: ($eligible | jst),
            deadlineJst: ($eligible + grace_days * 86400 | jst),
            overdue: ($now > $eligible + grace_days * 86400)
        }] as $excess
    | ([$excess[] | select(.overdue)] | length) as $overdue
    | {
        key: $all[0].key,
        noncurrent: (($all | length) - 1),
        excess: ($excess | length),
        overdue: $overdue,
        verdict: (if $overdue > 0 then "NG" elif ($excess | length) > 0 then "OK_PENDING" else "OK" end),
        excessVersions: [$excess[] | del(.overdue)]
    })
' aws s3api list-object-versions --bucket prd-amber-tfstate

finish
