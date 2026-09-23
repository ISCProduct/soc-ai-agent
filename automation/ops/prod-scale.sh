#!/bin/bash
# 本番ECSサービスをまとめて起動/停止する。
#
# desired_count だけを変えると min_capacity=0 のままCPUターゲット追跡がタスクを0へ縮退させ、
# タスク0ではCPUメトリクスが出ないため自力復帰できない（稼働日に断続的な503になる）。
# そのため desired と min_capacity を必ず揃える。
#
# 起動順は chroma -> rag-review -> backend -> frontend。rag-review は Cloud Map 経由で
# chroma に接続し、chroma が居ないとヘルスチェックが503を返して healthy にならない。
# RAG(履歴書レビュー/ES添削)は backend の同期依存でもあるため、漏らすとRAG機能だけ落ちる。
#
# 使い方: PROJECT_NAME=soc-app ./automation/ops/prod-scale.sh <desired_count>
#
# ponytail: 同じ処理が .github/workflows/prod-uptime-scheduler.yml にもある。
# スケジューラは毎時本番を操作しているため、本スクリプトへの寄せは別PRで行う。

set -uo pipefail

DESIRED="${1:?desired_count を指定してください}"
: "${PROJECT_NAME:?PROJECT_NAME を指定してください}"

SERVICES="chroma rag-review backend frontend"
failed=""

for SERVICE in $SERVICES; do
  RESOURCE_ID="service/${PROJECT_NAME}/${SERVICE}"

  # max は Terraform 側の値を引き継ぐ。ハードコードすると乖離する。
  if MAX=$(aws application-autoscaling describe-scalable-targets \
    --service-namespace ecs --resource-ids "$RESOURCE_ID" \
    --scalable-dimension ecs:service:DesiredCount \
    --query 'ScalableTargets[0].MaxCapacity' --output text); then
    if [ -z "$MAX" ] || [ "$MAX" = "None" ]; then
      # chroma/rag-review はオートスケーリング対象外（Terraformでも未登録）。
      echo "$SERVICE: スケーラブルターゲット未登録、desired_count のみ更新する"
    elif aws application-autoscaling register-scalable-target \
      --service-namespace ecs --resource-id "$RESOURCE_ID" \
      --scalable-dimension ecs:service:DesiredCount \
      --min-capacity "$DESIRED" --max-capacity "$MAX" > /dev/null; then
      echo "$SERVICE min_capacity -> $DESIRED (max=$MAX)"
    else
      echo "$SERVICE: register-scalable-target に失敗（IAM権限: application-autoscaling:RegisterScalableTarget）" >&2
      failed=1
    fi
  else
    echo "$SERVICE: describe-scalable-targets に失敗（IAM権限: application-autoscaling:DescribeScalableTargets）" >&2
    failed=1
  fi

  if ! aws ecs update-service --cluster "$PROJECT_NAME" --service "$SERVICE" \
    --desired-count "$DESIRED" > /dev/null; then
    # 握りつぶすと、停止したつもりでタスクが動き続けても気づけない。
    echo "$SERVICE: desired_count=$DESIRED への更新に失敗" >&2
    failed=1
    continue
  fi
  echo "$SERVICE desired_count -> $DESIRED"
done

if [ -n "$failed" ]; then
  echo "本番サービスのスケール変更に失敗したものがあります" >&2
  exit 1
fi
