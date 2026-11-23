// Package integration は、複数のパッケージをまたぐ結合テストを置く
//   - EventBridge の生のイベントを notifier.Handle に渡し、Slack に届く本文 (見出し、色、項目) までを通しで確かめる
//   - 各パッケージの関数やメソッドのテスト (単体テスト) は、それぞれのパッケージに置く
//   - Slack と DynamoDB は差し替える (fake_test.go)
package integration
