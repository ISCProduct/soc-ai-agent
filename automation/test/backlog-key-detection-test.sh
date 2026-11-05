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
# extract_bl_key（PRタイトル・本文から直接キーを読む側）も同じ要領で抜き出す
awk '/^ *def extract_bl_key\(/,/^ *return None$/' "$WORKFLOW" | sed 's/^          //' >> "$FRAGMENT"

if ! grep -q "BRANCH_NUM_RE" "$FRAGMENT" || ! grep -q "def candidate_issue_numbers" "$FRAGMENT" \
  || ! grep -q "def extract_bl_key" "$FRAGMENT"; then
  echo "NG: 抽出に失敗しました。pr-to-backlog.yml の構造変更を確認してください。" >&2
  exit 1
fi

python3 - "$FRAGMENT" <<'PY'
import re, sys

src = open(sys.argv[1]).read()
ns = {"re": re, "PROJ_KEY": "SOCAIAGENT"}
exec(src, ns)
candidate_issue_numbers = ns["candidate_issue_numbers"]
extract_bl_key = ns["extract_bl_key"]

# (ブランチ名, 本文, タイトル, 期待する番号)
cases = [
    # 本文に書いてある場合
    ("fix/1374-guest-promotion", "Fixes #1374", "feat(auth): ゲストの診断結果を引き継ぐ", ["1374"]),
    # Refs も拾う（このリポジトリで実際に使われている書き方）
    ("docs/1388-uptime-runbook", "Refs #1388", "docs(ops): 前日チェックリスト", ["1388"]),
    # 「Issue #N のレビュー指摘」という書き方も拾う。キーワードを絞っていたため
    # #1970 が紐づかず、ブランチ名にも番号が無くて同期が落ちていた。
    ("fix/tech-stack-company-name-untrusted", "Issue #1636 のレビューでいただいた指摘への対応です。",
     "fix(company): 技術スタック抽出プロンプトの企業名を囲む", ["1636"]),
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

# (タイトル, 本文, 期待するキー)
#
# タイトルのキーは囲みの種類を見ない。角括弧だけを見ていたため、丸括弧で
# 書かれた #1974 が同期されず SOCAIAGENT-474 が「処理中」で止まっていた。
key_cases = [
    ("[SOCAIAGENT-474] feat(scout): 氏名を出さない", "", "SOCAIAGENT-474"),
    ("feat(scout): 氏名を出さない (SOCAIAGENT-474)", "", "SOCAIAGENT-474"),
    ("feat(scout): 氏名を出さない SOCAIAGENT-474", "", "SOCAIAGENT-474"),
    ("docs(wiki): 方針を整理する (SOCAIAGENT-328 / #1817)", "", "SOCAIAGENT-328"),
    # 日本語が直後に続く裸書き。\b では末尾が境界にならず拾えなかった
    ("SOCAIAGENT-474の氏名表示を直す", "", "SOCAIAGENT-474"),
    ("対応: SOCAIAGENT-474、氏名を出さない", "", "SOCAIAGENT-474"),
    # 番号の途中で切らない
    ("feat: SOCAIAGENT-4741 を直す", "", "SOCAIAGENT-4741"),
    # タイトルに無ければ本文のマーカー・記述を見る
    ("feat: 何か", "<!-- backlog-key:SOCAIAGENT-12 -->", "SOCAIAGENT-12"),
    ("feat: 何か", "Backlog: SOCAIAGENT-34", "SOCAIAGENT-34"),
    # 他プロジェクトのキーや番号なしは拾わない
    ("[OTHER-1] feat: 何か", "", None),
    ("feat: #1770 スコア計算版の記録と再計算", "", None),
]

for title, body, want in key_cases:
    got = extract_bl_key(title, body)
    if got != want:
        print(f"NG: {title!r} 本文={body!r} → {got}（期待 {want}）")
        failures += 1
    else:
        print(f"ok: {title!r} → {got or 'なし'}")

if failures:
    print(f"FAIL: {failures} 件")
    sys.exit(1)
print(f"PASS (キー解決元の抽出 {len(cases)} ケース / キー直読み {len(key_cases)} ケース)")
PY
