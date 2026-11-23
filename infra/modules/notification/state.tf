# ================================================================================
# State Table
# ================================================================================

# 通知の Lambda が、デプロイごとの状態を残すテーブル
#   - パーティションキーは pk (文字列)。1 つのテーブルに、種類の違う項目を入れるため、汎用の名前にする
#     - デプロイごとの親の投稿 (ts など): pk = deployment#<デプロイ ID>
#     - 手順ごとの済んだ印 (流し直しても同じ返信を 2 回付けないため): pk = step#<キー>
#   - 項目の形は Lambda の本体で決める
#   - 古い項目は、TTL (expires_at、エポック秒) で消す。TTL での削除に料金はかからない
#   - オンデマンドのため、置いておくだけでは料金はかからない (リクエストと保存量の課金)
#   - ポイントインタイムリカバリは有料で、消えても次のデプロイの通知が新しいスレッドになるだけのため、使わない
# DOC: https://aws.amazon.com/dynamodb/pricing/on-demand/
resource "aws_dynamodb_table" "notifier_state" {
  name         = "${local.name_prefix}-notification"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "pk"

  attribute {
    name = "pk"
    type = "S"
  }

  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }
}
