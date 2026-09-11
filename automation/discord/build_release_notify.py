#!/usr/bin/env python3
"""本番リリース Discord 通知の本文を組み立てる。

merge commit の件名（Merge pull request #N ...）ではなく、
マージされた PR のタイトル（例: Release to production: ...）を見出しにする。
"""

from __future__ import annotations

import json
import re
import urllib.error
import urllib.request


MERGE_PR_RE = re.compile(r"Merge pull request #(\d+)\b", re.IGNORECASE)
# | **#1162** | **説明** | 形式の表行から機能概要を拾う
TABLE_ROW_RE = re.compile(
    r"\|\s*\*\*?#(\d+)\*\*?\s*\|\s*\*?\*?(.+?)\*?\*?\s*\|",
)


def resolve_headline(commit_subject: str, pr_title: str | None) -> str:
    """コミット件名と PR タイトルから通知見出しを決める。"""
    title = (pr_title or "").strip()
    if title:
        return title
    return (commit_subject or "").strip() or "(no subject)"


def extract_feature_lines(pr_body: str | None, limit: int = 8) -> list[str]:
    """PR本文から機能一覧の短い行を抽出する（Discord埋め込み用）。"""
    if not pr_body:
        return []
    lines: list[str] = []
    for match in TABLE_ROW_RE.finditer(pr_body):
        num, desc = match.group(1), match.group(2).strip()
        # ヘッダー行（PR | 内容）はスキップ
        if num.lower() == "pr" or desc in ("内容", "概要"):
            continue
        # Markdown強調を軽く落とす
        desc = re.sub(r"\*+", "", desc).strip()
        if not desc:
            continue
        lines.append(f"• #{num} {desc}")
        if len(lines) >= limit:
            break
    return lines


def build_content(
    *,
    headline: str,
    feature_lines: list[str],
    commit_url: str,
    run_url: str,
    actor: str,
    pr_url: str | None = None,
) -> str:
    parts = ["🚀 **本番リリース速報**", headline]
    if feature_lines:
        parts.append("")
        parts.extend(feature_lines)
    parts.append("")
    if pr_url:
        parts.append(f"PR: {pr_url}")
    parts.append(f"Commit: {commit_url}")
    parts.append(f"Run: {run_url}")
    parts.append(f"By: {actor}")
    return "\n".join(parts)


def parse_merge_pr_number(commit_subject: str) -> int | None:
    match = MERGE_PR_RE.search(commit_subject or "")
    if not match:
        return None
    return int(match.group(1))


def fetch_pr(repo: str, number: int, token: str) -> dict:
    url = f"https://api.github.com/repos/{repo}/pulls/{number}"
    req = urllib.request.Request(
        url,
        headers={
            "Accept": "application/vnd.github+json",
            "Authorization": f"Bearer {token}",
            "X-GitHub-Api-Version": "2022-11-28",
            "User-Agent": "soc-ai-agent-release-notify",
        },
    )
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read().decode())


def main() -> None:
    import argparse
    import os
    import sys

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--commit-subject", required=True)
    parser.add_argument("--commit-url", required=True)
    parser.add_argument("--run-url", required=True)
    parser.add_argument("--actor", required=True)
    parser.add_argument("--repo", default=os.environ.get("GITHUB_REPOSITORY", ""))
    parser.add_argument("--token", default=os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN", ""))
    args = parser.parse_args()

    pr_title = None
    pr_body = None
    pr_url = None
    pr_number = parse_merge_pr_number(args.commit_subject)
    if pr_number and args.repo and args.token:
        try:
            pr = fetch_pr(args.repo, pr_number, args.token)
            pr_title = pr.get("title")
            pr_body = pr.get("body")
            pr_url = pr.get("html_url")
        except (urllib.error.URLError, urllib.error.HTTPError, TimeoutError, json.JSONDecodeError) as exc:
            print(f"warn: failed to fetch PR #{pr_number}: {exc}", file=sys.stderr)

    headline = resolve_headline(args.commit_subject, pr_title)
    features = extract_feature_lines(pr_body)
    content = build_content(
        headline=headline,
        feature_lines=features,
        commit_url=args.commit_url,
        run_url=args.run_url,
        actor=args.actor,
        pr_url=pr_url,
    )
    print(content)


if __name__ == "__main__":
    main()
