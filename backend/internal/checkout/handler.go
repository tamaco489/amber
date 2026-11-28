package checkout

import (
	"crypto/rand"
	"encoding/json"
	"net/http"

	"github.com/tamaco489/amber/backend/internal/server"
)

// 注文の確定の本文の大きさの上限
const maxBody = 1 << 20

// Handle は、カートの中身で注文を確定する (POST /v1/carts/{cart_id}/checkout)
//   - DB は使わない。カートの中身は本文で受け、注文は残さない
//   - 応答には、成功でも失敗でも version を入れる (新旧が混ざったときに、どちらの版が返したかを数えるため)
func Handle(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&req); err != nil {
			server.WriteJSON(w, http.StatusBadRequest, &errorResponse{Error: "body must be a JSON object", Version: version})
			return
		}
		if err := req.validate(); err != nil {
			server.WriteJSON(w, http.StatusBadRequest, &errorResponse{Error: err.Error(), Version: version})
			return
		}

		server.WriteJSON(w, http.StatusOK, &response{
			OrderID: "ord_" + rand.Text(),
			Total:   totalPrice(req.Items),
			Version: version,
		})
	}
}
