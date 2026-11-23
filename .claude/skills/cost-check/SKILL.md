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

| ファイル                             | 役割                                                                                                                           |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------ |
| `SKILL.md`                           | メインのセッションの手順と、判断の表                                                                                           |
| `report-template.md`                 | レポートの雛形。エージェントが埋める                                                                                           |
| `scripts/cli/check.sh`               | CLI での確認の入り口。`checks/` を順に流す (読み取りだけ)                                                                      |
| `scripts/cli/lib.sh`                 | 共通の設定 (プロファイル、リージョン、JST) と関数 (失敗の検出、認証の確認)                                                     |
| `scripts/cli/checks/terraform.sh`    | `enable_runtime`、state のロック、`terraform plan`、state に残る時間課金のモジュール                                           |
| `scripts/cli/checks/compute.sh`      | 時間課金: ECS、ELB、VPC エンドポイント、NAT、EIP、EC2、EBS、ENI、オートスケーリング                                            |
| `scripts/cli/checks/monitoring.sh`   | 置いておくと課金: CloudWatch アラーム、ロググループ (通知の Lambda のものは分けて出す)                                         |
| `scripts/cli/checks/dns.sh`          | Route 53 のレコード、ACM の証明書                                                                                              |
| `scripts/cli/checks/notification.sh` | 通知: EventBridge、デッドレターキュー、消した接続とそのシークレット、Lambda の関数、状態のテーブル、ボットトークンのパラメータ |
| `scripts/cli/checks/storage.sh`      | ほぼ \$0: ECR、タスク定義、tfstate のバケット                                                                                  |
| `scripts/mcp/check.py`               | AWS MCP の `run_script` にそのまま渡す、固定の確認のコード                                                                     |
| `scripts/verify/verify.sh`           | メインのセッションの検証 (記入漏れ、時刻、CLI の確認の流し直しと突き合わせ)                                                    |
| `.shellcheckrc`                      | `checks/` が `lib.sh` を読み込むのを、shellcheck にたどらせる設定                                                              |
| `.claude/agents/cost-auditor.md`     | 調査するエージェントの定義                                                                                                     |

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
3. **CLI と MCP が一致するか。** 時間課金の行とアラームは、両方で確かめている。アカウントも一致している。片方だけで判定した行があれば、差し戻す
4. **結論と矛盾しないか。** NG や未確認の行があるのに「残っていない」としていない。state が `LOCKED` なら、結論は未確認になっている
5. **作業ツリーが変わっていないか。** `git status --porcelain` が、手順 1 と同じ (エージェントがレポート以外を書いていない)

## 判断の表

| 区分                           | リソース                                                                        | OK の条件                                                                                                                                                                    |
| ------------------------------ | ------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 前提                           | アカウント、state のロック、`terraform plan`                                    | amber のアカウント。`NOT_LOCKED`。`No changes`                                                                                                                               |
| 時間課金 (残っていたら NG)     | ECS のクラスター・サービス・タスク                                              | ACTIVE なクラスターがない (`INACTIVE`、または一覧にない)                                                                                                                     |
|                                | ALB、ターゲットグループ                                                         | `prd-amber` を含むものがない                                                                                                                                                 |
|                                | インターフェース型の VPC エンドポイント                                         | amber の VPC にない (Gateway の S3 だけ)                                                                                                                                     |
|                                | NAT、EC2、ENI (amber の VPC)、EIP、EBS (タグ `project=amber`)                   | ない                                                                                                                                                                         |
|                                | オートスケーリングの対象                                                        | `prd-amber` を含むものがない                                                                                                                                                 |
| 置いておくと課金 (残ったら NG) | CloudWatch アラーム (月 \$0.10/個)                                              | `prd-amber` を含むものがない (`TargetTracking-service/prd-amber/...` も含む)                                                                                                 |
|                                | ロググループ                                                                    | `prd-amber` を含むものがない (通知の Lambda の `/aws/lambda/prd-amber-notification` は除く)                                                                                  |
|                                | Route 53 のレコード、ACM の証明書                                               | `ecs-lab` を含むものがない                                                                                                                                                   |
| ほぼ \$0 (残ってよい)          | ECR `prd-amber-api`                                                             | 合計 100 MB 未満                                                                                                                                                             |
|                                | tfstate のバケット `prd-amber-tfstate`                                          | key ごとの古い版が 10 個以下。超えていたら NG (ライフサイクルの反映の途中でも NG)。料金はほぼ \$0 のため止める操作は要らず、料金の目安と、ライフサイクルが入っているかを書く |
|                                | 通知の Lambda のロググループ `/aws/lambda/prd-amber-notification`               | 保存期間が 3 日。`enable_runtime` で消さずに常に置く (apply の前で、まだないのもよい)                                                                                        |
| 無料 (残ってよい)              | VPC、サブネット、IGW、ルートテーブル、セキュリティグループ、S3 のエンドポイント | -                                                                                                                                                                            |
|                                | IAM ロール、タスク定義、プレフィックスリスト                                    | -                                                                                                                                                                            |
|                                | 通知 (EventBridge のルール、SQS)                                                | デッドレターキューが 0 件。EventBridge の接続と API 送信先がない (Lambda に切り替えたときに消した)                                                                           |
|                                | Secrets Manager の `prd-amber` を含むシークレット                               | ない (接続と一緒に `events!connection/prd-amber-slack/...` が消えている。残っていたら NG)                                                                                    |
|                                | DynamoDB の通知の状態のテーブル `prd-amber-notification`                        | このテーブル 1 つだけで、`PAY_PER_REQUEST` (オンデマンド)                                                                                                                    |
|                                | SSM のパラメータ `/prd-amber/notification/slack-bot-token`                      | このパラメータ 1 つだけで、`Standard` (標準)                                                                                                                                 |
|                                | 通知の Lambda の関数 `prd-amber-notification`                                   | この関数 1 つだけで、`provided.al2023`、`arm64` (リクエストと実行時間の課金のため、置いておくだけなら \$0。apply の前で、まだないのもよい)                                   |

- AWS Config は、アカウント全体で設定の変化を記録している。`enable_runtime` を切り替えるたびに、記録 1 件ごとの小さな料金がかかる (置いておくだけではかからない)
- 構成を変えて、置いておくだけで料金がかかるリソースを足したら、この表と `scripts/cli/checks/` の該当するスクリプトと `scripts/mcp/check.py` と `report-template.md` を一緒に直す。新しい区分なら `scripts/cli/checks/` にファイルを足し、`scripts/cli/check.sh` の一覧に加える
