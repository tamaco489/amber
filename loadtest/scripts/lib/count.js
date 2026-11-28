// 応答 1 件を見て、失敗なら種類ごとに、成功なら古い版か新しい版かで数える
//   - 失敗は 1 件ごとに、種類とステータス、エラーコードをログ (level=warning) に出す
import { classify } from "./failure.js";
import { failures, responses } from "./metrics.js";
import { readVersion, versionTag } from "./version.js";

export function countResponse(res, oldVersion) {
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

  responses.add(1, { version: versionTag(version, oldVersion) });
}
