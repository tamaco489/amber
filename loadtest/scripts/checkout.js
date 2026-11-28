// 注文の確定 API から必須の項目 (unit_price) を外す版を出す間に、新旧のクライアントの応答を数える
//   - 新しいクライアント (unit_price を送らない) は、古い版に当たると 400 になる。古い版が返したその 400 は想定どおりとし、失敗に数えない
//   - 古いクライアント (unit_price を送る) は、どちらの版でも 200 になる
//   - シナリオ (送り手) × 処理した版 (old/new) × ステータスごとの件数を、サマリーに出す (lib/checkout-metrics.js)
//   - 古い版は setup で 1 回読み、繰り返しに渡す。読めなければ試験を始めずに止める (lib/old-version.js)
//   - 想定外の応答が 1 件でもあれば閾値を超え、k6 は 99 で終わる
//   - 渡す値 (BASE_URL、RATE、DURATION、TIMEOUT) は lib/config.js。RATE はシナリオごとの RPS
import { countNewClient, countOldClient } from "./lib/checkout-count.js";
import { buildCheckoutThresholds } from "./lib/checkout-metrics.js";
import { postCheckout } from "./lib/checkout.js";
import { duration, rate } from "./lib/config.js";
import { readOldVersion } from "./lib/old-version.js";

// 前もって作る VU の数は「1 回の繰り返しの時間 × RPS + ゆとり」を目安にする (シナリオごと)
//   - 1 回の繰り返しは POST 1 件 (0.1 秒ほど) なので、RPS の 2 倍あれば、応答が 2 秒まで遅れても足りる
//   - 途中で VU を足すと結果がゆがむため、maxVUs は小さなゆとり (10) にとどめる
//   - それでも足りない分は dropped_iterations に出て、閾値で失敗になる
// DOC: https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/arrival-rate-vu-allocation/
const preAllocatedVUs = Math.max(rate * 2, 10);

// 2 つのクライアントは同じ RPS で、どちらもデプロイの前から同時に流す (startTime は付けない)
//   - Web のフロントが先に新しくなり、新しいクライアントが古い版に当たる場面に近い
//   - デプロイの前の 400 が、すべて古い版から返ったことを確かめられる
//   - 新しい版に切り替わったあと、400 が 0 件になることも、同じ流れで見られる
// constant-arrival-rate は、応答の速さに関係なく、決めた RPS で送り続ける
// シナリオ名は scenario のタグとして、メトリクスに付く
// DOC: https://grafana.com/docs/k6/latest/using-k6/scenarios/
// DOC: https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/
const scenario = {
  executor: "constant-arrival-rate",
  rate: rate,
  timeUnit: "1s",
  duration: duration,
  preAllocatedVUs: preAllocatedVUs,
  maxVUs: preAllocatedVUs + 10,
};

export const options = {
  scenarios: {
    new_client: { ...scenario, exec: "newClient" },
    old_client: { ...scenario, exec: "oldClient" },
  },
  thresholds: buildCheckoutThresholds(["new_client", "old_client"]),
};

// setup の戻り値は JSON にできる値だけを渡せる。各 VU は写しを受け取る
// DOC: https://grafana.com/docs/k6/latest/using-k6/test-lifecycle/
export function setup() {
  return { oldVersion: readOldVersion() };
}

// 新しいクライアント: unit_price を送らない
export function newClient(data) {
  countNewClient(postCheckout(false), data.oldVersion);
}

// 古いクライアント: unit_price を送る
export function oldClient(data) {
  countOldClient(postCheckout(true), data.oldVersion);
}
