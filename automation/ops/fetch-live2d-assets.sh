#!/usr/bin/env bash
# Live2D面接官の実行時資産（Cubism Core と書き出し済みモデル）を S3 から
# ビルドコンテキストへ展開する。
#
# なぜリポジトリに置かずS3から取るのか:
#   Cubism Core は Live2D Proprietary Software License の "Redistributable Code" で、
#   アプリに同梱して配る分は契約の想定内だが、SDK 単体としての再配布にあたる形
#   （public リポジトリへのコミット）はできない。このリポジトリは public なので、
#   非公開バケットを経由させる。モデル一式も配布条件がリポジトリと別。
#   `.gitignore` の `frontend/public/live2d/*` と `docs/wiki/live2d-assets.md` 参照。
#
# 使い方: ./automation/ops/fetch-live2d-assets.sh [宛先ディレクトリ]
#   既定の宛先は frontend/public/live2d
#   バケットは LIVE2D_ASSETS_BUCKET（未指定ならエラー）
#
# 置いたつもりで置けていない状態でビルドが通ると、NEXT_PUBLIC_LIVE2D_ENABLED=true の
# ままモデルが無いイメージが出来上がり、面接のたびに404を取ってからAvatar2Dへ落ちる。
# 黙って進めず、揃っていなければ非ゼロで落ちる。
set -euo pipefail

DEST="${1:-frontend/public/live2d}"
BUCKET="${LIVE2D_ASSETS_BUCKET:-}"

if [ -z "$BUCKET" ]; then
  echo "エラー: LIVE2D_ASSETS_BUCKET が未設定です" >&2
  exit 1
fi

mkdir -p "$DEST"
# --delete は付けない。宛先には Git 追跡下のシェーダ・素材が既にあり、消すと動かなくなる。
aws s3 sync "s3://$BUCKET/live2d/" "$DEST/" --only-show-errors

required=(
  "core/live2dcubismcore.min.js"
  "interviewers/male/male-interviewer.model3.json"
  "interviewers/male/male-interviewer.moc3"
  "interviewers/female/female-interviewer.model3.json"
  "interviewers/female/female-interviewer.moc3"
)
missing=0
for f in "${required[@]}"; do
  if [ ! -s "$DEST/$f" ]; then
    echo "エラー: $DEST/$f が無い（または空）" >&2
    missing=1
  fi
done
if [ "$missing" -ne 0 ]; then
  echo "エラー: s3://$BUCKET/live2d/ の内容が不足しています。Live2Dを有効にしたままでは出せません" >&2
  exit 1
fi

# model3.json が参照するテクスチャまで揃っているかを見る。モデルだけ来てテクスチャが
# 無いと、読み込みは通るのに真っ白なcanvasになる。
for g in male female; do
  m="$DEST/interviewers/$g/$g-interviewer.model3.json"
  python3 - "$m" "$DEST/interviewers/$g" <<'PY'
import json, os, sys
model, base = sys.argv[1], sys.argv[2]
with open(model, encoding='utf-8') as fh:
    refs = json.load(fh)['FileReferences']
targets = list(refs.get('Textures') or [])
for key in ('Moc', 'DisplayInfo'):
    if refs.get(key):
        targets.append(refs[key])
# Moc が無いとランタイムはモデルを読み込めない。Textures が空だと、テクスチャを
# 一枚も設定しないまま描画して真っ白になる。どちらも「参照が無い」ので、
# 下の存在チェックだけでは素通りしてしまう。
if not refs.get('Moc'):
    sys.exit(f"エラー: {model} に Moc の参照が無い")
if not refs.get('Textures'):
    sys.exit(f"エラー: {model} に Textures の参照が無い")
for rel in targets:
    path = os.path.join(base, rel)
    if not os.path.isfile(path) or os.path.getsize(path) == 0:
        sys.exit(f"エラー: {model} が参照する {rel} が無い（または空）")
PY
done

echo "Live2D資産を配置しました:"
find "$DEST/core" "$DEST/interviewers/male" "$DEST/interviewers/female" -type f \
  | sort | sed "s|^$DEST/|  |"
