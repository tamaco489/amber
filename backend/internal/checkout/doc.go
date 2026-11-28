// Package checkout は、注文の確定 (POST /v1/carts/{cart_id}/checkout) を持つ
//   - DB は使わず、商品の単価はコードの中に持つ
//   - 必須の項目はハンドラーの中で確かめ、足りなければ 400 を返す (一覧は validate.go)
//   - 必須の項目を変えた版を出し、新旧のクライアントが混ざったときに何が起きるかを確かめるために使う
package checkout
