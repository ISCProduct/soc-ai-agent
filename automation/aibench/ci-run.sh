#!/usr/bin/env bash
# aibench を CI で実行し、base と head の差分をジョブサマリーに出す（#1782）。
#
# 環境変数:
#   OPENAI_API_KEY         未設定なら実行せず、案内だけ出して成功で終わる（フォークの PR など）
#   AIBENCH_TARGETS        カンマ区切りの評価対象（既定: ES 以外の5つ。ES は RAG の起動が要るので CI では測らない）
#   AIBENCH_RUNS           同一入力の実行回数（既定 2）
#   AIBENCH_MAX_COST_USD   1回の実行の上限（USD）。base と head の概算の合計がこれを超えたら API を呼ばずに失敗する
#   AIBENCH_BASE_REF       比べる base（例: origin/develop）。空なら head だけ測る
#   AIBENCH_OUT_DIR        結果の置き場所（既定 /tmp/aibench。リポジトリの外）
#   AIBENCH_DRY_RUN        1 なら API を呼ばずに流れだけ確かめる（手元の確認用）
#
# 終了コード: 0 正常 / 1 劣化を検知・上限超過・実行の失敗
set -euo pipefail

TARGETS="${AIBENCH_TARGETS:-chat-answer,company-extract,matching-reason,resume,interview-report}"
RUNS="${AIBENCH_RUNS:-2}"
MAX_COST="${AIBENCH_MAX_COST_USD:-1.00}"
BASE_REF="${AIBENCH_BASE_REF:-}"
OUT="${AIBENCH_OUT_DIR:-/tmp/aibench}"
DRY="${AIBENCH_DRY_RUN:-}"
SUMMARY="${GITHUB_STEP_SUMMARY:-/dev/stdout}"
ROOT="$(git rev-parse --show-toplevel)"

summary() { printf '%s\n' "$@" >>"$SUMMARY"; }

summary "## AI Bench（aibench）"
if [ -z "${OPENAI_API_KEY:-}" ] && [ -z "$DRY" ]; then
  # 成功で終わるので、「測って問題なし」と見分けられるよう警告の注釈も出す
  echo "::warning title=AI Bench は未計測です::AIBENCH_OPENAI_API_KEY が未設定のため測っていません（成功扱いですが、品質は確認されていません）"
  summary "" "**未計測**: OPENAI_API_KEY（\`secrets.AIBENCH_OPENAI_API_KEY\`）が未設定のため、実行しませんでした。" \
    "フォークからの PR ではシークレットが渡らないので、メンテナーが手動実行（workflow_dispatch）で測ってください。"
  exit 0
fi

mkdir -p "$OUT"
IFS=',' read -r -a targets <<<"$TARGETS"

build() { (cd "$1/Backend" && go build -o "$2" ./cmd/aibench); }
build "$ROOT" "$OUT/aibench-head"
sides=(head)
declare -A src=([head]="$ROOT")
if [ -n "$BASE_REF" ]; then
  rm -rf "$OUT/base-src"
  git -C "$ROOT" worktree add --detach --force "$OUT/base-src" "$BASE_REF" >/dev/null
  trap 'git -C "$ROOT" worktree remove --force "$OUT/base-src" >/dev/null 2>&1 || true' EXIT
  build "$OUT/base-src" "$OUT/aibench-base"
  src[base]="$OUT/base-src"
  sides=(base head)
fi

dry_flag=()
[ -n "$DRY" ] && dry_flag=(-dry-run)

# 1) 概算コストの合計を先に出し、上限を超えるなら何も呼ばずに止める
total=0
declare -A skip=()
for side in "${sides[@]}"; do
  for t in "${targets[@]}"; do
    manifest="${src[$side]}/docs/research/ai-eval/$t.jsonl"
    if ! est_out="$("$OUT/aibench-$side" -target "$t" -manifest "$manifest" -n "$RUNS" -dry-run 2>&1)"; then
      if [ "$side" = base ]; then
        # base にまだ無い評価対象（固定セットや target が無い）は head だけ測る
        skip[$side-$t]=1
        continue
      fi
      # head の失敗は評価コードや固定セットの破損なので、飛ばさずに止める
      summary "" "head / $t の事前検証（-dry-run）に失敗しました。評価コードか固定セットが壊れています:" '````' "$(printf '%s\n' "$est_out" | tail -20)" '````'
      exit 1
    fi
    est="$(printf '%s\n' "$est_out" | sed -n 's/.*概算コスト \$\([0-9.]*\).*/\1/p' | head -1)"
    total="$(awk -v a="$total" -v b="${est:-0}" 'BEGIN { printf "%.4f", a + b }')"
  done
done
summary "" "- 評価対象: \`$TARGETS\` / 実行回数: $RUNS / 比較: ${BASE_REF:-なし（head のみ）}" \
  "- 概算コストの合計（上限見積もり。実測はおよそ3分の1）: \$$total / 上限 \$$MAX_COST"
if awk -v a="$total" -v b="$MAX_COST" 'BEGIN { exit !(a > b) }'; then
  summary "" "概算コストが上限を超えるため実行しませんでした。評価対象を絞るか、上限を上げて手動実行してください。"
  exit 1
fi

# 2) 実行
failed=0
for side in "${sides[@]}"; do
  for t in "${targets[@]}"; do
    [ -n "${skip[$side-$t]:-}" ] && continue
    manifest="${src[$side]}/docs/research/ai-eval/$t.jsonl"
    # 費用の上限は上の概算の合計で確かめ済み。-max-cost-usd は base の版に無いことがあるので
    # head だけに渡す（1つの評価対象が単独で上限を超えないかの二重の歯止め）。
    cost_flag=()
    [ "$side" = head ] && cost_flag=(-max-cost-usd "$MAX_COST")
    if ! "$OUT/aibench-$side" -target "$t" -manifest "$manifest" -n "$RUNS" -yes "${dry_flag[@]}" \
      "${cost_flag[@]}" -out "$OUT/$side-$t.json" >"$OUT/$side-$t.log" 2>&1; then
      summary "" "- ❌ $side / $t の実行に失敗しました（ログはアーティファクトの \`$side-$t.log\`）"
      failed=1
    elif [ -z "$DRY" ] && grep -qE '"measured_runs": *0[,}]' "$OUT/$side-$t.json"; then
      # 全呼び出しが通信エラー・レート制限などで測れなかった。ゼロ同士の比較は「劣化なし」に見えるので失敗にする
      summary "" "- ❌ $side / $t は1件も計測できませんでした（API の障害・レート制限・残高を確認）"
      failed=1
    fi
  done
done

# 3) 結果と差分
regressed=0
for t in "${targets[@]}"; do
  summary "" "### $t"
  if [ -n "${skip[head-$t]:-}" ]; then
    summary "head に固定セットが無いため測っていません。"
    continue
  fi
  if [ -n "$DRY" ]; then
    summary "（dry-run）"
    continue
  fi
  summary "<details><summary>head の結果</summary>" "" '````' "$(sed -n '\# / model=#,$p' "$OUT/head-$t.log" | head -80)" '````' "</details>"
  if [ "${#sides[@]}" -eq 2 ] && [ -f "$OUT/base-$t.json" ] && [ -f "$OUT/head-$t.json" ]; then
    base_manifest="${src[base]}/docs/research/ai-eval/$t.jsonl"
    head_manifest="$ROOT/docs/research/ai-eval/$t.jsonl"
    if "$OUT/aibench-head" -diff "$OUT/base-$t.json,$OUT/head-$t.json" >"$OUT/diff-$t.txt" 2>&1; then
      summary "base との差分: 劣化なし"
    elif ! cmp -s "$base_manifest" "$head_manifest"; then
      # 固定セットが変わると入力が違うので、差はモデルやプロンプトの劣化とは言えない。参考として出すだけにする。
      # ただし、プロンプトの変更と同じ PR だと劣化を見逃すので、注釈とサマリーで目立たせる
      echo "::warning title=AI Bench: $t は劣化の判定をしていません::固定セットが変わったため参考値です。プロンプトの変更と同じ PR なら、レビューで差分を確認してください"
      summary "base との差分: **固定セットが変わったため参考値です（劣化の判定に使いません）**" \
        "> ⚠️ 差分は劣化を示していますが、ジョブは失敗させていません。プロンプトやモデルの変更と同じ PR では、この差分をレビューで確認してください。固定セットの変更は別の PR に分けると判定が効きます。"
    else
      summary "base との差分: **劣化を検知しました**"
      regressed=1
    fi
    summary '````' "$(head -60 "$OUT/diff-$t.txt")" '````'
  elif [ "${#sides[@]}" -eq 2 ]; then
    summary "base にこの評価対象が無いため、差分はありません（head のみ）。"
  fi
done

if [ "$failed" -ne 0 ] || [ "$regressed" -ne 0 ]; then
  exit 1
fi
