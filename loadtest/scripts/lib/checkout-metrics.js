// 注文の確定の応答を数えるメトリクスと、その閾値
//   - メトリクスは init で作る (モジュールの一番上で作れば、import のときに init で作られる)
//   - 失敗は、availability と同じ failures (kind のタグで種類を分ける) に数える
// DOC: https://grafana.com/docs/k6/latest/using-k6/metrics/create-custom-metrics/
import { Counter } from "k6/metrics";

import { failureKinds } from "./failure.js";
import { versionTags } from "./version.js";

// 本文から版を読めた応答の件数
//   - version のタグで古い版 (old) か新しい版 (new) か、status のタグでステータスコードを分ける
//   - シナリオ (送り手) は、k6 が自動で付ける scenario のタグで分ける
//   - 失敗かどうかに関わらず数える (どの版がどのステータスを返したかを見るため)
export const checkoutResponses = new Counter("checkout_responses");

// サマリーに件数を出すステータスコード
//   - 200: 確定できた、400: 必須の項目が足りない (古い版が unit_price なしを断る)
//   - ほかのステータスはここに並べず、失敗 (failures) の件数とログで見る
const statusTags = ["200", "400"];

// タグ付きの件数は、閾値に書いたものだけがサマリーに出る
//   - 失敗はどの種類も、どのシナリオも 0 件を条件にする
//   - シナリオ × 版 × ステータスの件数は出すだけなので、必ず満たす条件 (count>=0) を置く
// DOC: https://grafana.com/docs/k6/latest/using-k6/thresholds/
export function buildCheckoutThresholds(scenarioNames) {
  const thresholds = {
    failures: ["count==0"],
    http_req_failed: ["rate==0"],
    dropped_iterations: ["count==0"], // 予定の RPS を送れなかった分 (VU が足りない) も、試験の失敗として扱う
  };
  for (const kind of failureKinds) {
    thresholds[`failures{kind:${kind}}`] = ["count==0"];
  }
  for (const scenario of scenarioNames) {
    thresholds[`failures{scenario:${scenario}}`] = ["count==0"];
    for (const version of versionTags) {
      for (const status of statusTags) {
        thresholds[
          `checkout_responses{scenario:${scenario},version:${version},status:${status}}`
        ] = ["count>=0"];
      }
    }
  }
  return thresholds;
}
