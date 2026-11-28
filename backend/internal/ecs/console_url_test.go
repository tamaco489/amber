package ecs

import "testing"

func TestConsoleURL(t *testing.T) {
	tests := map[string]struct {
		svc  *serviceARN
		id   string
		want string
	}{
		"東京のサービスデプロイ": {
			svc:  &serviceARN{region: "ap-northeast-1", cluster: "prd-amber", service: "prd-amber-api"},
			id:   "NCWGC2ZR-taawPAYrIaU5",
			want: "https://ap-northeast-1.console.aws.amazon.com/ecs/v2/clusters/prd-amber/services/prd-amber-api/deployments/NCWGC2ZR-taawPAYrIaU5?region=ap-northeast-1",
		},
		"ほかのリージョン": {
			svc:  &serviceARN{region: "us-west-2", cluster: "example", service: "sd-example"},
			id:   "abc",
			want: "https://us-west-2.console.aws.amazon.com/ecs/v2/clusters/example/services/sd-example/deployments/abc?region=us-west-2",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := consoleURL(tt.svc, tt.id); got != tt.want {
				t.Errorf("url = %q, want %q", got, tt.want)
			}
		})
	}
}
