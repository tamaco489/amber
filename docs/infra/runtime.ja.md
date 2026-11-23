# 検証環境の停止と再開

[English](./runtime.md) | [日本語](./runtime.ja.md)

[ドキュメント一覧](../README.ja.md)に戻る。

この環境は、デプロイ方式を確かめるための検証用です。動かし続ける必要がないため、**検証するときだけ時間課金のリソースを作り、終わったら消します。**
作るか消すかは、`infra/envs/prd/terraform.tfvars` の `enable_runtime` の 1 つで切り替えます。

## 進め方

```sh
# 1. 検証を始める: enable_runtime = true にして apply する
just infra::apply prd

# 2. 検証する

# 3. 検証を終える: enable_runtime = false にして apply する
just infra::apply prd
```

**検証が終わったら、その日のうちに `false` に戻します。** 動かしたままにすると、1 時間あたり約 \$0.14 (1 日あたり約 \$3.4) かかります。

`false` のまま PR を出せば、`ci-infra` の plan は `No changes` になります。`true` のまま PR を出すと、plan に 37 件の追加が出て、本当の差分が埋もれます。

## 切り替えで作るもの・消すもの

| モジュール | リソース                                                                         | 料金 (東京)                         |
| ---------- | -------------------------------------------------------------------------------- | ----------------------------------- |
| `alb`      | ALB、リスナー、ターゲットグループ、セキュリティグループ                          | 1 時間あたり約 $0.034 (IPv4 を含む) |
| `dns`      | ACM の証明書、Route 53 のレコード (`ecs-lab.tamaco-blog.jp`)                     | 無料                                |
| `network`  | インターフェース型エンドポイント (`ecr.api`、`ecr.dkr`、`logs`) を 2 つの AZ に  | 1 時間あたり約 $0.084               |
| `ecs`      | クラスター、サービス、タスク定義、IAM ロール、オートスケーリング、アラーム、ログ | タスク 2 つで 1 時間あたり約 $0.025 |

合計は、1 時間あたり約 $0.14 です。

ECS は、ALB のルールとターゲットグループを参照し、イメージの pull とログの出力をエンドポイント経由で行います。3 つは一緒にしか使えないため、切り替えを 1 つにまとめています。

## 切り替えに関係なく残るもの

| リソース                                                                              | 料金                                                                                                                                        |
| ------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| VPC、サブネット、IGW、ルートテーブル、セキュリティグループ                            | 無料                                                                                                                                        |
| S3 のゲートウェイ型エンドポイント                                                     | 無料                                                                                                                                        |
| ECR のリポジトリ (`prd-amber-api`)                                                    | イメージ 1 GB あたり月 $0.10。数 MB なので、ほぼ $0                                                                                         |
| plan 用の IAM ロール (`prd-amber-github-actions-plan`)                                | 無料                                                                                                                                        |
| deploy 用の IAM ロール (`prd-amber-github-actions-deploy`)                            | 無料                                                                                                                                        |
| 通知の Lambda のデプロイ用の IAM ロール (`prd-amber-github-actions-notifier`)         | 無料                                                                                                                                        |
| テスト用リスナーのプレフィックスリスト (Terraform の外で管理)                         | 無料                                                                                                                                        |
| tfstate のバケット (`prd-amber-tfstate`)                                              | 古い版はライフサイクルで 10 個まで残す ([tfstate のバケットのライフサイクル](./tfstate.ja.md))。数 MB なので、ほぼ $0                       |
| デプロイの通知 (EventBridge のルール、SQS のキュー)                                   | 呼び出しとリクエストごとの課金のみ。置いておくだけなら $0 ([デプロイの Slack への通知](./notification.ja.md))                               |
| 通知の Lambda (関数、DynamoDB のテーブル、SSM のパラメータ、IAM ロール、ロググループ) | 関数はリクエストと実行時間、テーブルはオンデマンドの課金で、パラメータは標準のため、置いておくだけなら $0。ログは 3 日で消えるため、ほぼ $0 |

止めたあとの料金は、ほぼ $0 (月 1 セント未満) です。

## 作り直したときに変わるもの

切り替えるたびに、リソースは新しく作られます。

- **変わるもの**: ARN、ID、ALB の DNS 名、証明書、ターゲットグループ
- **変わらないもの**: ドメイン (`https://ecs-lab.tamaco-blog.jp`)、リソースの名前、ECR のイメージ

ECS のサービスは、最初は ECR の `bootstrap` のタグのイメージで起動します (Terraform が plan のときにダイジェストを引きます)。CD で別のバージョンをデプロイしていても、作り直すと `bootstrap` に戻ります。続けて検証するバージョンは、作り直したあとに CD (`workflow_dispatch` でタグを選ぶ) で流し直します。

`bootstrap` のイメージは、ECR のライフサイクルで消えないようにしています (古いものを消すのは `v*` のタグだけ)。消えると、`enable_runtime = true` の plan がエラーになります。push し直すには `just backend::push bootstrap` を実行します。

## 新しいリソースを足すとき

時間課金のリソースや、置いておくだけで料金がかかるリソースは、`enable_runtime` で消えるモジュールに入れます。

- 時間課金: ALB、NAT ゲートウェイ、インターフェース型エンドポイント、Fargate のタスク
- 置いておくだけでかかるもの: CloudWatch アラーム (1 個あたり月 $0.10)

## 参考

- [Elastic Load Balancing の料金](https://aws.amazon.com/elasticloadbalancing/pricing/)
- [AWS PrivateLink の料金](https://aws.amazon.com/privatelink/pricing/)
- [AWS Fargate の料金](https://aws.amazon.com/fargate/pricing/)
- [Amazon VPC の料金 (パブリック IPv4 アドレス)](https://aws.amazon.com/vpc/pricing/)
- [Amazon CloudWatch の料金](https://aws.amazon.com/cloudwatch/pricing/)
