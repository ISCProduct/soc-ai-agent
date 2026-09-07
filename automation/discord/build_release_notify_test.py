#!/usr/bin/env python3
"""build_release_notify.py の自己チェック。"""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from build_release_notify import (  # noqa: E402
    build_content,
    extract_feature_lines,
    parse_merge_pr_number,
    resolve_headline,
)


def main() -> None:
    assert parse_merge_pr_number("Merge pull request #1173 from ISCProduct/release") == 1173
    assert parse_merge_pr_number("feat: something") is None

    assert (
        resolve_headline(
            "Merge pull request #1173 from ISCProduct/release",
            "Release to production: 2026-09-07 レート制限回避の修正",
        )
        == "Release to production: 2026-09-07 レート制限回避の修正"
    )
    assert (
        resolve_headline("Merge pull request #1 from x/y", None)
        == "Merge pull request #1 from x/y"
    )

    body = """
## 含まれる変更
| PR | 内容 |
|---|---|
| **#1162** | **X-Forwarded-For詐称によるIPレート制限の回避を防ぐ** |
| **#1153** | 本番インフラの構造的問題4件を解消 |
"""
    lines = extract_feature_lines(body, limit=8)
    assert any("#1162" in line and "レート制限" in line for line in lines), lines
    assert any("#1153" in line for line in lines), lines

    content = build_content(
        headline="Release to production: demo",
        feature_lines=lines,
        commit_url="https://example.com/c",
        run_url="https://example.com/r",
        actor="oohasikazuyuki",
        pr_url="https://example.com/p",
    )
    assert "本番リリース速報" in content
    assert "Release to production: demo" in content
    assert "Merge pull request" not in content
    assert "PR: https://example.com/p" in content
    print("ok")


if __name__ == "__main__":
    main()
