#!/usr/bin/env bash
# Live2D面接官の「制作用」資産（Cubism Editor のプロジェクトと元絵）を S3 から
# 作業コピーへ展開する。モデルを描き直す人だけが使う。
#
# 実行時資産（Core・.moc3・テクスチャ）は別物で、`fetch-live2d-assets.sh` が
# 別プレフィックスから取る。アプリを動かすだけなら、こちらは要らない。
#
# なぜ別プレフィックスなのか:
#   `fetch-live2d-assets.sh` は s3://$BUCKET/live2d/ を frontend/public/live2d/ へ
#   丸ごと sync する。public/ はブラウザへ配信されるので、live2d/ の下に制作用の
#   元絵（25MB）を置くと、全ビルドのイメージに入って公開されてしまう。
#   そのため制作用は live2d-authoring/ に分けてある。
#
# 使い方: ./automation/ops/fetch-live2d-authoring.sh
#   リポジトリのルートで実行する。S3 のキーはリポジトリ相対だが、ルート全体へ
#   sync すると想定外のキーで追跡ファイルを上書きしうるので、既知の3ディレクトリ
#   だけを同期する。
#   バケットは LIVE2D_ASSETS_BUCKET（未指定ならエラー）
set -euo pipefail

BUCKET="${LIVE2D_ASSETS_BUCKET:-}"
if [ -z "$BUCKET" ]; then
  echo "エラー: LIVE2D_ASSETS_BUCKET が未設定です" >&2
  echo "  例: LIVE2D_ASSETS_BUCKET=soc-build-assets-508897596159 $0" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# 既知の制作用ディレクトリだけを同期する。ルート全体 sync だと、
# live2d-authoring/Makefile などが入っていたときに作業コピーを上書きする。
# --delete は付けない。ローカルだけの作業ファイルまで消さないため。
pairs=(
  "docs/avatar/live2d-interviewers/cubism-wip"
  "frontend/public/live2d/interviewers/source"
  "frontend/public/live2d/interviewers/cubism-input"
)
for rel in "${pairs[@]}"; do
  mkdir -p "$rel"
  aws s3 sync "s3://$BUCKET/live2d-authoring/$rel/" "$rel/" --only-show-errors
done

required=(
  "docs/avatar/live2d-interviewers/cubism-wip/male-interviewer.cmo3"
  "docs/avatar/live2d-interviewers/cubism-wip/female-interviewer.cmo3"
  "frontend/public/live2d/interviewers/source/male/male-neutral.png"
  "frontend/public/live2d/interviewers/source/female/female-neutral.png"
  "frontend/public/live2d/interviewers/cubism-input/male/male-neutral-2048.png"
  "frontend/public/live2d/interviewers/cubism-input/female/female-neutral-2048.png"
)
missing=0
for f in "${required[@]}"; do
  if [ ! -s "$f" ]; then
    echo "エラー: $f が無い（または空）" >&2
    missing=1
  fi
done
if [ "$missing" -ne 0 ]; then
  echo "エラー: s3://$BUCKET/live2d-authoring/ の内容が不足しています" >&2
  exit 1
fi

echo "Live2Dの制作用資産を展開しました:"
echo "  Cubismプロジェクト: docs/avatar/live2d-interviewers/cubism-wip/"
echo "  元絵:               frontend/public/live2d/interviewers/source/"
echo "  取り込み用2048px:   frontend/public/live2d/interviewers/cubism-input/"
echo
echo "書き出したモデルを配る手順は docs/wiki/live2d-assets.md を参照。"
