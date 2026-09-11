#!/usr/bin/env bash

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
# Codex CLI はここに置いた *.md を `/名前` のカスタムコマンドとして読み込む
CODEX_PROMPT_DIR="${CODEX_HOME:-$HOME/.codex}/prompts"

# 使用するAI CLI。codex を指定すると同じプロンプトを Codex 側で実行する
AI_CLI="${COPILOT_SHORTCUTS_CLI:-copilot}"

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

Codex で実行する:
  COPILOT_SHORTCUTS_CLI=codex cimpl "123"
  ./scripts/copilot-shortcuts.sh --codex implement "123"

Codex のカスタムコマンド (/issue, /implement, ...) として登録する:
  ./scripts/copilot-shortcuts.sh install-codex
EOF
}

run_copilot_prompt() {
  local prompt_file="$1"
  local header="$2"
  local task="$3"

  if [[ ! -f "$PROMPT_DIR/$prompt_file" ]]; then
    echo "Prompt file not found: $PROMPT_DIR/$prompt_file" >&2
    return 1
  fi

  local prompt
  prompt="$(cat "$PROMPT_DIR/$prompt_file")

$header:
$task"

  case "$AI_CLI" in
    codex)
      if ! command -v codex >/dev/null 2>&1; then
        echo "codex command not found. Install Codex CLI first." >&2
        return 1
      fi
      # copilot -p と同じく非対話で実行する
      codex exec "$prompt"
      ;;
    copilot)
      if ! command -v copilot >/dev/null 2>&1; then
        echo "copilot command not found. Install GitHub Copilot CLI first." >&2
        return 1
      fi
      copilot -p "$prompt"
      ;;
    *)
      echo "Unknown AI CLI: $AI_CLI (expected 'copilot' or 'codex')" >&2
      return 1
      ;;
  esac
}

# .github/prompts/*.prompt.md を Codex のカスタムコマンドとして ~/.codex/prompts へ配置する。
# 配置後は Codex 内で /issue や /implement のように呼び出せる。
#
# ~/.codex/prompts は他プロジェクトと共有される個人設定なので、
# 既存ファイルは既定で上書きしない（手を入れたコマンドを黙って壊さないため）。
# 上書きしたい場合のみ --force を渡す。
install_codex_prompts() {
  local force=0
  [[ "${1:-}" == "--force" || "${1:-}" == "-f" ]] && force=1

  if [[ ! -d "$PROMPT_DIR" ]]; then
    echo "Prompt directory not found: $PROMPT_DIR" >&2
    return 1
  fi

  mkdir -p "$CODEX_PROMPT_DIR" || return 1

  local installed=0 skipped=0 found=0
  local prompt_file name target
  for prompt_file in "$PROMPT_DIR"/*.prompt.md; do
    [[ -e "$prompt_file" ]] || continue
    found=$((found + 1))
    name="$(basename "$prompt_file")"
    name="${name%.prompt.md}"
    target="$CODEX_PROMPT_DIR/$name.md"

    if [[ -e "$target" && "$force" -eq 0 ]]; then
      if cmp -s "$prompt_file" "$target"; then
        echo "up-to-date /$name"
      else
        echo "skipped /$name (既存: $target — 上書きするには --force)"
        skipped=$((skipped + 1))
      fi
      continue
    fi

    if ! cp "$prompt_file" "$target"; then
      echo "Failed to install: $target" >&2
      return 1
    fi
    echo "installed /$name -> $target"
    installed=$((installed + 1))
  done

  if [[ "$found" -eq 0 ]]; then
    echo "No *.prompt.md found in $PROMPT_DIR" >&2
    return 1
  fi
  echo "installed=$installed skipped=$skipped (Codex で /名前 として使えます)"
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

copilot_shortcuts_main() {
  # --codex / --copilot は先頭で受け取り、以降のサブコマンドの実行先を切り替える
  case "$1" in
    --codex)
      AI_CLI="codex"
      shift
      ;;
    --copilot)
      AI_CLI="copilot"
      shift
      ;;
  esac

  local subcommand="$1"
  if [[ $# -gt 0 ]]; then
    shift
  fi

  case "$subcommand" in
    install-codex) install_codex_prompts "$@" ;;
    issue) cissue "$*" ;;
    implement) cimpl "$*" ;;
    pr) cpr "$*" ;;
    plan) cplan "$*" ;;
    review) creview "$*" ;;
    "" | -h | --help | help)
      copilot_shortcuts_usage
      ;;
    *)
      echo "Unknown subcommand: $subcommand" >&2
      copilot_shortcuts_usage >&2
      return 1
      ;;
  esac
}

is_sourced=0
if [[ -n "${ZSH_VERSION:-}" ]]; then
  [[ "${ZSH_EVAL_CONTEXT:-}" == *:file* ]] && is_sourced=1
elif [[ -n "${BASH_VERSION:-}" ]]; then
  [[ "${BASH_SOURCE[0]}" != "$0" ]] && is_sourced=1
fi

if [[ "$is_sourced" -eq 0 ]]; then
  copilot_shortcuts_main "$@"
fi
