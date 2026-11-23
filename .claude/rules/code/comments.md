# コードのコメント

## 出典の書き方

コード (Terraform、シェル、ワークフロー、Go など) のコメントで出典の URL を書くときは、`DOC:` を先頭に付ける。`Ref:` や `参考:` は使わない。

```hcl
# DOC: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html
```

- 1 行に 1 つの URL を書く。複数あるときは行を分ける
- 説明のコメントの最後 (リソースの定義の直前) に置く
- `docs/` の Markdown は対象外 (「参考」の節にリンクで書く)
