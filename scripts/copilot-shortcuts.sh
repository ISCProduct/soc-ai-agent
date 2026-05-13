#!/usr/bin/env bash

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROMPT_DIR="$PROJECT_ROOT/.github/prompts"

get_script_path() {
  if [[ -n "${BASH_SOURCE[0]:-}" ]]; then
    printf '%s\n' "${BASH_SOURCE[0]}"
  elif [[ -n "${ZSH_VERSION:-}" ]]; then
    printf '%s\n' "${(%):-%N}"
  else
    printf '%s\n' "$0"
  fi
}

SCRIPT_PATH="$(get_script_path)"
PROJECT_ROOT="$(cd "$(dirname "$SCRIPT_PATH")/.." && pwd)"
PROMPT_DIR="$PROJECT_ROOT/.github/prompts"

copilot_shortcuts_usage() {
  cat <<'EOF'
Usage:
  source scripts/copilot-shortcuts.sh
  cissue "..."
  cimpl "123"
  cpr "123"

Or run directly:
  ./scripts/copilot-shortcuts.sh issue "..."
  ./scripts/copilot-shortcuts.sh implement "123"
  ./scripts/copilot-shortcuts.sh pr "123"
EOF
}

run_copilot_prompt() {
  local prompt_file="$1"
  local header="$2"
  local task="$3"

  if ! command -v copilot >/dev/null 2>&1; then
    echo "copilot command not found. Install GitHub Copilot CLI first." >&2
    return 1
  fi

  if [[ ! -f "$PROMPT_DIR/$prompt_file" ]]; then
    echo "Prompt file not found: $PROMPT_DIR/$prompt_file" >&2
    return 1
  fi

  copilot -p "$(cat "$PROMPT_DIR/$prompt_file")

$header:
$task"

Or run directly:
  ./scripts/copilot-shortcuts.sh issue "..."
  ./scripts/copilot-shortcuts.sh implement "123"
  ./scripts/copilot-shortcuts.sh pr "123"
EOF
}

run_copilot_prompt() {
  local prompt_file="$1"
  local header="$2"
  local task="$3"

  if ! command -v copilot >/dev/null 2>&1; then
    echo "copilot command not found. Install GitHub Copilot CLI first." >&2
    return 1
  fi

  if [[ ! -f "$PROMPT_DIR/$prompt_file" ]]; then
    echo "Prompt file not found: $PROMPT_DIR/$prompt_file" >&2
    return 1
  fi

  copilot -p "$(cat "$PROMPT_DIR/$prompt_file")

$header:
$task"
}

cplan() {
  local task="${*:-対象タスクを入力してください。}"
  run_copilot_prompt "plan.prompt.md" "対象タスク" "$task"
}

creview() {
  local task="${*:-現在の変更内容をレビューしてください。}"
  run_copilot_prompt "pr-review.prompt.md" "レビュー対象" "$task"
}

cissue() {
  local task="$*"
  run_copilot_prompt "issue.prompt.md" "Issue化したい内容" "$task"
}

cimpl() {
  local task="$*"
  run_copilot_prompt "implement.prompt.md" "実装したい内容" "$task"
}

cpr() {
  local task="$*"
  run_copilot_prompt "pr.prompt.md" "PR作成したいIssue番号" "$task"
}

cplan() {
  local task="${*:-対象タスクを入力してください。}"
  run_copilot_prompt "plan.prompt.md" "対象タスク" "$task"
}

creview() {
  local task="${*:-現在の変更内容をレビューしてください。}"
  run_copilot_prompt "pr-review.prompt.md" "レビュー対象" "$task"
}

copilot_shortcuts_main() {
  local subcommand="$1"
  shift || true
copilot_shortcuts_main() {
  local subcommand="$1"
  if [[ $# -gt 0 ]]; then
    shift
  fi

  case "$subcommand" in
    issue) cissue "$*" ;;
    implement) cimpl "$*" ;;
    pr) cpr "$*" ;;
    plan) cplan "$*" ;;
    review) creview "$*" ;;
    ""|-h|--help|help)
      copilot_shortcuts_usage
      ;;
    *)
      echo "Unknown subcommand: $subcommand" >&2
      copilot_shortcuts_usage >&2
      return 1
      ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
is_sourced=0
if [[ -n "${ZSH_VERSION:-}" ]]; then
  [[ "${ZSH_EVAL_CONTEXT:-}" == *:file* ]] && is_sourced=1
elif [[ -n "${BASH_VERSION:-}" ]]; then
  [[ "${BASH_SOURCE[0]}" != "$0" ]] && is_sourced=1
fi

if [[ "$is_sourced" -eq 0 ]]; then
  copilot_shortcuts_main "$@"
fi
fi
