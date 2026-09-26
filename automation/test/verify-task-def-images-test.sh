#!/bin/bash
# automation/ops/verify-task-def-images.sh の振る舞いを固定する。
#
# このスクリプトは「ECSのサーキットブレーカーがECRから消えたイメージへ戻していないか」を
# 見る最後の砦（#1518）。素通りすると、停止日の本番が起動不能なタスク定義を指したまま
# 次の稼働日を迎える。誤検知すると、正常なデプロイで本番のタスク定義を勝手に書き換える。
# どちらも本番で気づくと手遅れなので、aws をスタブして実際に実行して確かめる。
#
# 使い方: ./automation/test/verify-task-def-images-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TARGET="$ROOT/automation/ops/verify-task-def-images.sh"

fail=0
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

REGISTRY="508897596159.dkr.ecr.ap-northeast-1.amazonaws.com"

# aws スタブ。
#   - describe-services は TD_<サービス> / DC_<サービス> / RC_<サービス> で
#     「今指しているタスク定義 / desiredCount / runningCount」を返す（既定 0 / 0）
#   - describe-task-definition は `soc-app-<svc>:<rev>` から `<registry>/soc-<svc>:img<rev>` を作る
#     （NON_ECR に入れたサービスは Docker Hub のイメージを返す）
#   - describe-images は MISSING_TAGS / ERROR_TAGS に応じて失敗する
#   - update-service は呼ばれたことを CALLS へ記録する
mkdir -p "$WORK/bin"
cat > "$WORK/bin/aws" <<'STUB'
#!/bin/bash
arg_after() {
  local want="$1"; shift
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "$want" ]; then echo "${2:-}"; return 0; fi
    shift
  done
  return 1
}

case "$1 $2" in
  "ecs describe-services")
    svc=$(arg_after --services "$@")
    key="${svc//-/_}"
    td="TD_$key"; dc="DC_$key"; rc="RC_$key"
    # 本体は `--query 'services[0].[taskDefinition,desiredCount,runningCount]'` で
    # タブ区切り1行を受け取る。ここも同じ形で返す。
    printf '%s\t%s\t%s\n' \
      "${!td:-arn:aws:ecs:ap-northeast-1:1:task-definition/soc-app-${svc}:88}" \
      "${!dc:-0}" "${!rc:-0}"
    ;;
  "ecs describe-task-definition")
    td=$(arg_after --task-definition "$@")
    name="${td##*/}"          # soc-app-backend:88
    rev="${name##*:}"
    svc="${name%:*}"; svc="${svc#soc-app-}"
    case " ${NON_ECR:-} " in
      *" $svc "*) echo "chromadb/chroma:0.6.3"; exit 0 ;;
    esac
    case " ${BROKEN_TD:-} " in
      *" $name "*) echo "AccessDeniedException: describe-task-definition" >&2; exit 254 ;;
    esac
    echo "${REGISTRY}/soc-${svc}:img${rev}"
    ;;
  "ecr describe-images")
    id=$(arg_after --image-ids "$@")
    tag="${id#imageTag=}"
    case " ${ERROR_TAGS:-} " in
      *" $tag "*) echo "ThrottlingException: Rate exceeded" >&2; exit 254 ;;
    esac
    case " ${MISSING_TAGS:-} " in
      *" $tag "*) echo "An error occurred (ImageNotFoundException) when calling the DescribeImages operation" >&2; exit 254 ;;
    esac
    echo '{"imageDetails":[{}]}'
    ;;
  "ecs update-service")
    echo "update-service $(arg_after --service "$@") $(arg_after --task-definition "$@")" >> "$CALLS"
    ;;
  *)
    echo "予期しない aws 呼び出し: $*" >&2
    exit 99
    ;;
esac
STUB
chmod +x "$WORK/bin/aws"

# run は1ケースを実行し、終了コードと update-service の呼び出しを検証する。
# $1 ケース名 / $2 期待exit / $3 期待するupdate-service呼び出し(空なら呼ばれないこと) / 残り: 引数
run() {
  local name="$1" want_exit="$2" want_call="$3"; shift 3
  local out actual calls
  CALLS="$WORK/calls"
  : > "$CALLS"
  out=$(PATH="$WORK/bin:$PATH" PROJECT_NAME=soc-app REGISTRY="$REGISTRY" CALLS="$CALLS" \
    TD_backend="${TD_backend:-}" TD_frontend="${TD_frontend:-}" \
    DC_backend="${DC_backend:-}" RC_backend="${RC_backend:-}" \
    MISSING_TAGS="${MISSING_TAGS:-}" ERROR_TAGS="${ERROR_TAGS:-}" \
    NON_ECR="${NON_ECR:-}" BROKEN_TD="${BROKEN_TD:-}" \
    bash "$TARGET" "$@" 2>&1)
  actual=$?
  # `VAR=x run ...` の前置代入は、関数呼び出しでは呼び出し後も残る（bashの仕様）。
  # 消さないと次のケースへ設定が漏れ、「別の条件を確かめたつもり」になる。
  unset TD_backend TD_frontend DC_backend RC_backend MISSING_TAGS ERROR_TAGS NON_ECR BROKEN_TD
  calls=$(tr -d '\n' < "$CALLS")
  if [ "$actual" -ne "$want_exit" ]; then
    echo "FAIL $name: 終了コード 期待=$want_exit 実際=$actual"
    printf '%s\n' "$out" | sed 's/^/     /'
    fail=$((fail + 1))
    return
  fi
  if [ "$calls" != "$want_call" ]; then
    echo "FAIL $name: update-service 期待='$want_call' 実際='$calls'"
    printf '%s\n' "$out" | sed 's/^/     /'
    fail=$((fail + 1))
    return
  fi
  echo "ok   $name"
  LAST_OUT="$out"
}

ARN88="arn:aws:ecs:ap-northeast-1:1:task-definition/soc-app-backend:88"
ARN86="arn:aws:ecs:ap-northeast-1:1:task-definition/soc-app-backend:86"

# 正常系: 参照先のイメージが在るならタスク定義に触らない。
TD_backend="$ARN88" MISSING_TAGS="" \
  run "健全なら何もしない" 0 "" "backend=$ARN88"

# 本番で起きた形: サーキットブレーカーが :86 へ戻し、そのイメージは失効済み。
TD_backend="$ARN86" MISSING_TAGS="img86" \
  run "失効したリビジョンを指していたら今回のリビジョンへ戻す" 1 "update-service backend $ARN88" \
  "backend=$ARN88"
case "${LAST_OUT:-}" in
  *"::error::"*) : ;;
  *) echo "FAIL 戻したことが ::error:: として出ていない"; fail=$((fail + 1)) ;;
esac

# 戻し先が分からない場合（そのサービスを今回デプロイしていない）は失敗させるだけ。
# 勝手に別のリビジョンを選ぶと、動いている本番を壊しうる。
TD_backend="$ARN86" MISSING_TAGS="img86" \
  run "戻し先が無ければ書き換えずに失敗" 1 "" "backend="

# 戻し先のイメージも無いなら書き換えない（壊れた定義から壊れた定義へ移すだけ）。
TD_backend="$ARN86" MISSING_TAGS="img86 img88" \
  run "戻し先も失効していれば書き換えない" 1 "" "backend=$ARN88"

# 判定不能（スロットリング・権限）を「無い」に倒すと、API障害でタスク定義を書き換える。
TD_backend="$ARN88" ERROR_TAGS="img88" \
  run "ECRを引けない時は書き換えず失敗" 1 "" "backend=$ARN88"

# 上のケースは「戻し先＝今の定義」なのでガードが二重に効く。戻し先が別リビジョンでも
# 判定不能なら書き換えないことを、ここで単独に固定する。
TD_backend="$ARN86" ERROR_TAGS="img86" \
  run "判定不能を『無い』に倒して書き換えない" 1 "" "backend=$ARN88"

# タスク定義自体が引けない場合も同じ扱い。
TD_backend="$ARN88" BROKEN_TD="soc-app-backend:88" \
  run "タスク定義を引けない時は書き換えず失敗" 1 "" "backend=$ARN88"

# chroma は Docker Hub のイメージ。ECRに無いのは正常なので検査対象外。
NON_ECR="chroma" run "ECR以外のイメージは対象外" 0 "" "chroma="

# 1サービスが壊れていても残りのサービスを検査する（途中で return すると取りこぼす）。
TD_backend="$ARN86" MISSING_TAGS="img86" \
  run "壊れたサービスがあっても後続を検査する" 1 "update-service backend $ARN88" \
  "backend=$ARN88" "frontend="
case "${LAST_OUT:-}" in
  *"ok   frontend"*) : ;;
  *) echo "FAIL 後続サービス(frontend)を検査していない"; fail=$((fail + 1)) ;;
esac

# 稼働中のサービスは戻さない（Codex #1534 指摘）。
#
# 稼働日のデプロイで新リビジョンがヘルスチェックに落ち、サーキットブレーカーが
# 「タグは失効済みだが既存タスクは動いている」旧リビジョンへ戻した状況。
# ここで $wanted（直前に落ちたリビジョン）へ update-service すると、
# minimumHealthyPercent=0 のため健全な稼働タスクが先に落とされて本番が止まる。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend=1 RC_backend=1 \
  run "稼働中(desired=1 running=1)なら戻さず失敗" 1 "" "backend=$ARN88"
case "${LAST_OUT:-}" in
  *"稼働中"*) : ;;
  *) echo "FAIL 稼働中で見送ったことがログに出ていない"; fail=$((fail + 1)) ;;
esac

# 起動途中（desired=1, running=0）も「これから running になる」ので触らない。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend=1 RC_backend=0 \
  run "起動要求あり(desired=1 running=0)なら戻さず失敗" 1 "" "backend=$ARN88"

# ドレイン中（desired=0, running=1）も、残っているタスクを失う可能性がある。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend=0 RC_backend=1 \
  run "ドレイン中(desired=0 running=1)なら戻さず失敗" 1 "" "backend=$ARN88"

# desired/running が取れない（空・None）場合も「稼働中かもしれない」側へ倒す。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend="None" RC_backend="None" \
  run "desired/running が不明なら戻さず失敗" 1 "" "backend=$ARN88"

# サービス名を1つも渡さない使い方は事故（全サービス素通りで緑になる）。
run "引数無しは使用方法エラー" 2 ""

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS"
