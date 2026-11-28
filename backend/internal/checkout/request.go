package checkout

// request は、注文の確定の本文
//   - 項目があるかを見分けるため、ポインタで受ける (encoding/json には「必須」の仕組みがない)
type request struct {
	Items             []item  `json:"items"`               // 注文する品目
	ShippingAddressID *string `json:"shipping_address_id"` // 届け先の ID
}

// item は、注文する品目 1 つ
type item struct {
	SKU       *string `json:"sku"`        // 商品の ID (prices のキー)
	Quantity  *int    `json:"quantity"`   // 数量
	UnitPrice *int    `json:"unit_price"` // クライアントが送る単価。合計には使わない
}
