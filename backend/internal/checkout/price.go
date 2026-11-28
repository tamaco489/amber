package checkout

// 商品ごとの単価 (円)
//   - DB は使わず、値段はコードの中に持つ
//   - 合計は、クライアントが送る unit_price ではなく、この単価から決める
var prices = map[string]int{
	"TSHIRT-BLK-M": 2980,
	"TSHIRT-WHT-L": 2980,
	"MUG-RED":      1200,
}

// totalPrice は、品目ごとの単価 × 数量を足し合わせる
//   - 品目は validate を通したもの (SKU が prices にあり、数量が入っている) を渡す
func totalPrice(items []item) int {
	total := 0
	for _, it := range items {
		total += prices[*it.SKU] * *it.Quantity
	}
	return total
}
