# tfstate のバケットのライフサイクル

[English](./tfstate.md) | [日本語](./tfstate.ja.md)

[ドキュメント一覧](../README.ja.md) に戻る。

Terraform の state は、S3 のバケット `prd-amber-tfstate` に置いています。バケットはバージョニングが有効で、apply のたびに古い版がたまります。ロックの `.tflock` も、ロックを外すたびに削除マーカーと古い版が残ります。
そこで、ライフサイクルで古い版を新しいものから 10 個だけ残し、それより古いものを消します。10 個残すのは、state を壊したときに数回前の apply まで戻せるようにするためです。

バケットは state 自身では管理できないため、Terraform の外で AWS CLI で作っています (`infra/envs/prd/backend.tf`)。ライフサイクルも同じく、ユーザーが CLI で入れます。

## 設定の内容

定義は [`infra/tfstate/lifecycle.json`](../../infra/tfstate/lifecycle.json) に置き、これを正とします。画面や CLI で変更したら、JSON にも反映します。

| 項目                             | 値                                                 | 意味                                                         |
| -------------------------------- | -------------------------------------------------- | ------------------------------------------------------------ |
| 対象                             | `Filter: {}`                                       | バケット全体 (state 本体と `.tflock`)                        |
| `NoncurrentVersionExpiration`    | `NewerNoncurrentVersions: 10`、`NoncurrentDays: 1` | 古い版は、新しいものから 10 個を残し、それより古いものを消す |
| `Expiration`                     | `ExpiredObjectDeleteMarker: true`                  | 古い版が 1 つもなくなった削除マーカーを消す                  |
| `AbortIncompleteMultipartUpload` | `DaysAfterInitiation: 1`                           | 途中で止まったマルチパートアップロードを 1 日で消す          |

- 今の版 (最新の state) は消しません。`Expiration` には日数を書かず、削除マーカーの掃除だけに使います
- 古い版は、`NoncurrentDays` (古い版になってからの日数) と `NewerNoncurrentVersions` (それより新しい古い版の数) の両方を超えたときに消えます。`NoncurrentDays` は省略できないため、最小の `1` にしています。消えるまでの時間は「[2. 古い版がライフサイクルどおりに消えている](#2-古い版がライフサイクルどおりに消えている)」に書きます
- `NewerNoncurrentVersions` を使うには `Filter` が必要です。空の `Filter` でバケット全体を対象にしています
- 4 つの動作はどれも 1 つのルールに書けます (`ExpiredObjectDeleteMarker` と `AbortIncompleteMultipartUpload` は、タグの `Filter` と一緒に使えないだけです)
- `.tflock` は、今の版が削除マーカーで、その下に古い版 (ロックのファイルと削除マーカー) が 10 個残ります。古い版が残っている間は、今の削除マーカーは消えません

### 参考 (設定の内容)

- DOC: <https://docs.aws.amazon.com/AmazonS3/latest/userguide/intro-lifecycle-rules.html>

## 入れる・更新する

ユーザーが手元で実行します。今の設定は丸ごと置き換わります。

```sh
aws s3api put-bucket-lifecycle-configuration \
  --bucket prd-amber-tfstate \
  --lifecycle-configuration file://infra/tfstate/lifecycle.json \
  --profile tamaco489 --region ap-northeast-1
```

## 確かめる

### 1. 設定が入っている

```sh
aws s3api get-bucket-lifecycle-configuration \
  --bucket prd-amber-tfstate \
  --profile tamaco489 --region ap-northeast-1
```

JSON と同じルールが返れば想定どおりです。入れる前は `NoSuchLifecycleConfiguration` のエラーになります。

### 2. 古い版がライフサイクルどおりに消えている

古い版は、次の 2 つの条件を**両方**満たしたときに消えます。

| 条件                          | 満たす時刻                                                                                               |
| ----------------------------- | -------------------------------------------------------------------------------------------------------- |
| `NoncurrentDays: 1`           | 古い版になった時刻 (1 つ新しい版が作られた時刻) に 1 日を足し、次の UTC 0 時 (JST 9 時) に切り上げた時刻 |
| `NewerNoncurrentVersions: 10` | それより新しい古い版が 10 個そろった時刻                                                                 |

- ライフサイクルは 1 日に 1 回、非同期で動きます。条件を満たしてから実際に消えるまでの時刻は保証されません
- そのため、apply を何回も流した日の直後は、古い版が 10 個を超えるのが普通です。例えば、2026-11-28 17:00 (JST) に古い版になった版は、日数の条件を 2026-11-30 09:00 (JST) に満たし、そのあとのライフサイクルの実行で消えます
- 削除マーカーが `NewerNoncurrentVersions` の数に入るかは、AWS の文書では確かめられていません。`/cost-check` は、削除マーカーも古い版の 1 つとして数えます
- `/cost-check` は、古い版の数ではなく、2 つの条件を満たしてから 3 日を過ぎても残っている版があるかで判定します (`.claude/skills/cost-check/SKILL.md` の「tfstate の古い版の判定」)

古い版の数は、次のコマンドで確かめます。

```sh
aws s3api list-object-versions \
  --bucket prd-amber-tfstate \
  --profile tamaco489 --region ap-northeast-1 --output json \
  | jq '[(.Versions // [])[], (.DeleteMarkers // [])[]]
      | group_by(.Key)
      | map({key: .[0].Key, current: map(select(.IsLatest)) | length, noncurrent: map(select(.IsLatest | not)) | length})'
```

apply をしない日が 3 日ほど続いたあとに、どの key も `noncurrent` が 10 以下なら想定どおりです。入れる前 (2026-11-22) は、state 本体が古い版 34 個、`.tflock` が古い版 123 個 (ロックのファイル 62 個と削除マーカー 61 個) でした。

### 参考 (2. 古い版がライフサイクルどおりに消えている)

- DOC: <https://docs.aws.amazon.com/AmazonS3/latest/userguide/intro-lifecycle-rules.html>
- DOC: <https://repost.aws/knowledge-center/s3-lifecycle-rule-delay>
