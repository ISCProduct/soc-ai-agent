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
SCALE="$ROOT/automation/ops/prod-scale.sh"

fail=0
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

REGISTRY="508897596159.dkr.ecr.ap-northeast-1.amazonaws.com"

# aws スタブ。
#   - describe-services は TD_<サービス> / DC_<サービス> / RC_<サービス> / PC_<サービス> で
#     「今指しているタスク定義 / desiredCount / runningCount / pendingCount」を返す（既定 0）。
#     $STATE/DC_<サービス> があればそちらを優先する（prod-scale.sh を実行して
#     desired が変わる、停止日デプロイの順序をそのまま再現するため）
#     RC_SEQ を与えると runningCount は呼び出しごとに RC_SEQ の値を順に返す
#     （`update-service --desired-count 0` の後もしばらく running=1 が続く、
#     実運用の非同期なドレインを再現する。末尾に達したら最後の値を返し続ける）
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
    td="TD_$key"; dc="DC_$key"; rc="RC_$key"; pc="PC_$key"
    desired="${!dc:-0}"
    [ -f "${STATE:-/nonexistent}/DC_$svc" ] && desired=$(cat "$STATE/DC_$svc")
    running="${!rc:-0}"
    if [ -n "${RC_SEQ:-}" ]; then
      # 呼び出し回数を数えて RC_SEQ の n 番目を返す（ドレインの再現）。
      mkdir -p "$STATE"
      n=$(($(cat "$STATE/rc_calls_$svc" 2>/dev/null || echo 0) + 1))
      echo "$n" > "$STATE/rc_calls_$svc"
      running=$(echo "$RC_SEQ" | awk -v i="$n" '{print (i <= NF ? $i : $NF)}')
    fi
    # 本体は `--query 'services[0].[taskDefinition,desiredCount,runningCount,pendingCount]'`
    # でタブ区切り1行を受け取る。ここも同じ形で返す。
    printf '%s\t%s\t%s\t%s\n' \
      "${!td:-arn:aws:ecs:ap-northeast-1:1:task-definition/soc-app-${svc}:88}" \
      "$desired" "$running" "${!pc:-0}"
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
    svc=$(arg_after --service "$@")
    newtd=$(arg_after --task-definition "$@") || newtd=""
    count=$(arg_after --desired-count "$@") || count=""
    # タスク定義の付け替え（＝復旧）と desired の変更（＝prod-scale.sh）を混ぜない。
    # 混ぜると「復旧したつもりが起動しただけ」でもテストが緑になる。
    [ -n "$newtd" ] && echo "update-service $svc $newtd" >> "$CALLS"
    if [ -n "$count" ]; then
      mkdir -p "$STATE" && echo "$count" > "$STATE/DC_$svc"
    fi
    ;;
  "application-autoscaling describe-scalable-targets")
    # chroma/rag-review と同じ「スケーラブルターゲット未登録」を返し、
    # prod-scale.sh を desired の更新だけで走らせる。
    echo "None"
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
STATE="$WORK/state"

run() {
  local name="$1" want_exit="$2" want_call="$3"; shift 3
  local out actual calls
  CALLS="$WORK/calls"
  : > "$CALLS"
  # KEEP_STATE=1 のときだけ直前のケースが書いた desired を引き継ぐ
  # （prod-scale.sh -> 検査 の順序をそのまま通すケースで使う）。
  [ -n "${KEEP_STATE:-}" ] || rm -rf "$STATE"
  out=$(PATH="$WORK/bin:$PATH" PROJECT_NAME=soc-app REGISTRY="$REGISTRY" CALLS="$CALLS" \
    STATE="$STATE" \
    TD_backend="${TD_backend:-}" TD_frontend="${TD_frontend:-}" \
    DC_backend="${DC_backend:-}" RC_backend="${RC_backend:-}" PC_backend="${PC_backend:-}" \
    RC_SEQ="${RC_SEQ:-}" \
    DRAIN_WAIT_INTERVAL=0 DRAIN_WAIT_ATTEMPTS="${DRAIN_WAIT_ATTEMPTS:-5}" \
    MISSING_TAGS="${MISSING_TAGS:-}" ERROR_TAGS="${ERROR_TAGS:-}" \
    NON_ECR="${NON_ECR:-}" BROKEN_TD="${BROKEN_TD:-}" \
    bash "$TARGET" "$@" 2>&1)
  actual=$?
  # `VAR=x run ...` の前置代入は、関数呼び出しでは呼び出し後も残る（bashの仕様）。
  # 消さないと次のケースへ設定が漏れ、「別の条件を確かめたつもり」になる。
  unset TD_backend TD_frontend DC_backend RC_backend PC_backend RC_SEQ DRAIN_WAIT_ATTEMPTS \
    MISSING_TAGS ERROR_TAGS NON_ECR BROKEN_TD KEEP_STATE
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

# desired=1 でも running=0 なら戻す（Codex #1534 指摘: 一時起動中に復旧が発動しない）。
#
# 参照先のイメージがECRに無い時点でタスクは起動できないので、running=0 は
# 「これから起動する」ではなく「起動できない」。失うタスクが無い以上、
# 壊れた定義を指したまま残す方が危険（停止日は誰も気づかず次の稼働日に落ちる）。
# desired を条件に入れると、停止日デプロイの一時起動や、0へ戻すステップが
# override=on で見送られた場合に、検知だけして直せないまま終わる。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend=1 RC_backend=0 \
  run "一時起動中(desired=1 running=0)でも戻す" 1 "update-service backend $ARN88" \
  "backend=$ARN88"
# 起動状態(desired)は触らない。稼働日の本番でもここは変えてはいけない。
if [ -f "$STATE/DC_backend" ]; then
  echo "FAIL 復旧が desiredCount まで書き換えている（起動状態を変えてはいけない）"
  fail=$((fail + 1))
fi

# ドレインが終わらない（待っても running=1 のまま）なら、残っているタスクを失うので戻さない。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend=0 RC_backend=1 \
  run "ドレインが終わらなければ(running=1のまま)戻さず失敗" 1 "" "backend=$ARN88"

# desired/running が取れない（空・None）場合も「稼働中かもしれない」側へ倒す。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend="None" RC_backend="None" \
  run "desired/running が不明なら戻さず失敗" 1 "" "backend=$ARN88"

# PENDING / ACTIVATING のタスクが居る間は戻さない（Codex #1563 指摘）。
#
# 旧イメージを既に pull し終えたタスクが RUNNING へ遷移する直前にECRのタグが失効した場合、
# runningCount=0 でも「これから起動するタスク」が居る。ここで $wanted（直前にヘルス
# チェックで落ちたリビジョン）へ向け直すと、minimumHealthyPercent=0 のため
# 回復しかけた本番を再び止める。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend=0 RC_backend=0 PC_backend=1 \
  run "起動中(pending=1)なら running=0 でも戻さず失敗" 1 "" "backend=$ARN88"
case "${LAST_OUT:-}" in
  *"pending=1"*) : ;;
  *) echo "FAIL pending で見送ったことがログに出ていない"; fail=$((fail + 1)) ;;
esac

# pending も取れない（None）なら「起動中かもしれない」側へ倒す。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend=0 RC_backend=0 PC_backend="None" \
  run "pending が不明なら戻さず失敗" 1 "" "backend=$ARN88"

# 非同期なドレインを再現する（Codex #1563 指摘）。
#
# `update-service --desired-count 0` が返った直後も既存タスクはしばらく running=1 の
# まま残る。待たずに検査すると running=1 を見て復旧を見送り、その後 0 になっても
# 再試行が無いため、壊れたタスク定義のまま次の稼働日を迎える。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend=0 RC_SEQ="1 1 1 0" \
  run "ドレイン完了(running 1→0)を待って復旧する" 1 "update-service backend $ARN88" \
  "backend=$ARN88"

# 稼働日（desired=1）はタスクが減らないので待たない。待つと、その間に
# 「稼働中のタスクを失わない」判断が遅れるだけで得が無い。
TD_backend="$ARN86" MISSING_TAGS="img86" DC_backend=1 RC_SEQ="1 0" \
  run "稼働日(desired=1)はドレインを待たず即座に見送る" 1 "" "backend=$ARN88"

# 停止日デプロイの実際の順序をそのまま通す（Codex #1534 指摘: スクリプト単体の
# desired=0 ケースしか無く、本番ワークフローの順序では落ちなかった）。
#
#   一時起動(desired=1) -> サーキットブレーカーが :86 へ戻す(イメージは失効済み)
#   -> 「Scale production services back to zero」= prod-scale.sh 0
#   -> 「Verify task definition images exist in ECR」= 検査 -> :88 へ復旧
#
# prod-scale.sh 0 が返った直後はまだ running=1（ドレイン中）で、数回後に0になる。
# ここを running=0 で始めてしまうと、実運用で必ず通るドレイン期間を飛ばしてしまう。
rm -rf "$STATE"; mkdir -p "$STATE"
echo 1 > "$STATE/DC_backend" # 停止日の一時起動
if ! scale_out=$(PATH="$WORK/bin:$PATH" PROJECT_NAME=soc-app STATE="$STATE" \
  CALLS="$WORK/calls" bash "$SCALE" 0 2>&1); then
  echo "FAIL 停止日デプロイの順序: prod-scale.sh 0 が失敗した"
  printf '%s\n' "$scale_out" | sed 's/^/     /'
  fail=$((fail + 1))
elif [ "$(cat "$STATE/DC_backend")" != "0" ]; then
  echo "FAIL 停止日デプロイの順序: prod-scale.sh 0 が desired を0にしていない"
  fail=$((fail + 1))
fi
TD_backend="$ARN86" MISSING_TAGS="img86" RC_SEQ="1 1 0" KEEP_STATE=1 \
  run "停止日デプロイの順序(一時起動→0へ戻す→ドレイン待ち→検査)で壊れた定義が復旧する" 1 \
  "update-service backend $ARN88" \
  "backend=$ARN88" "frontend=" "rag-review=" "chroma="
if [ "$(cat "$STATE/DC_backend")" != "0" ]; then
  echo "FAIL 復旧のあとに本番が起動状態へ戻っている（停止日に課金が残る）"
  fail=$((fail + 1))
fi
rm -rf "$STATE"

# サービス名を1つも渡さない使い方は事故（全サービス素通りで緑になる）。
run "引数無しは使用方法エラー" 2 ""

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS"
