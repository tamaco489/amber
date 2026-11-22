# 各ディレクトリの justfile をモジュールとして読み込む (例: just infra::plan prd)
# まだ無いディレクトリがあっても動くよう、mod? で読み込む
mod? actions ".github/justfile"
mod? backend
mod? infra
mod? loadtest

default:
    @just --list --list-submodules
