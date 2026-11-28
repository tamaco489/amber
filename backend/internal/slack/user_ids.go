package slack

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// UserIDs は、GitHub のユーザー名から Slack のユーザー ID を引く対応表
//   - GitHub のユーザー名は大文字と小文字を区別しないため、小文字にそろえて持つ
type UserIDs map[string]string

// Slack のユーザー ID の形 (U か W で始まる英大文字と数字)
var userIDPattern = regexp.MustCompile(`^[UW][A-Z0-9]+$`)

// ParseUserIDs は、対応表の JSON ({"<GitHub のユーザー名>": "<Slack のユーザー ID>"}) を読む
//   - Slack のユーザー ID の形でない値があれば、エラーを返す (エラーには値を入れず、GitHub のユーザー名だけを入れる)
func ParseUserIDs(data string) (UserIDs, error) {
	var raw map[string]string
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, fmt.Errorf("decode slack user ids: %w", err)
	}

	ids := UserIDs{}
	var errs []error
	for login, id := range raw {
		if !userIDPattern.MatchString(id) {
			errs = append(errs, fmt.Errorf("slack user id of %q is not a slack user id", login))
			continue
		}
		ids[strings.ToLower(login)] = id
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return ids, nil
}

// Lookup は、GitHub のユーザー名の Slack のユーザー ID を返す。対応表に無いとき (ユーザー名が空のときを含む) は空を返す
func (u UserIDs) Lookup(login string) string {
	if login == "" {
		return ""
	}
	return u[strings.ToLower(login)]
}
