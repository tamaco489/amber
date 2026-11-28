// 注文の確定の応答 1 件を見て、版とステータスごとに数え、失敗なら種類ごとに数える
//   - 古いクライアント: 200 で本文から版を読めたものだけが成功
//   - 新しいクライアント: 上に加え、古い版が unit_price なしを断った 400 を「想定どおりの失敗」とし、失敗に数えない
//   - 失敗は 1 件ごとに、種類とステータス、版、本文のエラーの文言をログ (level=warning) に出す
import { checkoutResponses } from "./checkout-metrics.js";
import { classify } from "./failure.js";
import { failures } from "./metrics.js";
import { readVersion, versionTag } from "./version.js";

// 古い版が unit_price なしの本文に返すエラーの文言
const missingUnitPrice = "items[0].unit_price is required";

// 本文のエラーの文言を返す。読めなければ空の文字列を返す (読み方は version.js の readVersion と同じ)
// DOC: https://grafana.com/docs/k6/latest/javascript-api/k6-http/response/response-json/
function readError(res) {
  try {
    const v = res.json("error");
    return typeof v === "string" ? v : "";
  } catch {
    return "";
  }
}

// 古い版が unit_price なしを断った 400 か
function isExpectedRejection(res, tag) {
  return (
    res.status === 400 && tag === "old" && readError(res) === missingUnitPrice
  );
}

// 版を読めた応答を、版とステータスごとに数える。版のタグ (old/new) を返し、読めなければ空の文字列を返す
function countVersion(res, oldVersion) {
  const version = readVersion(res);
  if (version === "") {
    return "";
  }
  const tag = versionTag(version, oldVersion);
  checkoutResponses.add(1, { version: tag, status: String(res.status) });
  return tag;
}

// 失敗を種類ごとに数える。失敗でなければ何もしない
//   - 種類の分け方は availability と同じ (lib/failure.js)。想定と違う 400 は client になる
function countFailure(res, tag) {
  const kind = classify(res);
  if (kind !== "") {
    failures.add(1, { kind: kind });
    console.warn(
      `failure kind=${kind} status=${res.status} version=${tag} error_code=${res.error_code} error=${res.error} body_error=${readError(res)}`,
    );
    return;
  }
  if (tag === "") {
    failures.add(1, { kind: "body" });
    console.warn(`failure kind=body status=${res.status}`);
  }
}

// 古いクライアント (unit_price を送る) の応答を数える
export function countOldClient(res, oldVersion) {
  const tag = countVersion(res, oldVersion);
  countFailure(res, tag);
}

// 新しいクライアント (unit_price を送らない) の応答を数える
export function countNewClient(res, oldVersion) {
  const tag = countVersion(res, oldVersion);
  if (isExpectedRejection(res, tag)) {
    return;
  }
  countFailure(res, tag);
}
