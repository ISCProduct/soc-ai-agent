#!/usr/bin/env python3
"""develop が進んだあと、開いている PR のマイグレーション版番号を最新と比べる。

PR の Test は、その PR を最後に更新したときの develop としか比べない。
別の PR が同じ版番号を先にマージしても、残った PR の検査は再実行されない
（SOCAIAGENT-503）。このスクリプトは develop への push から呼び、各 PR の
head に check run を付ける。PR 自体は更新しない。

fork の PR は、このリポジトリの commit に検査を付けられないので飛ばす。
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import urllib.error
import urllib.request
from datetime import datetime, timezone

CHECK_NAME = "マイグレーション版番号（develop 最新）"
CHECKER = "automation/test/migration-version-unique-test.sh"
SUMMARY_LIMIT = 6000


def conclusion_for(returncode: int) -> str:
    return "success" if returncode == 0 else "failure"


def readable(output: str) -> str:
    lines: list[str] = []
    for line in output.splitlines():
        text = line.strip()
        if text.startswith("::error::"):
            text = text[len("::error::") :]
        if text:
            lines.append(text)
    body = "\n".join(lines)
    if len(body) > SUMMARY_LIMIT:
        body = body[:SUMMARY_LIMIT] + "\n…（省略）"
    return body or "検査の出力が空でした"


def summary_for(returncode: int, output: str, develop_sha: str) -> str:
    head = f"比較した develop: `{develop_sha}`"
    body = readable(output)
    if returncode == 0:
        return head + "\n\n" + body
    hint = (
        "衝突しているファイルは上の行です。"
        " develop を取り込んだうえで `make migrate-new name=...` で採番し直してください。"
    )
    if "make migrate-new" in body:
        return head + "\n\n" + body
    return head + "\n\n" + body + "\n\n" + hint


def should_check(pr: dict, repo: str) -> bool:
    """同じリポジトリの、base が develop の PR だけ検査する。"""
    base = (pr.get("base") or {}).get("ref")
    head = pr.get("head") or {}
    head_repo = (head.get("repo") or {}).get("full_name")
    sha = head.get("sha") or ""
    return base == "develop" and head_repo == repo and bool(sha)


def next_link(link_header: str | None) -> str | None:
    if not link_header:
        return None
    for part in link_header.split(","):
        section = part.strip()
        if 'rel="next"' not in section:
            continue
        start = section.find("<")
        end = section.find(">")
        if start >= 0 and end > start:
            return section[start + 1 : end]
    return None


def github_json(url: str, token: str, method: str = "GET", payload: dict | None = None):
    data = None if payload is None else json.dumps(payload).encode()
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("X-GitHub-Api-Version", "2022-11-28")
    req.add_header("User-Agent", "soc-migration-vs-develop")
    if payload is not None:
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=60) as response:
            raw = response.read()
            body = json.loads(raw) if raw else None
            return response.headers.get("Link"), body
    except urllib.error.HTTPError as exc:
        err = exc.read().decode(errors="replace")
        raise RuntimeError(f"GitHub API {exc.code} {method} {url}: {err}") from exc


def list_open_prs(repo: str, token: str) -> list[dict]:
    url = f"https://api.github.com/repos/{repo}/pulls?state=open&base=develop&per_page=100"
    found: list[dict] = []
    while url:
        link, batch = github_json(url, token)
        if not isinstance(batch, list):
            raise RuntimeError("PR 一覧の応答が配列ではありません")
        found.extend(batch)
        url = next_link(link) or ""
    return found


def post_check(repo: str, token: str, sha: str, conclusion: str, summary: str, details_url: str) -> None:
    title = (
        "develop の最新版番号と衝突していません"
        if conclusion == "success"
        else "develop の最新版番号と衝突しています"
    )
    now = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    payload = {
        "name": CHECK_NAME,
        "head_sha": sha,
        "status": "completed",
        "conclusion": conclusion,
        "completed_at": now,
        "details_url": details_url,
        "output": {"title": title, "summary": summary[:65000]},
    }
    github_json(
        f"https://api.github.com/repos/{repo}/check-runs",
        token,
        method="POST",
        payload=payload,
    )


def run_checker(repo_dir: str, develop_sha: str, number: int) -> tuple[int, str]:
    """PR head を worktree に取り出して、既存の版番号検査を BASE_REF 付きで走らせる。"""
    ref = f"refs/heads/migration-check-pr-{number}"
    worktree = tempfile.mkdtemp(prefix=f"migration-pr-{number}-")
    fetch = subprocess.run(
        ["git", "fetch", "--depth=1", "origin", f"pull/{number}/head:{ref}"],
        cwd=repo_dir,
        capture_output=True,
        text=True,
    )
    if fetch.returncode != 0:
        return 1, fetch.stderr or fetch.stdout or "PR の取得に失敗しました"
    add = subprocess.run(
        ["git", "worktree", "add", "--detach", worktree, ref],
        cwd=repo_dir,
        capture_output=True,
        text=True,
    )
    if add.returncode != 0:
        subprocess.run(["git", "update-ref", "-d", ref], cwd=repo_dir, check=False)
        return 1, add.stderr or add.stdout or "worktree の作成に失敗しました"
    try:
        env = os.environ.copy()
        env["BASE_REF"] = develop_sha
        proc = subprocess.run(
            [os.path.join(repo_dir, CHECKER)],
            cwd=worktree,
            env=env,
            capture_output=True,
            text=True,
        )
        return proc.returncode, (proc.stdout or "") + (proc.stderr or "")
    finally:
        subprocess.run(["git", "worktree", "remove", "--force", worktree], cwd=repo_dir, check=False)
        subprocess.run(["git", "update-ref", "-d", ref], cwd=repo_dir, check=False)


def main() -> int:
    repo = os.environ.get("GITHUB_REPOSITORY", "")
    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN") or ""
    develop_sha = os.environ.get("GITHUB_SHA", "")
    repo_dir = os.environ.get("GITHUB_WORKSPACE") or os.getcwd()
    if not repo or not token or not develop_sha:
        print("GITHUB_REPOSITORY / GH_TOKEN / GITHUB_SHA がありません", file=sys.stderr)
        return 1
    pulls = list_open_prs(repo, token)
    targets = [pr for pr in pulls if should_check(pr, repo)]
    skipped = len(pulls) - len(targets)
    print(f"開いている PR: {len(pulls)} 件、検査する: {len(targets)} 件、対象外: {skipped} 件")
    failed_posts = 0
    for pr in targets:
        number = int(pr["number"])
        sha = pr["head"]["sha"]
        url = pr.get("html_url") or f"https://github.com/{repo}/pull/{number}"
        code, output = run_checker(repo_dir, develop_sha, number)
        conclusion = conclusion_for(code)
        summary = summary_for(code, output, develop_sha)
        print(f"PR #{number} {conclusion}")
        try:
            post_check(repo, token, sha, conclusion, summary, url)
        except RuntimeError as exc:
            failed_posts += 1
            print(exc, file=sys.stderr)
    if failed_posts:
        print(f"check run の作成に失敗: {failed_posts} 件", file=sys.stderr)
        return 1
    return 0


def self_check() -> None:
    assert conclusion_for(0) == "success"
    assert conclusion_for(1) == "failure"
    output = (
        "::error::版番号 000054 は origin/develop で既に使われています\n"
        "    origin/develop:   000054_first\n"
        "    このブランチ: 000054_second\n"
    )
    summary = summary_for(1, output, "abc123")
    for needle in ("000054", "000054_first", "000054_second", "make migrate-new", "abc123"):
        assert needle in summary, needle
    ok = summary_for(0, "✓ マイグレーション 3 件: 版番号の重複なし", "abc123")
    assert "make migrate-new" not in ok
    assert "重複なし" in ok

    repo = "ISCProduct/soc-ai-agent"
    pulls = [
        {
            "number": 1,
            "base": {"ref": "develop"},
            "head": {"sha": "a", "repo": {"full_name": repo}},
        },
        {
            "number": 2,
            "base": {"ref": "develop"},
            "head": {"sha": "b", "repo": {"full_name": "someone/fork"}},
        },
        {
            "number": 3,
            "base": {"ref": "main"},
            "head": {"sha": "c", "repo": {"full_name": repo}},
        },
        {
            "number": 4,
            "base": {"ref": "develop"},
            "head": {"sha": "", "repo": {"full_name": repo}},
        },
    ]
    chosen = [pr["number"] for pr in pulls if should_check(pr, repo)]
    assert chosen == [1], chosen
    link = '<https://example.test/next>; rel="next", <https://example.test/last>; rel="last"'
    assert next_link(link) == "https://example.test/next"
    assert next_link(None) is None
    long_line = "x" * (SUMMARY_LIMIT + 50)
    clipped = readable(long_line)
    assert clipped.endswith("…（省略）")
    assert len(clipped) < SUMMARY_LIMIT + 20
    print("ok")


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--self-check":
        self_check()
    else:
        sys.exit(main())
