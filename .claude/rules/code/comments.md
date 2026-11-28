# コードのコメント

## 書かないこと

コード (Terraform、シェル、ワークフロー、Go、k6 など) のコメントに、PR や Issue の番号 (`PR #58`、`#71`) と、作業の経緯 (「PR で手元で確かめた」など) を書かない。

- コメントには、コードが今なぜそうなっているかだけを書く。経緯や確かめた結果は、PR の本文と Issue に残す
- 番号はコードを読む時点では意味を持たず、あとで PR や Issue が増えると、どれを指すのか分からなくなるため

```js
// NG
// 本文が JSON でないときは Response.json() が例外を出すため、try/catch で受ける (手元で確かめた。PR #58)

// OK
// 本文が JSON でないときは Response.json() が例外を出すため、try/catch で受ける
```

## 出典の書き方

コード (Terraform、シェル、ワークフロー、Go など) のコメントで出典の URL を書くときは、`DOC:` を先頭に付ける。`Ref:` や `参考:` は使わない。

```hcl
# DOC: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_deployment_events.html
```

- 1 行に 1 つの URL を書く。複数あるときは行を分ける
- 説明のコメントの最後 (リソースの定義の直前) に置く
- `docs/` の Markdown は対象外 (「参考」の節にリンクで書く)
