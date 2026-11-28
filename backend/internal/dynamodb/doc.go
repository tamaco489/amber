// Package dynamodb は、デプロイの通知の、親の投稿と手順の記録を DynamoDB のテーブルに残す
//
// # アイテム
//
// テーブルのパーティションキーは、文字列の pk だけ (ソートキーはない)。expires_at は TTL の属性 (Unix 秒) で、3 日後に消える (デプロイは長くても数時間で終わるため、それより長く残さない)。
//
// 親の投稿 (pk = "deployment#<デプロイ ID>")
//
//	ts                  S  親の投稿の ts
//	channel             S  投稿したチャンネルの ID
//	service             S  サービスの名前
//	status              S  deploying / rolling_back / completed / failed
//	version             S  バージョン (Git のタグ。開始が届くまでは無い)
//	actor               S  起動した人 (同上)
//	run_url             S  GitHub Actions の実行の URL (同上)
//	task_definition     S  タスク定義の ARN (同上)
//	ecs_deployment_url  S  ECS のデプロイのページの URL (ECS のイベントで引けるまでは無い)
//	completed_at        S  完了した時刻 (RFC 3339)
//	rev                 N  書き換えるたびに 1 ずつ増やす (楽観ロック。同時に届いたイベントが、互いの書き換えを消さないようにする)
//	expires_at          N  TTL
//
// 手順の記録 (pk = "step#<手順のキー>")
//
//	lease_until  N     処理中の印 (Unix 秒)。この時刻までは、ほかの呼び出しは同じ手順を進めない
//	done         BOOL  済んだ印。あれば、その手順は飛ばす
//	expires_at   N     TTL
//
// 手順のキーは次のとおり。
//
//	parent#<デプロイ ID>       親の投稿 (デプロイごとに 1 つ。どのイベントが作ったかによらない)
//	<イベント ID>#reply        スレッドへの返信
//	<イベント ID>#parent       親の書き換え
//	<イベント ID>#reaction     スタンプ
package dynamodb
