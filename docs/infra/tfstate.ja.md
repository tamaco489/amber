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
- 古い版は、`NoncurrentDays` (古い版になってからの日数) と `NewerNoncurrentVersions` (それより新しい古い版の数) の両方を超えたときに消えます。`NoncurrentDays` は省略できないため、最小の `1` にしています
- `NewerNoncurrentVersions` を使うには `Filter` が必要です。空の `Filter` でバケット全体を対象にしています
- 4 つの動作はどれも 1 つのルールに書けます (`ExpiredObjectDeleteMarker` と `AbortIncompleteMultipartUpload` は、タグの `Filter` と一緒に使えないだけです)
- `.tflock` は、今の版が削除マーカーで、その下に古い版 (ロックのファイルと削除マーカー) が 10 個残ります。古い版が残っている間は、今の削除マーカーは消えません

仕様: [ライフサイクルの設定の要素 (AWS 公式)](https://docs.aws.amazon.com/AmazonS3/latest/userguide/intro-lifecycle-rules.html)

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

### 2. 古い版が 10 個以下になっている

ライフサイクルは非同期で動き、反映に 1〜2 日かかります。入れてから数日後に確かめます。

```sh
aws s3api list-object-versions \
  --bucket prd-amber-tfstate \
  --profile tamaco489 --region ap-northeast-1 --output json \
  | jq '[(.Versions // [])[], (.DeleteMarkers // [])[]]
      | group_by(.Key)
      | map({key: .[0].Key, current: map(select(.IsLatest)) | length, noncurrent: map(select(.IsLatest | not)) | length})'
```

どの key も `noncurrent` が 10 以下なら想定どおりです。入れる前 (2026-11-22) は、state 本体が古い版 34 個、`.tflock` が古い版 123 個 (ロックのファイル 62 個と削除マーカー 61 個) でした。
