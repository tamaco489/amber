---
name: cost-check
description: amber の AWS のリソースのうち、置いておくだけで料金がかかるものが残っていないかを、別のエージェントに読み取りだけで調べさせ、レポートをメインのセッションで検証してから報告するスキル。「コストがかからない状態か確かめて」「課金されるリソースが残っていないか調べて」「止めたあとの状態をチェックして」などのリクエストでトリガーする。
---

# 料金がかかるリソースの確認

`enable_runtime = false` で apply したあとに、料金がかかるリソースが残っていないかを確かめる。
調査は `cost-auditor` エージェントが行い、メインのセッションはレポートが正しいかを検証してから、ユーザーに報告する。

## 制約事項

> [!IMPORTANT]
>
> - **即時性は求めない。どれだけ時間がかかってもよいので、根拠に基づく正確な調査と検証を行う**
> - 調査するエージェントと、検証するメインのセッションの両方に適用する
> - 推測で埋めない。「たぶん」「おそらく」で結論を出さない。出力で確かめたことだけを書く
> - 少しでも疑わしい値は、別の方法 (CLI と MCP、個別の describe) で確かめ直す
> - 確かめられなかったものは、「未確認」と理由をはっきり書く。OK として扱わない
> - 調査は `cost-auditor` エージェントに委譲し、メインのコンテキストを生の出力で汚さない
> - 読み取りだけで調べる。AWS を変更する操作はしない
> - **AWS CLI、AWS MCP のどちらかが認証の不備 (SSO の期限切れ、権限の不足、MCP の再ログインが必要など) で使えないときは、作業を止めてユーザーに報告する。** 使える方法だけで代わりに調べたり、推測で結論を出したりしない

## ファイル

| ファイル                             | 役割                                                                                                              |
| ------------------------------------ | ----------------------------------------------------------------------------------------------------------------- |
| `SKILL.md`                           | メインのセッションの手順と、判断の表                                                                              |
| `report-template.md`                 | レポートの雛形。エージェントが埋める                                                                              |
| `scripts/cli/check.sh`               | CLI での確認の入り口。`checks/` を順に流す (読み取りだけ)                                                         |
| `scripts/cli/lib.sh`                 | 共通の設定 (プロファイル、リージョン、JST) と関数 (失敗の検出、認証の確認)                                        |
| `scripts/cli/checks/terraform.sh`    | `enable_runtime`、state のロック、`terraform plan`、state に残る時間課金のモジュール                              |
| `scripts/cli/checks/compute.sh`      | 時間課金: ECS、ELB、VPC エンドポイント、NAT、EIP、EC2、EBS、ENI、オートスケーリング                               |
| `scripts/cli/checks/monitoring.sh`   | 置いておくと課金: CloudWatch アラーム、ロググループ (通知の Lambda のものは分けて出す)                            |
| `scripts/cli/checks/dns.sh`          | Route 53 のレコード、ACM の証明書                                                                                 |
| `scripts/cli/checks/notification.sh` | 通知: EventBridge、 DLQ 、消した接続とそのシークレット、Lambda の関数、状態のテーブル、ボットトークンのパラメータ |
| `scripts/cli/checks/storage.sh`      | ほぼ \$0: ECR、タスク定義、tfstate のバケット                                                                     |
| `scripts/mcp/check.py`               | AWS MCP の `run_script` にそのまま渡す、固定の確認のコード                                                        |
| `scripts/verify/verify.sh`           | メインのセッションの検証 (記入漏れ、時刻、CLI の確認の流し直しと突き合わせ)                                       |
| `.shellcheckrc`                      | `checks/` が `lib.sh` を読み込むのを、shellcheck にたどらせる設定                                                 |
| `.claude/agents/cost-auditor.md`     | 調査するエージェントの定義                                                                                        |

## 冪等性 (何回流しても同じ結果になるための決まり)

- 確認は読み取りだけ。`terraform plan` は `-lock=false` で流す (ロックを取ると、そのたびに tfstate の版が増えるため)
- apply の途中かは、state のロックのファイル (`.tflock`) があるかを読み取りで確かめる
- プロファイル (`tamaco489`)、リージョン (`ap-northeast-1`)、タイムゾーン (JST) は、スクリプトで固定する
- 調べるアカウントが amber のものか (tfstate のバケットの持ち主か) を、最初に確かめる。違えば止める
- コマンドの失敗は `ERROR` と出し、スクリプトは 0 以外で終わる。失敗を「空」と取り違えない
- 認証の不備は `AUTH_ERROR` と出し、終了コード 2 で、残りを流さずに止まる (MCP は `authError` を返す)
- 出力は並び順を固定する。空なら `[]` と出す
- MCP のコードは `scripts/mcp/check.py` を書き換えずに渡す
- 毎回違ってよいのは、時刻とレポートのパスだけ

## 手順 (メインのセッション)

1. **依頼の時刻と作業ツリーを控える。** `TZ=Asia/Tokyo date '+%F %T'` と `git status --porcelain` を実行する
2. **レポートのパスを決める。** `tmp/cost-check-skill-reports/<YYYYMMDD>/<YYYYMMDD-HHMMSS>.md` (`tmp/` は Git の対象外)。日付と時刻は、手順 1 の依頼の時刻 (JST) から作る

   ```bash
   TZ=Asia/Tokyo date '+tmp/cost-check-skill-reports/%Y%m%d/%Y%m%d-%H%M%S.md'
   ```

3. **エージェントに調べさせる。** Agent ツールで `subagent_type: cost-auditor` を呼ぶ。プロンプトには、レポートのパスと、依頼の時刻を書く
   - エージェントが `AUTH_ERROR` を返したら、**ここで止める。** 検証もやり直しもせず、何が失敗したか (CLI か MCP か、エラーの文言) と、直し方 (`aws sso login --profile tamaco489`、`/mcp` での再ログイン) をユーザーに伝え、ユーザーの指示を待つ
4. **レポートを検証する。** 下の「検証すること」をすべて確かめる。1 つでも満たさなければ、エージェントに差し戻すか、自分で読み取りで確かめ直す
5. **ユーザーに報告する。** 結論、表の要約、期待と違ったもの、確かめられなかったもの、レポートのパスを伝える。検証で見つけたこと、直したことがあれば、それも伝える

## 検証すること

1. **`scripts/verify/verify.sh` を流す。** 記入漏れ、時刻の前後、CLI の確認 (`scripts/cli/check.sh`) の流し直しと付録の一致を確かめる

   ```bash
   .claude/skills/cost-check/scripts/verify/verify.sh <レポートのパス> '<依頼の時刻>'
   ```

   - 突き合わせで差分が出たら、調査のあとに状態が変わったのか、レポートの写し間違いかを見分ける
   - `verify.sh` が `AUTH_ERROR` で止まったら、検証を止めてユーザーに報告する

2. **表と付録が一致するか。** 表の「実際」と「時刻」が、付録の出力 (見出しの `[時刻]`) から写したものになっている。付録に根拠がない行は、根拠がないとみなす
3. **CLI と MCP が一致するか。** 時間課金の行とアラームと tfstate のバケットは、両方で確かめている。アカウントも一致している。片方だけで判定した行があれば、差し戻す
4. **結論と矛盾しないか。** NG や未確認の行があるのに「残っていない」としていない。state が `LOCKED` なら、結論は未確認になっている
5. **作業ツリーが変わっていないか。** `git status --porcelain` が、手順 1 と同じ (エージェントがレポート以外を書いていない)

## 判断の表

| 区分                           | リソース                                                                        | OK の条件                                                                                                                                                                |
| ------------------------------ | ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 前提                           | アカウント、state のロック、`terraform plan`                                    | amber のアカウント。`NOT_LOCKED`。`No changes`                                                                                                                           |
| 時間課金 (残っていたら NG)     | ECS のクラスター・サービス・タスク                                              | ACTIVE なクラスターがない (`INACTIVE`、または一覧にない)                                                                                                                 |
|                                | ALB、ターゲットグループ                                                         | `prd-amber` を含むものがない                                                                                                                                             |
|                                | インターフェース型の VPC エンドポイント                                         | amber の VPC にない (Gateway の S3 だけ)                                                                                                                                 |
|                                | NAT、EC2、ENI (amber の VPC)、EIP、EBS (タグ `project=amber`)                   | ない                                                                                                                                                                     |
|                                | オートスケーリングの対象                                                        | `prd-amber` を含むものがない                                                                                                                                             |
| 置いておくと課金 (残ったら NG) | CloudWatch アラーム (月 \$0.10/個)                                              | `prd-amber` を含むものがない (`TargetTracking-service/prd-amber/...` も含む)                                                                                             |
|                                | ロググループ                                                                    | `prd-amber` を含むものがない (通知の Lambda の `/aws/lambda/prd-amber-notification` は除く)                                                                              |
|                                | Route 53 のレコード、ACM の証明書                                               | `ecs-lab` を含むものがない                                                                                                                                               |
| ほぼ \$0 (残ってよい)          | ECR `prd-amber-api`                                                             | 合計 100 MB 未満                                                                                                                                                         |
|                                | tfstate のバケット `prd-amber-tfstate`                                          | 消える条件を満たして 3 日を過ぎた古い版がない (`OK` か `OK_PENDING`。下の「tfstate の古い版の判定」)。止める操作は要らず、料金の目安とライフサイクルが入っているかを書く |
|                                | 通知の Lambda のロググループ `/aws/lambda/prd-amber-notification`               | 保存期間が 3 日。`enable_runtime` で消さずに常に置く (apply の前で、まだないのもよい)                                                                                    |
| 無料 (残ってよい)              | VPC、サブネット、IGW、ルートテーブル、セキュリティグループ、S3 のエンドポイント | -                                                                                                                                                                        |
|                                | IAM ロール、タスク定義、プレフィックスリスト                                    | -                                                                                                                                                                        |
|                                | 通知 (EventBridge のルール、SQS)                                                | DLQ が 0 件。EventBridge の接続と API 送信先がない (Lambda に切り替えたときに消した)                                                                                     |
|                                | Secrets Manager の `prd-amber` を含むシークレット                               | ない (接続と一緒に `events!connection/prd-amber-slack/...` が消えている。残っていたら NG)                                                                                |
|                                | DynamoDB の通知の状態のテーブル `prd-amber-notification`                        | このテーブル 1 つだけで、`PAY_PER_REQUEST` (オンデマンド)                                                                                                                |
|                                | SSM のパラメータ `/prd-amber/notification/slack-{bot-token,user-ids}`           | この 2 つだけで、どちらも `Standard` (標準)                                                                                                                              |
|                                | 通知の Lambda の関数 `prd-amber-notification`                                   | この関数 1 つだけで、`provided.al2023`、`arm64` (リクエストと実行時間の課金のため、置いておくだけなら \$0。apply の前で、まだないのもよい)                               |

- AWS Config は、アカウント全体で設定の変化を記録している。`enable_runtime` を切り替えるたびに、記録 1 件ごとの小さな料金がかかる (置いておくだけではかからない)
- 構成を変えて、置いておくだけで料金がかかるリソースを足したら、この表と `scripts/cli/checks/` の該当するスクリプトと `scripts/mcp/check.py` と `report-template.md` を一緒に直す。新しい区分なら `scripts/cli/checks/` にファイルを足し、`scripts/cli/check.sh` の一覧に加える

### tfstate の古い版の判定

ライフサイクル (`infra/tfstate/lifecycle.json`) は、`NoncurrentDays: 1` と `NewerNoncurrentVersions: 10` の**両方**を満たした古い版だけを消す。古い版の数だけで判定すると、apply を何回も流した日の直後に、毎回 NG になる。そこで、消えるはずの時刻を過ぎても残っている版があるかで判定する (`scripts/cli/checks/storage.sh` と `scripts/mcp/check.py` で同じ値を出す)。

| 出力の値             | 意味                                                                                                            |
| -------------------- | --------------------------------------------------------------------------------------------------------------- |
| `noncurrent`         | 古い版の数 (削除マーカーを含む)                                                                                 |
| `excess`             | 新しい 10 個より古い版の数 (ライフサイクルが消す対象)                                                           |
| `excessVersions`     | `excess` の版ごとの時刻 (下の 3 つ) を、新しい順に並べたもの                                                    |
| `noncurrentSinceJst` | 古い版になった時刻 (1 つ新しい版の `LastModified`)                                                              |
| `eligibleAtJst`      | 消える条件を両方満たした時刻。次の 2 つの遅いほう                                                               |
|                      | - 日数: `noncurrentSinceJst` に 1 日を足し、次の UTC 0 時 (JST 9 時) に切り上げた時刻                           |
|                      | - 数: それより新しい古い版が 10 個そろった時刻 (新しいほうから 10 番目の古い版が、古い版になった時刻)           |
| `deadlineJst`        | `eligibleAtJst` から 3 日あと                                                                                   |
| `overdue`            | `deadlineJst` を過ぎても残っている版の数                                                                        |
| `verdict`            | `OK` (`excess` が 0) / `OK_PENDING` (`excess` はあるが、どれも `deadlineJst` の前) / `NG` (`overdue` が 1 以上) |

- 待つ日数を 3 日にした理由
  - ライフサイクルは 1 日に 1 回動く。条件を満たしてから、次に動くまで最大で約 1 日かかる
  - 動いたあとの削除は非同期で、完了の時刻は保証されない。小さなバケットでも、遅れを 2 日ぶん見込む
  - 待ちすぎても困らない。tfstate の版の料金は月約 \$0.0002 で、消えるのが数日遅れても料金は変わらない。短くすると、遅れただけで NG になり、調べ直しが増える
- 削除マーカーも古い版の 1 つとして数える。S3 の原文では、削除マーカーが `NewerNoncurrentVersions` の数に入るかを確かめられていない。`OK_PENDING` のまま `deadlineJst` を過ぎて NG になったら、削除マーカーの扱いが想定と違う可能性も調べる
- 出力には今の時刻を含めない。時刻で変わるのは `overdue` と `verdict` だけで、`deadlineJst` を過ぎたときに変わる

#### 参考 (tfstate の古い版の判定)

- DOC: <https://docs.aws.amazon.com/AmazonS3/latest/userguide/intro-lifecycle-rules.html>
- DOC: <https://repost.aws/knowledge-center/s3-lifecycle-rule-delay>
