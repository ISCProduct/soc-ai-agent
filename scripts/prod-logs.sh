#!/usr/bin/env bash
# 本番(AWS ECS on Fargate)のCloudWatchログを見るためのラッパー (#1010 運用支援)
#
# aws logs tail をそのまま使えるようにしたショートカット。
# ロググループ名を覚えなくてよくすることが目的で、独自のログ取得は行わない。
set -euo pipefail

CLUSTER="${PROD_ECS_CLUSTER:-soc-app}"
REGION="${AWS_REGION:-ap-northeast-1}"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/prod-logs.sh <service> [aws logs tail のオプション...]

  service: backend | frontend | rag | chroma | all
  既定は直近30分。オプションはそのまま `aws logs tail` へ渡す。

例:
  ./scripts/prod-logs.sh backend                    # 直近30分
  ./scripts/prod-logs.sh backend --since 3h         # 直近3時間
  ./scripts/prod-logs.sh backend --follow           # ライブ追尾
  ./scripts/prod-logs.sh backend --filter-pattern ERROR
  ./scripts/prod-logs.sh all --since 1h             # 全サービス横断

補足:
  本番は展示会運用のため通常は desired_count=0 で停止している。
  停止中は新しいログが出ないため、まず `./scripts/prod-logs.sh status` で稼働状態を確認すること。
EOF
}

log_group_for() {
  case "$1" in
    backend)  echo "/ecs/soc-app/backend" ;;
    frontend) echo "/ecs/soc-app/frontend" ;;
    rag|rag-review) echo "/ecs/soc-app/rag-review" ;;
    chroma)   echo "/ecs/soc-app/chroma" ;;
    *) return 1 ;;
  esac
}

# サービスの稼働状態とロググループの最終書き込み時刻を出す。
# 「ログが出ない」のが停止中のためなのか異常なのかを切り分けるために使う。
show_status() {
  echo "== ECS サービス (cluster: $CLUSTER) =="
  aws ecs describe-services --cluster "$CLUSTER" --region "$REGION" \
    --services backend frontend rag-review chroma \
    --query 'services[].{service:serviceName,desired:desiredCount,running:runningCount,status:status}' \
    --output table

  echo "== ロググループの最終イベント =="
  printf '%-12s %-24s %s\n' "SERVICE" "LAST EVENT (JST)" "LOG GROUP"
  for svc in backend frontend rag chroma; do
    local group last
    group="$(log_group_for "$svc")"
    # --max-items はページトークン行を追加で出すため --limit を使い、念のため1行目だけ取る
    last="$(aws logs describe-log-streams --log-group-name "$group" --region "$REGION" \
      --order-by LastEventTime --descending --limit 1 \
      --query 'logStreams[0].lastEventTimestamp' --output text 2>/dev/null | head -1 || echo "None")"
    if [[ "$last" == "None" || -z "$last" ]]; then
      printf '%-12s %-24s %s\n' "$svc" "(なし)" "$group"
    else
      printf '%-12s %-24s %s\n' "$svc" \
        "$(python3 -c "import datetime,sys;print(datetime.datetime.fromtimestamp(int(sys.argv[1])/1000).strftime('%Y-%m-%d %H:%M:%S'))" "$last")" \
        "$group"
    fi
  done
}

main() {
  if ! command -v aws >/dev/null 2>&1; then
    echo "aws CLI が見つかりません。AWS CLI v2 をインストールしてください。" >&2
    exit 1
  fi
  if ! aws sts get-caller-identity --region "$REGION" >/dev/null 2>&1; then
    echo "AWS 認証情報が無効です。aws sso login または認証情報の設定を確認してください。" >&2
    exit 1
  fi

  local service="${1:-}"
  [[ $# -gt 0 ]] && shift || true

  case "$service" in
    ""|-h|--help|help) usage; exit 0 ;;
    status) show_status; exit 0 ;;
  esac

  # 既定は直近30分。呼び出し側が --since を渡していればそちらを優先する。
  local since_args=()
  if [[ ! " $* " =~ " --since " ]]; then
    since_args=(--since 30m)
  fi

  if [[ "$service" == "all" ]]; then
    for svc in backend frontend rag chroma; do
      echo "===== $svc ====="
      aws logs tail "$(log_group_for "$svc")" --region "$REGION" \
        --format short ${since_args[@]+"${since_args[@]}"} "$@" || true
    done
    exit 0
  fi

  local group
  if ! group="$(log_group_for "$service")"; then
    echo "不明なサービス: $service" >&2
    usage >&2
    exit 1
  fi

  aws logs tail "$group" --region "$REGION" --format short ${since_args[@]+"${since_args[@]}"} "$@"
}

main "$@"
