package main

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
)

// 注文の確定の本文の大きさの上限
const maxCheckoutBody = 1 << 20

// checkoutResponse は、注文を確定できたときの応答
type checkoutResponse struct {
	OrderID string `json:"order_id"` // 注文の ID
	Total   int    `json:"total"`    // 合計 (円)。サーバーの単価から決める
	Version string `json:"version"`  // 処理した版
}

// errorResponse は、注文を確定できなかったときの応答
type errorResponse struct {
	Error   string `json:"error"`   // 確定できなかった理由
	Version string `json:"version"` // 処理した版
}

// カートの中身で注文を確定する
//   - DB は使わない。カートの中身は本文で受け、注文は残さない
//   - 応答には、成功でも失敗でも version を入れる (新旧が混ざったときに、どちらの版が返したかを数えるため)
func handleCheckout(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req checkoutRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCheckoutBody)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, &errorResponse{Error: "body must be a JSON object", Version: version})
			return
		}
		if err := req.validate(); err != nil {
			writeJSON(w, http.StatusBadRequest, &errorResponse{Error: err.Error(), Version: version})
			return
		}

		writeJSON(w, http.StatusOK, &checkoutResponse{
			OrderID: "ord_" + rand.Text(),
			Total:   totalPrice(req.Items),
			Version: version,
		})
	}
}
