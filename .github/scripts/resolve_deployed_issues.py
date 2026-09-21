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


def resolve_issues(prs, body_of, commits_of):
    """本番へ入ったPR番号の一覧から、閉じるべき issue -> 根拠PR を解決する。

    body_of(pr) は PR 本文、commits_of(pr) は PR が含むコミット題名の一覧を返す。

    本文に close キーワードが無いPRは1段だけ掘る。
    release→main が squash マージされると、デプロイ範囲(HEAD~1..HEAD)に現れるのは
    リリースPR1件だけになる。リリースPRの本文に issue 番号は書かれないため、
    ここで止まると「閉じる対象の issue はありません」で毎回終わり、
    本番反映済みの issue が開いたまま溜まる(実際に12件溜まった)。
    リリースPRのコミット題名には各 feature PR の番号が (#1234) で残っているので、
    そこから feature PR の本文へ辿る。

    掘るのは1段だけ。リリースPRが過去のリリースPRを含むことがあり、
    無制限に辿ると範囲が際限なく広がる。
    """
    issues: dict[int, list[int]] = {}

    def record(issue: int, pr: int) -> None:
        refs = issues.setdefault(issue, [])
        if pr not in refs:
            refs.append(pr)

    for pr in prs:
        found = extract_issue_numbers(body_of(pr))
        if found:
            for i in found:
                record(i, pr)
            continue
        for sub in extract_pr_numbers(commits_of(pr)):
            if sub == pr:
                continue
            for i in extract_issue_numbers(body_of(sub)):
                record(i, sub)

    return issues


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

    # --- resolve_issues: リリースPRを1段掘る ---
    # 実際に起きた形: release→main が squash され、範囲にはリリースPR1件だけが残る。
    bodies = {
        1421: "## 中身\n\n| #1401 | リファクタ |\n",   # リリースPR。close キーワード無し
        1401: "Closes #1300",
        1413: "Closes #1404\nCloses #1405",
        1414: "close キーワードの無いPR",
    }
    commits = {1421: ["refactor: 分離 (#1401)", "fix: 直す (#1413)", "fix(ci): (#1414)"]}
    got = resolve_issues([1421], lambda n: bodies.get(n, ""), lambda n: commits.get(n, []))
    assert got == {1300: [1401], 1404: [1413], 1405: [1413]}, f"リリースPRを掘れていない: {got}"

    # 本文に close キーワードがあるPRは掘らない（feature PR が直接 main に入る場合）
    got = resolve_issues([1413], lambda n: bodies.get(n, ""), lambda n: commits.get(n, []))
    assert got == {1404: [1413], 1405: [1413]}, got

    # 掘るのは1段だけ。孫は辿らない（過去のリリースPRで範囲が際限なく広がる）
    nested_bodies = {90: "", 91: "", 92: "Closes #7"}
    nested_commits = {90: ["release: 前回 (#91)"], 91: ["fix: 何か (#92)"]}
    got = resolve_issues([90], lambda n: nested_bodies.get(n, ""), lambda n: nested_commits.get(n, []))
    assert got == {}, f"2段目まで辿っている: {got}"

    # 自分自身を含むコミット題名（squash後の題名）で無限に戻らない
    got = resolve_issues([55], lambda n: "", lambda n: ["release: x (#55)"])
    assert got == {}, got

    # 同じ issue を複数PRが閉じる場合は根拠を重複なく並べる
    dup_bodies = {10: "", 11: "Closes #1", 12: "Closes #1"}
    got = resolve_issues([10], lambda n: dup_bodies.get(n, ""), lambda n: ["a (#11)", "b (#12)"])
    assert got == {1: [11, 12]}, got

    print("OK")
