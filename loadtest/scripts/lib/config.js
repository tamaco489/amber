// スクリプトに渡す値 (justfile から k6 の -e で渡す) を読み、既定値を置く
//   - __ENV はこのファイルだけで読む
//   - k6 inspect は、コンテナの環境変数 (docker run -e) を __ENV に入れない。k6 の -e で渡す
//
// 値
//   - BASE_URL: 送り先 (既定は手元のコンテナ。prd の URL は既定に置かず、justfile の target で選ぶ)
//   - RATE: 1 秒あたりのリクエスト数
//   - DURATION: 実行時間 (例: 5m)
//   - TIMEOUT: 1 件あたりのタイムアウト
// DOC: https://grafana.com/docs/k6/latest/using-k6/environment-variables/
export const baseUrl = __ENV.BASE_URL || "http://host.docker.internal:8080";
export const rate = Number(__ENV.RATE || "10");
export const duration = __ENV.DURATION || "5m";
export const timeout = __ENV.TIMEOUT || "10s";
