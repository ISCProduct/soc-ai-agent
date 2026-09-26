#!/bin/bash
# 本番サービスが参照しているタスク定義のイメージがECRに存在するかを検査し、
# 存在しなければ「今回のデプロイで登録したリビジョン」へ戻す。
# 戻すのは desired=0 かつ running=0 のサービスだけ。稼働中のタスクがある場合は
# 検査結果を出して失敗させるだけにする（詳細は後述の「稼働中のサービスは触らない」）。
#
# なぜ必要か（#1518）:
#   ECSのデプロイサーキットブレーカーは失敗すると「直前の安定デプロイ」へ戻すが、
#   そのタスク定義が参照するイメージがECRに残っているかは見ない。
#   2026-09-25 の本番デプロイでは、戻された先のイメージがECRのライフサイクルで
#   既に失効しており、backend が CannotPullContainerError のまま
#   「起動できないタスク定義を指したサービス」として残った。
#   停止日は desired=0 なので即時の障害は出ず、次の稼働日に初めて気づくことになる。
#
# 使い方:
#   PROJECT_NAME=soc-app ./automation/ops/verify-task-def-images.sh \
#     backend=arn:aws:ecs:...:task-definition/soc-app-backend:88 \
#     frontend= \
#     chroma=
#
#   引数は `サービス名=戻し先のタスク定義ARN`。ARNが空なら検査のみ（戻さない）。
#   ECR以外のイメージ（chroma の chromadb/chroma など）は検査対象外。
#
# 終了コード: 0=全サービス健全 / 1=存在しないイメージを見つけた（戻せたかは別途ログ）

set -uo pipefail

: "${PROJECT_NAME:?PROJECT_NAME を指定してください}"
[ "$#" -gt 0 ] || { echo "サービスを1つ以上指定してください" >&2; exit 2; }

# ecr_state は「そのイメージ参照がECRに在るか」を返す。
#   0 = 存在する / ECR以外のイメージ（検査対象外）
#   1 = 存在しない（ImageNotFound / RepositoryNotFound）
#   2 = 判定できない（権限・スロットリング・ネットワーク等）
#
# 判定不能を「存在しない」に倒すと、API障害でタスク定義を書き換えてしまう。
# 逆に「存在する」へ倒すと壊れたまま緑で終わる。3値で分けて呼び出し側に決めさせる。
ecr_state() {
  local img="$1" path repo image_id err
  case "$img" in
    *.dkr.ecr.*.amazonaws.com/*) ;;
    *) return 0 ;;
  esac

  path="${img#*/}" # レジストリホストを落として repo[:tag|@digest] にする
  case "$path" in
    *@*) repo="${path%@*}"; image_id="imageDigest=${path#*@}" ;;
    *:*) repo="${path%:*}"; image_id="imageTag=${path##*:}" ;;
    *) repo="$path"; image_id="imageTag=latest" ;;
  esac

  if err=$(aws ecr describe-images --repository-name "$repo" --image-ids "$image_id" 2>&1 >/dev/null); then
    return 0
  fi
  if printf '%s' "$err" | grep -qE 'ImageNotFoundException|RepositoryNotFoundException'; then
    echo "  イメージが存在しない: $img" >&2
    return 1
  fi
  echo "  ECRを参照できない: $img" >&2
  printf '  %s\n' "$err" >&2
  return 2
}

# task_def_state はタスク定義が参照する全イメージを検査し、最も悪い状態を返す。
task_def_state() {
  local td="$1" images img state worst=0
  if ! images=$(aws ecs describe-task-definition --task-definition "$td" \
    --query 'taskDefinition.containerDefinitions[].image' --output text 2>&1); then
    echo "  タスク定義を取得できない: $td" >&2
    printf '  %s\n' "$images" >&2
    return 2
  fi
  for img in $images; do
    ecr_state "$img"
    state=$?
    [ "$state" -gt "$worst" ] && worst=$state
  done
  return $worst
}

fail=0
for arg in "$@"; do
  service="${arg%%=*}"
  wanted="${arg#*=}"
  [ "$wanted" = "$service" ] && wanted=""

  # desired / running も一緒に読む。復旧(update-service)して良いのは「今どのタスクも
  # 動いていないサービス」だけなので、タスク定義だけでは判断できない。
  if ! svc=$(aws ecs describe-services --cluster "$PROJECT_NAME" --services "$service" \
    --query 'services[0].[taskDefinition,desiredCount,runningCount]' --output text); then
    echo "::error::$service: サービスを取得できず、参照しているタスク定義を検査できません"
    fail=1
    continue
  fi
  current=$(printf '%s' "$svc" | awk '{print $1}')
  desired=$(printf '%s' "$svc" | awk '{print $2}')
  running=$(printf '%s' "$svc" | awk '{print $3}')
  if [ -z "$current" ] || [ "$current" = "None" ]; then
    echo "::error::$service: サービスのタスク定義が取得できません(値='$current')"
    fail=1
    continue
  fi

  task_def_state "$current"
  case $? in
    0)
      echo "ok   $service: $current のイメージはECRに存在する"
      continue
      ;;
    2)
      echo "::error::$service: $current のイメージ存在を確認できませんでした"
      fail=1
      continue
      ;;
  esac

  # ここから「参照先のイメージが無い」= 起動すれば必ず CannotPullContainerError。
  fail=1
  if [ -z "$wanted" ] || [ "$wanted" = "None" ]; then
    echo "::error::$service が参照する $current のイメージがECRにありません。戻し先が不明なため手動で復旧してください"
    continue
  fi
  if [ "$wanted" = "$current" ]; then
    echo "::error::$service は今回デプロイしたリビジョン($current)を指していますが、そのイメージがECRにありません"
    continue
  fi

  # 稼働中のサービスは触らない。
  #
  # 稼働日のデプロイで新リビジョンがヘルスチェックに落ち、サーキットブレーカーが
  # 「ECRのタグは失効済みだが既存タスクは動き続けている」旧リビジョンへ戻した場合、
  # ここで $wanted（＝直前に落ちたリビジョン）へ update-service すると、
  # minimumHealthyPercent=0 のため健全な稼働タスクが先に落とされ、
  # 起動しないタスクに入れ替わって本番が停止する。
  # このステップは安定待ち失敗時にも always() で走るので、その経路に必ず当たる。
  #
  # 逆に desired=0 / running=0 のサービス（停止日の本番、または起動前）は、
  # 壊れたタスク定義を指したまま残す方が危険（次の稼働日に起動しない）。
  # 失うタスクが無いのでここだけ自動で向け直す。
  if [ "$desired" != "0" ] || [ "$running" != "0" ]; then
    echo "::error::$service は $current のイメージがECRにありませんが、タスクが稼働中(desired=$desired running=$running)のため自動では戻しません。稼働中のタスクを失うおそれがあります。手動で復旧してください(戻し先候補: $wanted)"
    continue
  fi

  task_def_state "$wanted"
  if [ $? -ne 0 ]; then
    echo "::error::$service: 戻し先 $wanted のイメージも確認できないため、タスク定義は変更しません"
    continue
  fi

  # 戻し先は今回ビルドしてpushしたイメージなので、ここへ向け直せば次の起動で上がる。
  # リビジョンまで固定したARNを渡す（family だけだと最新ACTIVEが選ばれて巻き戻る）。
  if aws ecs update-service --cluster "$PROJECT_NAME" --service "$service" \
    --task-definition "$wanted" > /dev/null; then
    echo "::error::$service が存在しないイメージ($current)を指していたため $wanted へ戻しました。デプロイは失敗として扱います"
  else
    echo "::error::$service を $wanted へ戻せませんでした。手動で update-service してください"
  fi
done

exit $fail
