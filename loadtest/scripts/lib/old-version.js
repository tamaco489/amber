// 古い版 (k6 を流し始めた時点で動いている版) の番号を、setup で 1 回だけ読む
//   - 読めなければ試験を始められないため、exec.test.abort で止める (終了コードは 108)
//   - 閾値の超過 (99) とは別のコードにして、「試験を始められなかった」ことを分ける
// DOC: https://grafana.com/docs/k6/latest/javascript-api/k6-execution/
// DOC: https://grafana.com/docs/k6/latest/using-k6/test-lifecycle/
import exec from "k6/execution";

import { classify } from "./failure.js";
import { getRoot } from "./root.js";
import { readVersion } from "./version.js";

export function readOldVersion() {
  const res = getRoot();

  const kind = classify(res);
  if (kind !== "") {
    exec.test.abort(
      `old version: request failed kind=${kind} status=${res.status} error_code=${res.error_code} error=${res.error}`,
    );
  }

  const version = readVersion(res);
  if (version === "") {
    exec.test.abort(
      `old version: cannot read the version from the body status=${res.status}`,
    );
  }

  console.log(`old version=${version}`);
  return version;
}
