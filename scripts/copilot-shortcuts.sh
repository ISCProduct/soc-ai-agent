#!/usr/bin/env bash

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROMPT_DIR="$PROJECT_ROOT/.copilot/prompts"

cplan() {
  local task="$*"
  copilot -p "$(cat "$PROMPT_DIR/plan.md")

対象タスク:
$task"
}

creview() {
  local task="${*:-現在の変更内容をレビューしてください。}"
  copilot -p "$(cat "$PROMPT_DIR/pr-review.md")

レビュー対象:
$task"
}

cissue() {
  local task="$*"
  copilot -p "$(cat "$PROMPT_DIR/issue.md")

Issue化したい内容:
$task"
}

cimpl() {
  local task="$*"
  copilot -p "$(cat "$PROMPT_DIR/implement.md")

実装したい内容:
$task"
}