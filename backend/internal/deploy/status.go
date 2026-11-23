package deploy

// Status は、親の投稿が表す、デプロイの今の状態
type Status string

// デプロイの状態
const (
	StatusDeploying   Status = "deploying"
	StatusRollingBack Status = "rolling_back"
	StatusCompleted   Status = "completed"
	StatusFailed      Status = "failed"
)

// 状態の進み具合
//   - 親の状態は、進む方向にだけ変える。遅れて届いたイベントで、完了や失敗が「デプロイ中」に戻らないようにする
//   - 失敗は完了より強くする (ロールバックが完了しても、失敗したことを残すため)
var statusRanks = map[Status]int{
	StatusDeploying:   0,
	StatusRollingBack: 1,
	StatusCompleted:   2,
	StatusFailed:      3,
}

func (s Status) rank() int {
	r, ok := statusRanks[s]
	if !ok {
		return -1
	}
	return r
}
