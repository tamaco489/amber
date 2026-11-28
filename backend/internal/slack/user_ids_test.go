package slack_test

import (
	"testing"

	"github.com/tamaco489/amber/backend/internal/slack"
)

func TestParseUserIDs(t *testing.T) {
	tests := map[string]struct {
		data    string
		want    slack.UserIDs
		wantErr bool
	}{
		"GitHub のユーザー名を小文字にそろえて読む": {
			data: `{"Octocat": "U0123ABCD", "hubot": "W0456EFGH"}`,
			want: slack.UserIDs{"octocat": "U0123ABCD", "hubot": "W0456EFGH"},
		},
		"空の対応表を読む": {
			data: `{}`,
			want: slack.UserIDs{},
		},
		"JSON でなければエラーを返す": {
			data:    `octocat=U0123ABCD`,
			wantErr: true,
		},
		"Slack のユーザー ID の形でない値があればエラーを返す": {
			data:    `{"octocat": "@octocat"}`,
			wantErr: true,
		},
		"チャンネルの ID はエラーを返す": {
			data:    `{"octocat": "C0123ABCD"}`,
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := slack.ParseUserIDs(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if got != nil {
					t.Errorf("got = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got = %v, want %v", got, tt.want)
			}
			for login, id := range tt.want {
				if got[login] != id {
					t.Errorf("%s = %q, want %q", login, got[login], id)
				}
			}
		})
	}
}

func TestUserIDsLookup(t *testing.T) {
	ids := slack.UserIDs{"octocat": "U0123ABCD"}

	tests := map[string]struct {
		ids   slack.UserIDs
		login string
		want  string
	}{
		"対応表にあるユーザーの ID を返す": {
			ids:   ids,
			login: "octocat",
			want:  "U0123ABCD",
		},
		"大文字と小文字が違っても引ける": {
			ids:   ids,
			login: "OctoCat",
			want:  "U0123ABCD",
		},
		"対応表に無いユーザーは空を返す": {
			ids:   ids,
			login: "hubot",
		},
		"起動した人が分からないときは空を返す": {
			ids: ids,
		},
		"対応表が無ければ空を返す": {
			login: "octocat",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.ids.Lookup(tt.login); got != tt.want {
				t.Errorf("Lookup(%q) = %q, want %q", tt.login, got, tt.want)
			}
		})
	}
}
