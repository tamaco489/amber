// デプロイ中に一定の RPS でリクエストを送り、失敗が 0 件であることを確かめる
//   - 失敗は種類ごとに分けて数える (timeout・network は回線側、server は ALB かアプリ側)
//   - 返ってきたバージョン番号ごとの件数を数える (新旧の版がいつ、どの割合で切り替わったかを見るため)
//   - 1 件でも失敗すれば閾値を超え、k6 は 0 以外のコードで終わる
//
// 環境変数 (justfile から渡す)
//   - BASE_URL: 送り先 (既定は https://ecs-lab.tamaco-blog.jp)
//   - RATE: 1 秒あたりのリクエスト数
//   - DURATION: 実行時間 (例: 5m)
//   - VERSIONS: 件数を出すバージョン番号をカンマ区切りで (例: v1,v2)。ほかの番号は unknown として数える
//   - TIMEOUT: 1 件あたりのタイムアウト
import http from "k6/http";
import { Counter } from "k6/metrics";

const baseUrl = __ENV.BASE_URL || "https://ecs-lab.tamaco-blog.jp";
const rate = Number(__ENV.RATE || "10");
const duration = __ENV.DURATION || "5m";
const versions = (__ENV.VERSIONS || "v1,v2")
  .split(",")
  .map((v) => v.trim())
  .filter((v) => v !== "");
const timeout = __ENV.TIMEOUT || "10s";

// 失敗の種類
//   - timeout: 決めた時間内に応答が無い (回線側か、ALB の先で詰まっている)
//   - network: DNS・TCP・TLS の失敗、接続の切断 (回線側)
//   - server: HTTP 5xx (ALB かアプリ側。ALB の 5xx の件数と突き合わせる)
//   - client: 200 以外のそのほかの応答 (4xx など)
//   - body: 200 だが、本文からバージョン番号を読めない
const failureKinds = ["timeout", "network", "server", "client", "body"];

const failures = new Counter("failures");
const responses = new Counter("responses");

// k6 のタイムアウトのエラーコード
//   - 1050: リクエストのタイムアウト、1211: 接続 (dial) のタイムアウト
// DOC: https://grafana.com/docs/k6/latest/javascript-api/error-codes/
const timeoutCodes = [1050, 1211];

// タグ付きの件数は、閾値に書いたものだけがサマリーに出る
//   - 失敗はどの種類も 0 件を条件にする
//   - バージョンごとの件数は出すだけなので、必ず満たす条件 (count>=0) を置く
function buildThresholds() {
  const thresholds = {
    failures: ["count==0"],
    http_req_failed: ["rate==0"],
    // 予定の RPS を送れなかった分 (VU が足りない) も、試験の失敗として扱う
    dropped_iterations: ["count==0"],
  };
  for (const kind of failureKinds) {
    thresholds[`failures{kind:${kind}}`] = ["count==0"];
  }
  for (const version of [...versions, "unknown"]) {
    thresholds[`responses{version:${version}}`] = ["count>=0"];
  }
  return thresholds;
}

// constant-arrival-rate は、応答の速さに関係なく、決めた RPS で送り続ける
//   - 応答が遅くなっても送る数が減らないため、デプロイ中の詰まりを見逃さない
// DOC: https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/
export const options = {
  scenarios: {
    deploy: {
      executor: "constant-arrival-rate",
      rate: rate,
      timeUnit: "1s",
      duration: duration,
      preAllocatedVUs: Math.max(rate * 2, 10),
      maxVUs: Math.max(rate * 10, 50),
    },
  },
  thresholds: buildThresholds(),
};

function classify(res) {
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

function readVersion(res) {
  try {
    const v = res.json("version");
    return typeof v === "string" ? v : "";
  } catch {
    return "";
  }
}

export default function () {
  const res = http.get(`${baseUrl}/`, { timeout: timeout });

  const kind = classify(res);
  if (kind !== "") {
    failures.add(1, { kind: kind });
    console.warn(
      `failure kind=${kind} status=${res.status} error_code=${res.error_code} error=${res.error}`,
    );
    return;
  }

  const version = readVersion(res);
  if (version === "") {
    failures.add(1, { kind: "body" });
    console.warn(`failure kind=body status=${res.status}`);
    return;
  }

  responses.add(1, {
    version: versions.includes(version) ? version : "unknown",
  });
}
