package failmode

import "strings"

// Mode は、アプリがどう動くか
type Mode string

// アプリの動き (起動のログの mode に、この値が出る)
const (
	Normal Mode = "normal"      // ふつうに動く
	Exit   Mode = "fail-exit"   // HTTP サーバーを立てずに、終了コード 1 で終わる
	Health Mode = "fail-health" // ふつうに起動するが、/health は 500 を返す
)

// suffixes は、版の名前の末尾と、その版の動き
//   - 末尾どうしは、どちらかがもう一方の末尾にならないようにする (map の順番で結果が変わらないようにするため)
var suffixes = map[string]Mode{
	"-" + string(Exit):   Exit,
	"-" + string(Health): Health,
}

// FromVersion は、版の名前の末尾から動きを決める
//   - 決まった末尾 (-fail-exit、-fail-health) で終わらなければ Normal
func FromVersion(version string) Mode {
	for suffix, mode := range suffixes {
		if strings.HasSuffix(version, suffix) {
			return mode
		}
	}
	return Normal
}
