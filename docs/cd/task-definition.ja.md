# タスク定義の変更の反映

[English](./task-definition.md) | [日本語](./task-definition.ja.md)

[ドキュメント一覧](../README.ja.md)に戻る。

Terraform でタスク定義 (CPU、メモリ、環境変数、ログなど) を変えたあと、それをサービスに届ける手順です。

## 仕組み

- ECS のサービスは、Terraform で `ignore_changes = [task_definition]` にしています。apply するとタスク定義に新しいリビジョンができますが、**サービスはそれに切り替わりません。** plan にも差分は出ません
- `cd-backend` は、ファミリー (`prd-amber-api`) の最新の ACTIVE なリビジョンを複製し、イメージだけを差し替えて新しいリビジョンを登録します。そのため、Terraform の変更は次のデプロイで届きます
- Terraform が作るリビジョンのイメージは `bootstrap` です。そのリビジョンを直接デプロイすると、アプリが `bootstrap` に戻ります。イメージは必ず `cd-backend` で差し替えます

## 手順

1. Terraform でタスク定義を変えて apply する (`just infra::apply prd`)。ファミリーに新しいリビジョン (`:N`) ができる
2. 今動いているタグを確かめる (`https://ecs-lab.tamaco-blog.jp/` が返すバージョン、または直前の `cd-backend` の実行結果のサマリー)
3. GitHub の Actions で `cd-backend` を開き、`Run workflow` の ref にそのタグを選んで実行する (`workflow_dispatch`)
4. ログの `copied from` に `:N` が出ること、実行結果のサマリーのタスク定義が `:N` より新しい番号であることを確かめる

新しいタグを push してデプロイする場合も、同じように Terraform の変更が乗ります。

> [!NOTE]
> 同じタグを流し直すときは、ECR にあるイメージを使い、ビルドはしません。
