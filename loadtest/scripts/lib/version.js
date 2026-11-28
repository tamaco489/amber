// 本文 ({"version":"v1"}) からバージョン番号を読み、古い版か新しい版かに分ける
//   - 古い版は、k6 を流し始めた時点で動いている版 (setup で読む)
//   - 見たいのは切り替えの途中で新旧がどれだけ混ざったかなので、番号そのものはタグにしない

// 数えるときのタグの値の一覧
export const versionTags = ["old", "new"];

// 本文のバージョン番号を返す。読めなければ空の文字列を返す
//   - 本文が JSON でないとき、Response.json() はセレクターなしなら例外を出し、セレクター付きなら undefined を返す
//   - どちらでも空の文字列になるよう、try/catch と型の確かめの両方で受ける
// DOC: https://grafana.com/docs/k6/latest/javascript-api/k6-http/response/response-json/
export function readVersion(res) {
  try {
    const v = res.json("version");
    return typeof v === "string" ? v : "";
  } catch {
    return "";
  }
}

// 古い版と同じ番号なら old、違う番号なら new にする
export function versionTag(version, oldVersion) {
  return version === oldVersion ? "old" : "new";
}
