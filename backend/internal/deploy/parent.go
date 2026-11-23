package deploy

import "time"

// Parent は、親の投稿と、それを組み立て直すのに要る値
//   - chat.update は本文をまるごと置き換えるため、書き換えるたびに、ここから組み立て直す
type Parent struct {
	DeploymentID   string    // デプロイ ID
	TS             string    // 親の投稿の ts
	Channel        string    // 投稿したチャンネルの ID
	Service        string    // サービスの名前
	Status         Status    // デプロイの今の状態
	Version        string    // バージョン (開始が届くまでは空)
	Actor          string    // 起動した人 (同上)
	RunURL         string    // GitHub Actions の実行の URL (同上)
	TaskDefinition string    // タスク定義の ARN (同上)
	CompletedAt    time.Time // 完了した時刻
	Rev            int       // 書き換えるたびに 1 ずつ増やす (楽観ロック)
}

// Apply は、イベントを親の状態に反映する。変わったかどうかを返す
//   - 開始: バージョン、起動した人、リンク、タスク定義を埋める
//   - ロールバック・完了・失敗: 状態を進める (戻す方向には変えない)
//   - ロールバックでない進行中: 変えない
func (p *Parent) Apply(ev *Event) bool {
	changed := setString(&p.Service, ev.Service)

	switch ev.Kind {
	case KindStarted:
		changed = p.applyStarted(ev) || changed
	case KindRollback:
		changed = p.advance(StatusRollingBack) || changed
	case KindCompleted:
		changed = p.applyCompleted(ev) || changed
	case KindFailed:
		changed = p.advance(StatusFailed) || changed
	case KindInProgress:
	}
	return changed
}

// applyStarted は、開始のイベントの値を埋める
func (p *Parent) applyStarted(ev *Event) bool {
	changed := setString(&p.Version, ev.Version)
	changed = setString(&p.Actor, ev.Actor) || changed
	changed = setString(&p.RunURL, ev.RunURL) || changed
	changed = setString(&p.TaskDefinition, ev.TaskDefinition) || changed
	return changed
}

// applyCompleted は、完了に進め、完了した時刻を残す
func (p *Parent) applyCompleted(ev *Event) bool {
	if !p.advance(StatusCompleted) {
		return false
	}
	p.CompletedAt = ev.Time.UTC().Truncate(time.Second)
	return true
}

// setString は、空でない値で書き換える。変わったかどうかを返す
func setString(dst *string, v string) bool {
	if v == "" || *dst == v {
		return false
	}
	*dst = v
	return true
}

// advance は、状態を進める。戻す方向なら変えない
func (p *Parent) advance(s Status) bool {
	if s.rank() <= p.Status.rank() {
		return false
	}
	p.Status = s
	return true
}
