// デプロイ中に一定の RPS でリクエストを送り、失敗が 0 件であることを確かめる
//   - 失敗は種類ごとに分けて数える (timeout・network は回線側、server は ALB かアプリ側。lib/failure.js)
//   - 古い版 (流し始めた時点の版) と新しい版の件数を数える (切り替えの途中で新旧がどれだけ混ざったかを見るため)
//   - 古い版は setup で 1 回読み、繰り返しに渡す。読めなければ試験を始めずに止める (lib/old-version.js)
//   - 1 件でも失敗すれば閾値を超え、k6 は 99 で終わる
//   - 渡す値 (BASE_URL、RATE、DURATION、TIMEOUT) は lib/config.js
import { duration, rate } from "./lib/config.js";
import { countResponse } from "./lib/count.js";
import { buildThresholds } from "./lib/metrics.js";
import { readOldVersion } from "./lib/old-version.js";
import { getRoot } from "./lib/root.js";

// 前もって作る VU の数は「1 回の繰り返しの時間 × RPS + ゆとり」を目安にする
//   - 1 回の繰り返しは GET 1 件 (0.1 秒ほど) なので、RPS の 2 倍あれば、応答が 2 秒まで遅れても足りる
//   - 途中で VU を足すと結果がゆがむため、maxVUs は小さなゆとり (10) にとどめる
//   - それでも足りない分は dropped_iterations に出て、閾値で失敗になる
// DOC: https://grafana.com/docs/k6/latest/using-k6/scenarios/concepts/arrival-rate-vu-allocation/
const preAllocatedVUs = Math.max(rate * 2, 10);

// constant-arrival-rate は、応答の速さに関係なく、決めた RPS で送り続ける
//   - 応答が遅くなっても送る数が減らないため、デプロイ中の詰まりを見逃さない
// DOC: https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/
export const options = {
  scenarios: {
    availability: {
      executor: "constant-arrival-rate",
      rate: rate,
      timeUnit: "1s",
      duration: duration,
      preAllocatedVUs: preAllocatedVUs,
      maxVUs: preAllocatedVUs + 10,
    },
  },
  thresholds: buildThresholds(),
};

// setup の戻り値は JSON にできる値だけを渡せる。各 VU は写しを受け取る
// DOC: https://grafana.com/docs/k6/latest/using-k6/test-lifecycle/
export function setup() {
  return { oldVersion: readOldVersion() };
}

export default function (data) {
  countResponse(getRoot(), data.oldVersion);
}
