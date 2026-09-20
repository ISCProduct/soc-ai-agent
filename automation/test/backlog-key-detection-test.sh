#!/bin/bash
# pr-to-backlog.yml の「Backlog キーの解決元になる番号」の抽出をテストする。
#
# ここが黙って壊れると、同期は success のまま「キーが見つかりません」を出して
# 何もしない。失敗しないので気づけない（実際に8件中6件が取りこぼされていた）。
#
# ワークフロー内の正規表現と関数をそのまま抜き出して実行する。
# 抜き出せなければ「テストしたつもり」になるので必ず失敗させる。
#
# 使い方: ./automation/test/backlog-key-detection-test.sh

set -uo pipefail

WORKFLOW="$(dirname "$0")/../../.github/workflows/pr-to-backlog.yml"
FRAGMENT=$(mktemp)
trap 'rm -f "$FRAGMENT"' EXIT

# ISSUE_REF_RE の定義から candidate_issue_numbers の終わりまでを抜き出す
awk '/^ *ISSUE_REF_RE = re\.compile\(/,/^ *return nums$/' "$WORKFLOW" | sed 's/^          //' > "$FRAGMENT"

if ! grep -q "BRANCH_NUM_RE" "$FRAGMENT" || ! grep -q "def candidate_issue_numbers" "$FRAGMENT"; then
  echo "NG: 抽出に失敗しました。pr-to-backlog.yml の構造変更を確認してください。" >&2
  exit 1
fi

python3 - "$FRAGMENT" <<'PY'
import re, sys

src = open(sys.argv[1]).read()
ns = {"re": re}
exec(src, ns)
candidate_issue_numbers = ns["candidate_issue_numbers"]

# (ブランチ名, 本文, タイトル, 期待する番号)
cases = [
    # 本文に書いてある場合
    ("fix/1374-guest-promotion", "Fixes #1374", "feat(auth): ゲストの診断結果を引き継ぐ", ["1374"]),
    # Refs も拾う（このリポジトリで実際に使われている書き方）
    ("docs/1388-uptime-runbook", "Refs #1388", "docs(ops): 前日チェックリスト", ["1388"]),
    # 本文とブランチ名の両方。本文が先
    ("feature/619-observability", "Closes #1185", "feat: #619 Sentry導入", ["1185", "619"]),
    # 本文に何も書かれていなくてもブランチ名から拾う（書き忘れ対策の本命）
    ("fix/1374-guest-promotion", "", "feat(auth): ...", ["1374"]),
    ("feature/issue-521", "", "feat: ...", ["521"]),
    # 参照先が別PRでも候補には挙げる（呼び出し側がもう一段たどる）
    ("fix/1361-review-followup", "#1361 のレビュー指摘への対応", "fix(infra): ...", ["1361"]),
    # 番号を含まないブランチ名では何も拾わない
    ("chore/ops-doc-followup", "マージ済みの #1389 / #1390 のレビュー", "chore(ops): ...", []),
    ("fix/website-extract-comments", "", "docs: ...", []),
    ("release", "", "release: ...", []),
    # 「番号ではない数字」を誤って拾わない
    ("chore/repo-cleanup-phase1", "", "chore: ...", []),
    ("feat/v2-redesign", "", "feat: ...", []),
]

failures = 0
for head, body, title, want in cases:
    got = candidate_issue_numbers(title, body, head)
    if got != want:
        print(f"NG: {head} 本文={body!r} → {got}（期待 {want}）")
        failures += 1
    else:
        print(f"ok: {head} → {got or 'なし'}")

if failures:
    print(f"FAIL: {failures} 件")
    sys.exit(1)
print(f"PASS (キー解決元の抽出 {len(cases)} ケース)")
PY
