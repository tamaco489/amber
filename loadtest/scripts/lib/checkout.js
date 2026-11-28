// 注文の確定 (POST /v1/carts/{cart_id}/checkout) の本文を組み立てて送る
//   - 新しいクライアントは unit_price を送らず、古いクライアントは送る
//   - 本文の値はどの繰り返しも同じ。合計はサーバーの単価から決まるため、unit_price の値は結果に効かない
import http from "k6/http";

import { baseUrl, timeout } from "./config.js";

// 注文する品目 (sku は backend の単価の一覧にあるもの)
const sku = "TSHIRT-BLK-M";
const quantity = 2;
const unitPrice = 2980;
const shippingAddressId = "addr_01";

// unit_price を送らない本文は、unit_price を必須とする古い版に 400 で断られうる
//   - その 400 を http_req_failed に数えないよう、200 と 400 を想定どおりとする判定を、このリクエストにだけ渡す
//   - 判定は init で 1 回だけ作り、使い回す
// DOC: https://grafana.com/docs/k6/latest/javascript-api/k6-http/expected-statuses/
// DOC: https://grafana.com/docs/k6/latest/javascript-api/k6-http/params/
const allowBadRequest = http.expectedStatuses(200, 400);

// 本文を組み立てる。withUnitPrice が false なら、品目に unit_price を入れない
function buildBody(withUnitPrice) {
  const item = { sku: sku, quantity: quantity };
  if (withUnitPrice) {
    item.unit_price = unitPrice;
  }
  return { items: [item], shipping_address_id: shippingAddressId };
}

// 注文の確定を 1 件送り、応答を返す
//   - JSON の本文は、文字列にしたうえで Content-Type も付ける (オブジェクトのまま渡すとフォームの形で送られる)
//   - cart_id は毎回作り直す。URL の name タグは 1 つにまとめ、cart_id ごとに時系列を分けない
// DOC: https://grafana.com/docs/k6/latest/javascript-api/k6-http/post/
// DOC: https://grafana.com/docs/k6/latest/javascript-api/k6-http/url/
// DOC: https://grafana.com/docs/k6/latest/javascript-api/crypto/
export function postCheckout(withUnitPrice) {
  const cartId = crypto.randomUUID();
  const params = {
    headers: { "Content-Type": "application/json" },
    tags: { name: "checkout" },
    timeout: timeout,
  };
  if (!withUnitPrice) {
    params.responseCallback = allowBadRequest;
  }
  return http.post(
    `${baseUrl}/v1/carts/${cartId}/checkout`,
    JSON.stringify(buildBody(withUnitPrice)),
    params,
  );
}
