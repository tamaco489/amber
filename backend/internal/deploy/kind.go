package deploy

// Kind は、通知の種類
type Kind int

// 通知の種類 (届くイベントは 4 種類。ECS の進行中は、reason でロールバックかどうかを分ける)
const (
	KindStarted Kind = iota + 1
	KindInProgress
	KindRollback
	KindCompleted
	KindFailed
)

var kindNames = map[Kind]string{
	KindStarted:    "started",
	KindInProgress: "in_progress",
	KindRollback:   "rollback",
	KindCompleted:  "completed",
	KindFailed:     "failed",
}

func (k Kind) String() string {
	name, ok := kindNames[k]
	if !ok {
		return "unknown"
	}
	return name
}
