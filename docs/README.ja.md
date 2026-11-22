# ドキュメント一覧

[English](./README.md) | [日本語](./README.ja.md)

## このリポジトリについて

ECS Fargate のデプロイ方式を、実際に動かして確かめるための検証用リポジトリです。

ECS は、CodeDeploy を使わずにブルー/グリーン (2025 年 7 月)、カナリアとリニア (2025 年 10 月) のデプロイを行えるようになりました (ECS ネイティブのデプロイ)。

参考:

- [Amazon ECS blue/green deployments (AWS 公式)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-type-blue-green.html)
- [Amazon ECS canary deployments (AWS 公式)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/canary-deployment.html)
- [Amazon ECS linear deployments (AWS 公式)](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-type-linear.html)
- [Amazon ECS で組み込みのブルー/グリーンデプロイが利用可能に (2025 年 7 月)](https://aws.amazon.com/about-aws/whats-new/2025/07/amazon-ecs-built-in-blue-green-deployments/)
- [Amazon ECS でリニアデプロイとカナリアデプロイが標準機能としてサポート開始 (2025 年 10 月)](https://aws.amazon.com/about-aws/whats-new/2025/10/amazon-ecs-built-in-linear-canary-deployments/)

このリポジトリでは、小さな Go のアプリを Fargate で動かし、デプロイの方式ごとに、切り替えとロールバックがどう動くかを確かめます。

## 目的

- **方式ごとの挙動を知る**: ローリング、ブルー/グリーン、カナリア、リニアで、通信がいつ、どの割合で新しい版に切り替わるか
- **ロールバックの条件を知る**: サーキットブレーカー (タスクが起動できないとき) と CloudWatch アラーム (5xx が増えたとき) が、それぞれどの時点で働くか
- **デプロイに介入する方法を知る**: ライフサイクルフック (Lambda) と PAUSE フックで、決まった地点で止める、確認を差し込む、承認してから進める
- **利用者への影響を測る**: 負荷をかけながらデプロイし、エラーや遅延が出るかを k6 で確かめる

確かめた結果は、設定を選ぶときの判断の材料として残します。

## 対象外

- 本番のサービスの運用。検証が終わったら壊す環境で、動かすのは検証するときだけ ([検証環境の停止と再開](./infra/runtime.ja.md))
- CodeDeploy を使ったデプロイ
- 複数の環境 (dev / stg) の構成。環境は `prd` の 1 つだけ

## 利用ツール

手元で使うツールです。Go、golangci-lint、Terraform、tflint、trivy のバージョンは `.tool-versions` (asdf) で固定し、手元と CI の両方がここを読みます。
actionlint と shellcheck は `ci-actions.yaml` で固定しています。それ以外は固定しておらず、動作を確かめたバージョンを書いています。

| ツール        | バージョン | 用途                                                           |
| ------------- | ---------- | -------------------------------------------------------------- |
| Go            | 1.27.4     | 検証用のアプリ (`/health` と、バージョン番号を返す `/`)        |
| golangci-lint | 2.15.0     | Go の lint と整形の検査                                        |
| Docker        | 29.8.2     | `linux/arm64` のイメージのビルド                               |
| Terraform     | 1.17.2     | AWS のリソースの定義                                           |
| tflint        | 0.66.0     | Terraform の lint (AWS のルールセットを含む)                   |
| trivy         | 0.76.2     | Terraform の設定の脆弱性の検査                                 |
| AWS CLI       | 2.40.0     | 状態の確認、プレフィックスリストの更新、ECR へのログイン       |
| k6            | -          | デプロイ中の負荷試験                                           |
| just          | 1.46.0     | 作業のレシピ (`just backend::push`、`just infra::plan` など)   |
| actionlint    | 1.7.12     | GitHub Actions のワークフローの検査 (`ci-actions.yaml` で固定) |
| shellcheck    | 0.11.0     | シェルスクリプトの検査 (`ci-actions.yaml` で固定)              |
| asdf          | 0.18.0     | ツールのバージョンの管理 (`.tool-versions`)                    |

## ドキュメント

| ドキュメント                                               | 内容                                                                      |
| ---------------------------------------------------------- | ------------------------------------------------------------------------- |
| [検証環境の停止と再開](./infra/runtime.ja.md)              | `enable_runtime` による時間課金リソースの作成と削除、常に残るリソース     |
| [タスク定義の変更の反映](./cd/task-definition.ja.md)       | Terraform で変えたタスク定義を、`cd-backend` でサービスに届ける手順       |
| [ワークフローとスクリプトの検査](./ci/workflow-lint.ja.md) | `ci-actions` が actionlint と shellcheck で検査する内容と、手元での流し方 |
| [v* のタグのルールセット](./github/tag-ruleset.ja.md)      | デプロイの起点の `v*` のタグを、作成・更新・削除できる人を管理者に絞る    |
