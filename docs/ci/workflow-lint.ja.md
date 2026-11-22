# ワークフローとスクリプトの検査

[English](./workflow-lint.md) | [日本語](./workflow-lint.ja.md)

[ドキュメント一覧](../README.ja.md)に戻る。

GitHub Actions のワークフローと、そこから呼ぶシェルスクリプトを、`ci-actions` で検査します。

## 検査の内容

| 対象                                      | ツール     | 見るもの                                                 |
| ----------------------------------------- | ---------- | -------------------------------------------------------- |
| `.github/workflows/*.yaml`                | actionlint | 構文、`${{ }}` の式、action の入力、runner のラベルなど  |
| ワークフローの `run:` の中のシェル        | shellcheck | クォートの漏れ、未定義の変数など (actionlint が呼び出す) |
| `.github/workflows/scripts/` 配下の `.sh` | shellcheck | 同上 (直接検査する)                                      |

- 指摘が 1 件でもあると失敗します。shellcheck の `info` も含みます
- `.github/workflows/` 配下を変えた PR と、`main` への push で動きます
- バージョンは `ci-actions.yaml` の `env` で固定しています (actionlint 1.7.12、shellcheck 0.11.0)。ダウンロードしたファイルはチェックサムで確かめます。上げるときは、チェックサムも書き換えます

## 手元での流し方

actionlint と shellcheck を入れて (例: `brew install actionlint shellcheck`)、リポジトリの直下で実行します。

```sh
just actions::check       # 下の 2 つをまとめて実行する
just actions::lint        # actionlint だけ
just actions::shellcheck  # scripts/ 配下の .sh だけ
```

## 誤検知を抑える

意図した書き方で指摘が出るときは、その行の直前に `# shellcheck disable=<コード>` を、理由のコメントと一緒に書きます。

例: `cd-backend.yaml` の「Update service」は、JMESPath の `` `PRIMARY` `` を展開させないため、`--query` をわざとシングルクォートで囲んでいます。そのため SC2016 を抑えています。
