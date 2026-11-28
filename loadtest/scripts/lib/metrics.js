// 自作のメトリクスと、その閾値
//   - メトリクスは init で作る (モジュールの一番上で作れば、import のときに init で作られる)
// DOC: https://grafana.com/docs/k6/latest/using-k6/metrics/create-custom-metrics/
import { Counter } from "k6/metrics";

import { failureKinds } from "./failure.js";
import { versionTags } from "./version.js";

// 失敗の件数 (kind のタグで種類を分ける)
export const failures = new Counter("failures");

// 成功した応答の件数 (version のタグで古い版 (old) か新しい版 (new) かを分ける)
export const responses = new Counter("responses");

// タグ付きの件数は、閾値に書いたものだけがサマリーに出る
//   - 失敗はどの種類も 0 件を条件にする
//   - 新旧ごとの件数は出すだけなので、必ず満たす条件 (count>=0) を置く
// DOC: https://grafana.com/docs/k6/latest/using-k6/thresholds/
export function buildThresholds() {
  const thresholds = {
    failures: ["count==0"],
    http_req_failed: ["rate==0"],
    dropped_iterations: ["count==0"], // 予定の RPS を送れなかった分 (VU が足りない) も、試験の失敗として扱う
  };
  for (const kind of failureKinds) {
    thresholds[`failures{kind:${kind}}`] = ["count==0"];
  }
  for (const version of versionTags) {
    thresholds[`responses{version:${version}}`] = ["count>=0"];
  }
  return thresholds;
}
