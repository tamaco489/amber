package checkout

// response は、注文を確定できたときの応答
type response struct {
	OrderID string `json:"order_id"` // 注文の ID
	Total   int    `json:"total"`    // 合計 (円)。サーバーの単価から決める
	Version string `json:"version"`  // 処理した版
}

// errorResponse は、注文を確定できなかったときの応答
type errorResponse struct {
	Error   string `json:"error"`   // 確定できなかった理由
	Version string `json:"version"` // 処理した版
}
