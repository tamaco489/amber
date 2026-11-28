package ecs

import "testing"

func TestParseServiceARN(t *testing.T) {
	got, err := parseServiceARN(testServiceARN)
	if err != nil {
		t.Fatalf("err = %v", err)
	}

	want := serviceARN{arn: testServiceARN, region: "ap-northeast-1", cluster: "prd-amber", service: "prd-amber-api"}
	if *got != want {
		t.Errorf("arn = %+v, want %+v", *got, want)
	}
}

func TestParseServiceARNError(t *testing.T) {
	tests := map[string]string{
		"空":        "",
		"ECS ではない": "arn:aws:lambda:ap-northeast-1:111122223333:function:x",
		"クラスターを含まない古い形式":       "arn:aws:ecs:ap-northeast-1:111122223333:service/prd-amber-api",
		"サービスではない":             "arn:aws:ecs:ap-northeast-1:111122223333:task/prd-amber/abc",
		"リージョンが無い":             "arn:aws:ecs::111122223333:service/prd-amber/prd-amber-api",
		"サービスの名前が無い":           "arn:aws:ecs:ap-northeast-1:111122223333:service/prd-amber/",
		"サービスデプロイの ARN (形が違う)": "arn:aws:ecs:ap-northeast-1:111122223333:service-deployment/prd-amber/prd-amber-api/abc",
	}

	for name, arn := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := parseServiceARN(arn)
			if err == nil {
				t.Errorf("arn = %+v, want an error", got)
			}
			if got != nil {
				t.Errorf("arn = %+v, want nil", got)
			}
		})
	}
}
