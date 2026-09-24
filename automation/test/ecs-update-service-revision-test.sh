#!/bin/bash
# 運用手順に「リビジョンを省略した update-service」が混ざらないことを固定する。
#
# `aws ecs update-service --task-definition soc-app-backend`（リビジョン省略）は最新の
# ACTIVE リビジョンを選ぶ。terraform apply 直後はそれが var.backend_image / var.frontend_image
# （tfvars 例では `:latest`）から登録した定義になるが、本番デプロイ(deployment.yml)は
# SHA タグしか push せず latest を更新しない。手順どおりに叩くと稼働中のイメージが
# 巻き戻るか、存在しないタグでデプロイが落ちる。
# 必ず register-task-definition で得た ARN / family:revision を渡すこと。
#
# 使い方: ./automation/test/ecs-update-service-revision-test.sh

set -uo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

fail=0
found=0

# 行継続(\)を畳んでから update-service のコマンドを1行として取り出す。
while IFS= read -r file; do
  while IFS= read -r cmd; do
    td=$(printf '%s\n' "$cmd" | sed -n 's/.*--task-definition[[:space:]]\{1,\}\([^[:space:]]*\).*/\1/p' | tr -d '"'"'")
    if [ -z "$td" ]; then
      # --desired-count や --force-new-deployment だけの更新は稼働中の定義を保つので対象外
      continue
    fi
    found=$((found + 1))
    if [ "${td#\$}" = "$td" ] && [ "${td#*:}" = "$td" ]; then
      # 変数でもなく `family:revision` / ARN でもない ＝ family 直書き
      echo "FAIL $file: --task-definition '$td' はリビジョン省略。稼働中イメージを巻き戻す"
      fail=$((fail + 1))
    else
      echo "ok   $file: --task-definition '$td'"
    fi
  done < <(sed -e :a -e '/\\$/N; s/\\\n/ /; ta' "$file" | grep 'aws ecs update-service')
done < <(grep -rl 'aws ecs update-service' "$ROOT/docs" "$ROOT/automation" "$ROOT/.github" 2>/dev/null | grep -v 'ecs-update-service-revision-test.sh')

echo "検査した update-service: ${found}件"
if [ "$fail" -ne 0 ]; then
  echo "NG ${fail}件"
  exit 1
fi
echo "OK"
