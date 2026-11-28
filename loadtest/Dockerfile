# k6 のイメージの版を決めるファイル。ビルドはせず、loadtest/justfile が FROM の行からイメージを読む
#   - タグとダイジェスト (マルチアーキテクチャのインデックス) で固定する
#   - Dependabot (docker) は justfile を読まないため、Dependabot が読める Dockerfile に置く
#   - FROM の行は 1 行だけにする (justfile は最初の FROM の行を読む)
# DOC: https://hub.docker.com/r/grafana/k6
FROM grafana/k6:2.4.0@sha256:d41bb79cd8de5a7ad619b53570e3712633e8bff0937010a5c6c57a4747e7a276
