#!/usr/bin/env python3
"""本番へ反映されたコミット範囲から、閉じるべき GitHub Issue 番号を解決する。

feature/* -> develop -> release -> main のフローでは、GitHub の
`Closes #123` は発火しない。close キーワードはデフォルトブランチ(main)へ
マージされた PR の本文でのみ効くが、feature PR の base は develop であり、
プロモーション PR の本文には issue 番号が書かれないため。

結果として、本番へ反映済みの issue が開いたまま溜まる
（2026-09-10 時点で前回リリース分を含め16件が該当）。

ここではコミットメッセージから解決する。squash マージのコミット題名には
元 PR 番号が `(#1234)` の形で残るので、その PR 本文の close キーワードを辿る。
"""
from __future__ import annotations

import re
import sys

# squash マージのコミット題名末尾に付く元PR番号: "feat: 〜 (#1234)"
PR_REF_PATTERN = re.compile(r"\(#(\d+)\)\s*$")

# GitHub が解釈する close キーワード（大文字小文字を区別しない）。
# 同じリポジトリの issue のみ対象にする。`owner/repo#12` 形式は拾わない。
CLOSE_PATTERN = re.compile(
    r"\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s*:?\s+#(\d+)\b",
    re.IGNORECASE,
)


def extract_pr_numbers(commit_subjects: list[str]) -> list[int]:
    """コミット題名の一覧から、元PR番号を重複なく順序を保って返す。"""
    seen: set[int] = set()
    out: list[int] = []
    for subject in commit_subjects:
        m = PR_REF_PATTERN.search(subject.strip())
        if not m:
            continue
        num = int(m.group(1))
        if num not in seen:
            seen.add(num)
            out.append(num)
    return out


def extract_issue_numbers(pr_body: str) -> list[int]:
    """PR本文から close 対象の issue 番号を重複なく順序を保って返す。"""
    if not pr_body:
        return []
    seen: set[int] = set()
    out: list[int] = []
    for m in CLOSE_PATTERN.finditer(pr_body):
        num = int(m.group(1))
        if num not in seen:
            seen.add(num)
            out.append(num)
    return out


if __name__ == "__main__":
    # 自己チェック: python3 .github/scripts/resolve_deployed_issues.py
    assert extract_pr_numbers(["feat: 何か (#1234)"]) == [1234]
    assert extract_pr_numbers(["fix: 直す (#12)", "feat: 足す (#34)"]) == [12, 34]
    assert extract_pr_numbers(["fix: 直す (#12)", "chore: 別 (#12)"]) == [12], "重複は1つ"
    assert extract_pr_numbers(["マージコミット"]) == []
    # 文中の #123 は拾わない（末尾のみ）
    assert extract_pr_numbers(["fix: #123 を直す"]) == []
    # Revert なども題名末尾の形式なら拾う
    assert extract_pr_numbers(['Revert "feat: 何か (#99)"']) == [], "引用符付きは末尾が ) でない"

    assert extract_issue_numbers("Closes #794") == [794]
    assert extract_issue_numbers("closes #794") == [794]
    assert extract_issue_numbers("Fixes #1") == [1]
    assert extract_issue_numbers("fixed #2") == [2]
    assert extract_issue_numbers("Resolves #3") == [3]
    assert extract_issue_numbers("resolved #4") == [4]
    assert extract_issue_numbers("Closes: #5") == [5], "コロン付きも GitHub は解釈する"
    assert extract_issue_numbers("Closes #6\nFixes #7") == [6, 7]
    assert extract_issue_numbers("Closes #8 Closes #8") == [8], "重複は1つ"
    # close キーワードが無い単なる参照は閉じない
    assert extract_issue_numbers("関連: #9") == []
    assert extract_issue_numbers("#10 を参照") == []
    assert extract_issue_numbers("") == []
    assert extract_issue_numbers("Closes ISCProduct/other#11") == [], "他リポジトリは対象外"
    # 単語境界: "Discloses #12" を拾わない
    assert extract_issue_numbers("Discloses #12") == []

    print("OK")
