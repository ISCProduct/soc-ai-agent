#!/bin/bash
# 本番デプロイのマイグレーションがRDS停止中でも通ることを、ワークフロー定義の構造で固定する。
#
# 本番は「指定日のみ終日起動」ポリシー(prod-uptime-scheduler)のため、通常日はRDSが停止している。
# ガードが無いとマイグレーションタスクがDBへ到達できず、mainへのマージが必ず失敗する
# （実際に #1488 のデプロイが "no route to host" で落ちた 2026-09-23）。
#
# 使い方: ./automation/test/prod-deploy-db-guard-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WF="$ROOT/.github/workflows/deployment.yml"

fail=0
line_of() { grep -n "$1" "$WF" | head -1 | cut -d: -f1; }

START=$(line_of "Start production RDS if stopped")
MIGRATE=$(line_of "Run DB migration (production)")
STOP=$(line_of "Stop production RDS if this deploy started it")

if [ -z "$START" ]; then
  echo "FAIL 本番RDSの起動ステップが無い（停止中はマイグレーションが必ず失敗する）"
  fail=$((fail + 1))
elif [ -z "$MIGRATE" ] || [ "$START" -ge "$MIGRATE" ]; then
  echo "FAIL 本番RDSの起動はマイグレーションより前に置くこと（start=$START migrate=${MIGRATE}）"
  fail=$((fail + 1))
else
  echo "ok   本番RDSの起動がマイグレーションより前にある（$START < ${MIGRATE}）"
fi

if [ -z "$STOP" ]; then
  echo "FAIL 起動したRDSを停止へ戻すステップが無い（通常日に課金が残る）"
  fail=$((fail + 1))
elif [ -n "$MIGRATE" ] && [ "$STOP" -le "$MIGRATE" ]; then
  echo "FAIL RDSの停止はマイグレーションより後に置くこと（stop=$STOP migrate=${MIGRATE}）"
  fail=$((fail + 1))
else
  echo "ok   起動したRDSを停止へ戻すステップがある（${STOP}）"
  # 停止ステップ本体（次のステップ行まで）を切り出して検査する。
  STOP_END=$(awk -v s="$STOP" 'NR > s && /^      - name: /{print NR; exit}' "$WF")
  [ -n "$STOP_END" ] || STOP_END=$((STOP + 40))
  STOP_BODY=$(sed -n "${STOP},${STOP_END}p" "$WF")

  # 稼働中の本番を止めないためのガード。条件が消えると展示会当日にDBを落とす。
  if ! grep -q "desiredCount" <<< "$STOP_BODY"; then
    echo "FAIL 停止ステップに desiredCount の確認が無い（稼働中の本番を止めうる）"
    fail=$((fail + 1))
  fi
  # 状態を取得できないまま停止すると、稼働中のDBを落としうる（取得失敗は停止しない）。
  if ! grep -q "ECSサービスの状態を取得できないためRDSは停止しない" <<< "$STOP_BODY"; then
    echo "FAIL describe-services の失敗時に停止を中止していない"
    fail=$((fail + 1))
  fi
  # 実行中のマイグレーションからDB接続を切ると dirty なマイグレーションが残る。
  if ! grep -q "lastStatus" <<< "$STOP_BODY"; then
    echo "FAIL マイグレーションタスクの停止確認が無い（DDL実行中にDBを落としうる）"
    fail=$((fail + 1))
  fi
  # 停止失敗を握りつぶすと、非稼働日にRDSが起動したまま残っても気づけない。
  if grep -q "stop-db-instance .*|| true" <<< "$STOP_BODY"; then
    echo "FAIL stop-db-instance の失敗を握りつぶしている"
    fail=$((fail + 1))
  fi
  if ! sed -n "${STOP},$((STOP + 3))p" "$WF" | grep -q "started_by_deploy == 'true'"; then
    echo "FAIL 停止ステップが started_by_deploy を見ていない（他者が起動したDBを止めうる）"
    fail=$((fail + 1))
  fi
fi

# 停止日のデプロイでは、新しいタスク定義が起動するかを一時起動して確認し、必ず0へ戻す。
# 戻し忘れると非稼働日にFargateが動き続ける。
UP=$(line_of "Start production services for verification")
WAIT=$(line_of "Wait for services to stabilize")
DOWN=$(line_of "Scale production services back to zero")

if [ -z "$UP" ] || [ -z "$DOWN" ]; then
  echo "FAIL 本番サービスの一時起動 / 0へ戻す ステップが揃っていない"
  fail=$((fail + 1))
else
  if [ -n "$WAIT" ] && { [ "$UP" -ge "$WAIT" ] || [ "$DOWN" -le "$WAIT" ]; }; then
    echo "FAIL 一時起動 → 安定待ち → 0へ戻す の順になっていない（up=${UP} wait=${WAIT} down=${DOWN}）"
    fail=$((fail + 1))
  fi
  if [ -n "$STOP" ] && [ "$DOWN" -ge "$STOP" ]; then
    echo "FAIL サービスを0へ戻すのはRDS停止より前にすること（down=${DOWN} stopRds=${STOP}）"
    fail=$((fail + 1))
  fi
  # always() が無いと、安定待ちで落ちたときにタスクが起動したまま課金が続く。
  DOWN_END=$(awk -v s="$DOWN" 'NR > s && /^      - name:/ { print NR - 1; exit }' "$WF")
  [ -z "$DOWN_END" ] && DOWN_END=$((DOWN + 40))
  # `if:` はステップ名の直後にしか書けない。次のステップ直前までを見ると、そのステップの
  # 説明コメント（「…always()。」）を拾って always() が無くても通ってしまう。
  if ! sed -n "${DOWN},$((DOWN + 3))p" "$WF" | grep -q "always()"; then
    echo "FAIL 0へ戻すステップに always() が無い（失敗時に起動したまま残る）"
    fail=$((fail + 1))
  fi
  if ! sed -n "${DOWN},$((DOWN + 3))p" "$WF" | grep -q "was_down == 'true'"; then
    echo "FAIL 0へ戻すステップが was_down を見ていない（稼働日の本番を0にしうる）"
    fail=$((fail + 1))
  fi
  # 変更のあったサービスだけ起動すると、chroma 不在で rag-review が healthy にならない。
  # min_capacity を揃えないとターゲット追跡が0へ縮退し、壊れたイメージでも安定待ちが通る。
  SCALE="$ROOT/automation/ops/prod-scale.sh"
  # ステップ本体は複数行になりうる（0へ戻す側は運用指示の読み直しガードを持つ）。
  # 次のステップ宣言(- name:)までを本体として見る。3行固定だと、ガードを足した
  # だけで落ちてしまい、本当に見たい「prod-scale.sh を経由しているか」を見失う。
  for step in "$UP" "$DOWN"; do
    step_end=$(awk -v s="$step" 'NR > s && /^      - name:/ { print NR - 1; exit }' "$WF")
    [ -z "$step_end" ] && step_end=$((step + 40))
    if ! sed -n "${step},${step_end}p" "$WF" | grep -q "automation/ops/prod-scale.sh"; then
      echo "FAIL 一時起動/停止が automation/ops/prod-scale.sh を使っていない"
      fail=$((fail + 1))
      break
    fi
  done
  if [ ! -x "$SCALE" ]; then
    echo "FAIL automation/ops/prod-scale.sh が無い、または実行権限が無い"
    fail=$((fail + 1))
  else
    if ! grep -q "register-scalable-target" "$SCALE"; then
      echo "FAIL prod-scale.sh が min_capacity を揃えていない（ターゲット追跡が0へ縮退する）"
      fail=$((fail + 1))
    fi
    CHROMA=$(grep -n "SERVICES=" "$SCALE" | head -1)
    case "$CHROMA" in
      *"chroma rag-review"*) : ;;
      *) echo "FAIL prod-scale.sh の起動順に chroma -> rag-review が無い"; fail=$((fail + 1)) ;;
    esac
  fi

  # /prod on は日付リストを無視して起動させる指示。ここを見落とすと手動起動した本番を落とす。
  DOWN_END=$(awk -v s="$DOWN" 'NR > s && /^      - name: /{print NR; exit}' "$WF")
  [ -n "$DOWN_END" ] || DOWN_END=$((DOWN + 40))
  DOWN_BODY=$(sed -n "${DOWN},${DOWN_END}p" "$WF")
  if ! grep -q 'OVERRIDE_NOW%%:\*' <<< "$DOWN_BODY"; then
    echo "FAIL 0へ戻すステップが override=on を見ていない（手動起動した本番を落としうる）"
    fail=$((fail + 1))
  fi
  # ドレイン中のタスクが残ったままDBを落とすと、処理中のリクエストが切れる。
  if ! grep -q "runningCount" <<< "$STOP_BODY"; then
    echo "FAIL RDS停止前に runningCount の確認が無い（ドレイン中にDBを落としうる）"
    fail=$((fail + 1))
  fi
  # ワンオフタスクを残すと、次のスケジューラがRDSだけ止めて実行中のDDLを切る。
  if ! grep -q "stop-task" <<< "$STOP_BODY"; then
    echo "FAIL マイグレーションタスクを停止させる処理が無い（残留してDDLが切られる）"
    fail=$((fail + 1))
  fi

  # 反映確認を人の目視に任せない（#1518）。ECSの安定待ちは「タスクが立った」までしか
  # 見ておらず、画面が出ているかは分からない。停止日は後段で0へ戻すため、スモークは
  # 一時起動の最中でなければ実行できない。
  SMOKE=$(line_of "Run Playwright smoke (production)")
  if [ -z "$SMOKE" ]; then
    echo "FAIL 本番デプロイに Playwright スモークが無い（反映確認が人の目視に戻る）"
    fail=$((fail + 1))
  elif [ -n "$WAIT" ] && { [ "$SMOKE" -le "$WAIT" ] || [ "$SMOKE" -ge "$DOWN" ]; }; then
    echo "FAIL スモークは 安定待ち→スモーク→0へ戻す の順に置くこと（wait=${WAIT} smoke=${SMOKE} down=${DOWN}）"
    fail=$((fail + 1))
  else
    echo "ok   一時起動→安定待ち→スモーク→0へ戻す の順になっている"
    SMOKE_END=$(awk -v s="$SMOKE" 'NR > s && /^      - name:/ { print NR - 1; exit }' "$WF")
    [ -z "$SMOKE_END" ] && SMOKE_END=$((SMOKE + 20))
    SMOKE_BODY=$(sed -n "${SMOKE},${SMOKE_END}p" "$WF")
    # continue-on-error を付けると「スモークが落ちてもデプロイ成功」になり意味が無い。
    if grep -q "continue-on-error" <<< "$SMOKE_BODY"; then
      echo "FAIL スモークが continue-on-error（失敗してもデプロイが成功扱いになる）"
      fail=$((fail + 1))
    fi
    if ! grep -q "playwright.smoke.config.ts" <<< "$SMOKE_BODY"; then
      echo "FAIL スモークが playwright.smoke.config.ts を使っていない（staging と別実装になる）"
      fail=$((fail + 1))
    fi
    # 通知を二重実装しない（deploy-smoke.yml と同じスクリプトを使う）。
    if ! grep -q "automation/deploy-smoke/notify-discord.sh" "$WF"; then
      echo "FAIL 本番スモークの結果通知が既存スクリプトを使っていない"
      fail=$((fail + 1))
    fi
  fi
  if [ ! -f "$ROOT/frontend/e2e/smoke/smoke.spec.ts" ]; then
    echo "FAIL frontend/e2e/smoke/smoke.spec.ts が無い（staging/production 共用のスモーク）"
    fail=$((fail + 1))
  fi

  # サーキットブレーカーの戻し先イメージがECRから消えていると、起動不能なタスク定義を
  # 指したまま残る（#1518: 停止日は desired=0 なので次の稼働日まで気づけない）。
  VERIFY=$(line_of "Verify task definition images exist in ECR")
  if [ -z "$VERIFY" ]; then
    echo "FAIL タスク定義のイメージ存在検査が無い（起動不能なリビジョンを指したまま残る）"
    fail=$((fail + 1))
  else
    VERIFY_END=$(awk -v s="$VERIFY" 'NR > s && /^      - name:/ { print NR - 1; exit }' "$WF")
    [ -z "$VERIFY_END" ] && VERIFY_END=$((VERIFY + 20))
    VERIFY_BODY=$(sed -n "${VERIFY},${VERIFY_END}p" "$WF")
    # 失敗した経路でこそ必要な検査。always() が無いと壊れた状態のまま終わる。
    # `if:` はステップ名の直後にしか書けない。次のステップまでを見ると、そのステップの
    # 説明コメント（「always() が無いと…」）を拾って常に通ってしまう。
    if ! sed -n "${VERIFY},$((VERIFY + 3))p" "$WF" | grep -q "always()"; then
      echo "FAIL イメージ存在検査に always() が無い（ロールバックした時に走らない）"
      fail=$((fail + 1))
    fi
    if ! grep -q "automation/ops/verify-task-def-images.sh" <<< "$VERIFY_BODY"; then
      echo "FAIL イメージ存在検査が automation/ops/verify-task-def-images.sh を使っていない"
      fail=$((fail + 1))
    fi
    if [ ! -x "$ROOT/automation/ops/verify-task-def-images.sh" ]; then
      echo "FAIL automation/ops/verify-task-def-images.sh が無い、または実行権限が無い"
      fail=$((fail + 1))
    fi
  fi

  if [ "$fail" -eq 0 ]; then
    echo "ok   一時起動→安定待ち→スモーク→0へ戻す→RDS停止 の順で、失敗時も0へ戻る"
    echo "ok   override=on / runningCount / ワンオフタスク停止のガードがある"
    echo "ok   起動/停止は prod-scale.sh 経由（chroma込み・min_capacity同期）"
    echo "ok   タスク定義のイメージ存在検査が always() で走る"
  fi
fi

# 毎時の起動/停止ジョブ(prod-uptime-scheduler)が、デプロイ中の本番RDSを止めないこと。
# 2026-09-25 はこれが無く、デプロイの最中にRDSを停止され backend がDBへ繋がらず、
# サーキットブレーカーがECRから消えた旧イメージへロールバックした（#1518）。
SCHED="$ROOT/.github/workflows/prod-uptime-scheduler.yml"
if [ ! -f "$SCHED" ]; then
  echo "FAIL prod-uptime-scheduler.yml が無い"
  fail=$((fail + 1))
else
  if ! grep -q "workflow deployment.yml" "$SCHED"; then
    echo "FAIL スケジューラが本番デプロイの実行中かを見ていない（デプロイ中にRDSを止める）"
    fail=$((fail + 1))
  elif ! grep -q 'DESIRED" == "0" \]; then' "$SCHED"; then
    echo "FAIL デプロイ中の見送りが停止側(DESIRED=0)限定になっていない（稼働日の起動が遅れる）"
    fail=$((fail + 1))
  else
    echo "ok   スケジューラはデプロイ中の停止処理を見送る（起動側は見送らない）"
  fi
  # permissions に actions:read が無いと gh run list が失敗し、ガードが常に素通りする。
  if ! grep -q "actions: read" "$SCHED"; then
    echo "FAIL スケジューラの permissions に actions: read が無い（デプロイ実行中を読めない）"
    fail=$((fail + 1))
  fi
fi

echo
if [ "$fail" -gt 0 ]; then echo "FAIL ($fail 件)"; exit 1; fi
echo "PASS"
