// Package slack は、Slack の Web API のうち、デプロイの通知に使うものだけを net/http で呼ぶ
//   - chat.postMessage: 親の投稿と、スレッドへの返信
//   - chat.update: 親の投稿の書き換え
//   - reactions.add: 親の投稿へのスタンプ
//
// HTTP のエラー (429 を含む) と、HTTP 200 で返る ok: false は、どちらもエラーとして返す。
// 呼び出し元 (Lambda) がエラーを返すと、非同期呼び出しの再試行と失敗時の送り先に乗る。
package slack
