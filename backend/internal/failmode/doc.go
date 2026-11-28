// Package failmode は、版の名前の末尾から、検証用のアプリ (cmd/api) がどう動くか (わざと失敗するか) を決める
//   - -fail-exit で終わる版: HTTP サーバーを立てずに、すぐ終了する (起動の途中で落ちるアプリの真似)
//   - -fail-health で終わる版: ふつうに起動するが、/health だけが 500 を返す (ヘルスチェックに通らないアプリの真似)
//   - それ以外の版: ふつうに動く
//
// ECS のデプロイのサーキットブレーカーが、失敗と判定して元の版に戻すかを確かめるために使う。
// 版の名前はタグ名 (cd-backend が VERSION としてビルドに渡す) のため、Terraform も CD も変えずに失敗する版を作れる。
// 決めるだけで、終了や /health の応答は cmd/api と internal/health が持つ。
package failmode
