package failmode

import "testing"

func TestFromVersion(t *testing.T) {
	tests := map[string]struct {
		version string
		want    Mode
	}{
		"-fail-exit で終われば Exit": {
			version: "v9-fail-exit",
			want:    Exit,
		},
		"-fail-health で終われば Health": {
			version: "v9-fail-health",
			want:    Health,
		},
		"ふつうの版は Normal": {
			version: "v9",
			want:    Normal,
		},
		"ビルドで版を渡さないとき (dev) は Normal": {
			version: "dev",
			want:    Normal,
		},
		"末尾でなく途中に -fail-exit があれば Normal": {
			version: "v9-fail-exit-2",
			want:    Normal,
		},
		"大文字の -FAIL-EXIT は Normal": {
			version: "v9-FAIL-EXIT",
			want:    Normal,
		},
		"ハイフンが無い fail-exit は Normal": {
			version: "v9fail-exit",
			want:    Normal,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := FromVersion(tt.version); got != tt.want {
				t.Errorf("FromVersion(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}
