// 失敗の種類と、応答から種類への分け方
//   - timeout: 決めた時間内に応答が無い (回線側か、ALB の先で詰まっている)
//   - network: DNS・TCP・TLS の失敗、接続の切断 (回線側)
//   - server: HTTP 5xx (ALB かアプリ側。ALB の 5xx の件数と突き合わせる)
//   - client: 200 以外のそのほかの応答 (4xx など)
//   - body: 200 だが、本文からバージョン番号を読めない
export const failureKinds = ["timeout", "network", "server", "client", "body"];

// k6 のタイムアウトのエラーコード
//   - 1050: リクエストのタイムアウト、1211: 接続 (dial) のタイムアウト
// DOC: https://grafana.com/docs/k6/latest/javascript-api/error-codes/
const timeoutCodes = [1050, 1211];

// 応答のステータスとエラーコードから、失敗の種類を返す。失敗でなければ空の文字列を返す
//   - body の失敗は、本文を読んだあとに決まるため、ここでは返さない
export function classify(res) {
  if (res.error_code !== 0 && timeoutCodes.includes(res.error_code)) {
    return "timeout";
  }
  if (res.status === 0) {
    return "network";
  }
  if (res.status >= 500) {
    return "server";
  }
  if (res.status !== 200) {
    return "client";
  }
  return "";
}
