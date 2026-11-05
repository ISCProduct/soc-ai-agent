#!/usr/bin/env python3
"""Backlog のステータスと GitHub の PR の実態がズレている課題・PR を洗い出す。

なぜ必要か:
  pr-to-backlog.yml は課題キーが解決できないと `::warning::` を残して success で
  終わる。Actions の警告は誰も読まないので、沈黙と同じになる。実際に 2026-11-05 の
  時点で、直近92回の同期実行のうち35回が「キーが見つかりません」で何もしておらず、
  SOCAIAGENT-474 は PR が開いているのに「処理中」で止まっていた。
  手で突き合わせないと気づけない状態だったので、突き合わせを機械にやらせる。

紐づけの判定に PR タイトルのキー解決をもう一度書かない。
pr-to-backlog.yml が成功したときに課題の説明欄へ `## GitHub PR` として PR の URL を
書いているので、**それを逆引き索引として使う**。索引に無い PR は「同期が効かなかった PR」
そのものなので、検知したいものと一致する。
"""

from __future__ import annotations

import json
import os
import re
import sys
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from backlog_client import bl_request, load_backlog_env, resolve_bl_base  # noqa: E402

# ステータスの前後。pr-to-backlog.yml の MERGE_STATUS_ORDER と同じ並びにする。
STATUS_ORDER = (
    "未対応",
    "保留",
    "処理中",
    "PR確認待ち",
    "処理済み",
    "ステージング環境反映",
    "リリース待ち",
    "完了",
)

# マージ先ブランチごとに、少なくともここまで進んでいるべきというステータス。
MERGED_MIN_STATUS = {
    "develop": "ステージング環境反映",
    "release": "リリース待ち",
    "main": "完了",
}

# 「完了」も含めて全ステータスを取る。完了済みの課題に追いPRが出るのは正常だが、
# 索引から外すとその PR が「どの課題にも紐づいていない」と誤検知される
# （#1821 は完了済みの SOCAIAGENT-347 の追いPR）。
# ステータスの前後比較で弾かれるので、含めても完了の課題が指摘に出ることはない。
AUDIT_STATUS_IDS = (1, 2, 416375, 416451, 3, 437943, 437944, 4)

PR_URL_RE = re.compile(r"https://github\.com/[^/\s]+/[^/\s]+/pull/(\d+)")

# Discord の 1 通は 2000 文字。余裕を見て切る。
MESSAGE_CHARS = 1700


def rank(status_name: str) -> int:
    """ステータスの位置。知らない名前は -1（比較しない）。"""
    try:
        return STATUS_ORDER.index((status_name or "").strip())
    except ValueError:
        return -1


def linked_pr_numbers(description: str) -> list[int]:
    """説明欄に書かれた PR 番号。重複は除き、書かれた順に返す。"""
    nums: list[int] = []
    for m in PR_URL_RE.finditer(description or ""):
        n = int(m.group(1))
        if n not in nums:
            nums.append(n)
    return nums


def find_mismatches(issues: list[dict], prs: list[dict]) -> list[str]:
    """ズレの一覧を人が読む1行ずつにして返す。ズレが無ければ空。

    issues: Backlog の課題（issueKey / status.name / description）
    prs:    GitHub の PR（number / draft / state / base / mergedAt / title）
    どちらも API の形をそのまま受ける。テストから素の dict を渡せるようにしている。
    """
    by_pr: dict[int, list[dict]] = {}
    for issue in issues:
        for n in linked_pr_numbers(issue.get("description") or ""):
            by_pr.setdefault(n, []).append(issue)

    pr_by_number = {int(p["number"]): p for p in prs}
    lines: list[str] = []

    for num in sorted(pr_by_number, reverse=True):
        pr = pr_by_number[num]
        title = (pr.get("title") or "").strip()
        linked = by_pr.get(num, [])
        is_open = pr.get("state") == "open"

        # 1. キーが解決できず、どの課題にも紐づいていない PR。
        #    ドラフトは作業中なので、まだ書いていなくてもおかしくない。
        if not linked:
            if is_open and not pr.get("draft"):
                lines.append(f"- #{num} どの課題にも紐づいていません（{title[:50]}）")
            continue

        for issue in linked:
            key = issue["issueKey"]
            name = (issue.get("status") or {}).get("name", "")
            cur = rank(name)
            if cur < 0:
                continue

            # 2. レビューを待っている PR なのに、課題が PR確認待ち より手前。
            if is_open and not pr.get("draft") and cur < rank("PR確認待ち"):
                lines.append(f"- {key} は「{name}」ですが #{num} はレビュー待ちです")

            # 3. マージ済みなのに課題が進んでいない。
            if pr.get("mergedAt"):
                want = MERGED_MIN_STATUS.get(pr.get("base") or "")
                if want and cur < rank(want):
                    lines.append(
                        f"- {key} は「{name}」ですが #{num} は {pr['base']} にマージ済みです"
                        f"（「{want}」以降のはず）"
                    )

    # 4. PR確認待ち なのに、見られる PR が無い課題。
    for issue in issues:
        name = (issue.get("status") or {}).get("name", "")
        if name != "PR確認待ち":
            continue
        key = issue["issueKey"]
        nums = linked_pr_numbers(issue.get("description") or "")
        open_prs = [n for n in nums if (pr_by_number.get(n) or {}).get("state") == "open"]
        if open_prs:
            continue
        if not nums:
            lines.append(f"- {key} は「PR確認待ち」ですが PR が紐づいていません")
        else:
            closed = "/".join(f"#{n}" for n in nums)
            lines.append(f"- {key} は「PR確認待ち」ですが {closed} は閉じています")

    # 同じ行が重複しうる（1つのPRに複数課題が紐づく等）ので、順序を保って一意にする。
    seen = set()
    uniq = []
    for line in lines:
        if line not in seen:
            seen.add(line)
            uniq.append(line)
    return uniq


def build_message(lines: list[str], repo: str) -> str:
    """Discord へ投げる本文。長すぎるときは件数を残して切る。"""
    head = (
        f"⚠️ **Backlog と GitHub のズレ {len(lines)} 件**\n"
        f"（{repo} / PR のタイトルか本文に `SOCAIAGENT-N` を書くと自動で同期されます）\n"
    )
    body = ""
    shown = 0
    for line in lines:
        if len(head) + len(body) + len(line) + 1 > MESSAGE_CHARS:
            break
        body += line + "\n"
        shown += 1
    if shown < len(lines):
        body += f"- ほか {len(lines) - shown} 件\n"
    return (head + body).rstrip()


def fetch_issues(base: str, api_key: str, project_key: str) -> list[dict]:
    project = bl_request(base, api_key, "GET", f"/projects/{project_key}")
    issues: list[dict] = []
    offset = 0
    while True:  # 101件目以降を黙って落とさない
        page = bl_request(
            base,
            api_key,
            "GET",
            "/issues",
            {
                "projectId[]": [project["id"]],
                "statusId[]": list(AUDIT_STATUS_IDS),
                "count": 100,
                "offset": offset,
            },
        )
        issues += page or []
        if not page or len(page) < 100:
            return issues
        offset += 100


def gh_prs(repo: str, token: str, merged_days: int) -> list[dict]:
    """オープンな PR と、直近 merged_days 日にマージされた PR。

    古いマージまで見ない。直してもらえない過去のズレを毎日並べると読まれなくなる。
    """
    import datetime

    since = (
        datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(days=merged_days)
    ).isoformat()
    out = []
    for state in ("open", "closed"):
        page = 1
        while True:
            url = (
                f"https://api.github.com/repos/{repo}/pulls"
                f"?state={state}&per_page=100&page={page}&sort=updated&direction=desc"
            )
            req = urllib.request.Request(url)
            req.add_header("Authorization", f"Bearer {token}")
            req.add_header("Accept", "application/vnd.github+json")
            req.add_header("X-GitHub-Api-Version", "2022-11-28")
            with urllib.request.urlopen(req, timeout=30) as resp:
                prs = json.load(resp)
            if not prs:
                break
            stop = False
            for p in prs:
                merged_at = p.get("merged_at")
                if state == "closed":
                    if not merged_at:
                        continue
                    if merged_at < since:
                        stop = True
                        continue
                out.append(
                    {
                        "number": p["number"],
                        "title": p.get("title") or "",
                        "draft": bool(p.get("draft")),
                        "state": p.get("state"),
                        "base": (p.get("base") or {}).get("ref", ""),
                        "mergedAt": merged_at,
                    }
                )
            if stop or len(prs) < 100:
                break
            page += 1
    return out


def main(argv: list[str]) -> int:
    # 通知本文は引数で渡されたファイルへ書く。backlog_client が標準出力へ
    # 接続情報を出すので、本文を混ぜないため分けている。
    out_path = argv[1] if len(argv) > 1 else ""
    api_key, space_id, project_key, domain = load_backlog_env()
    base, _ = resolve_bl_base(space_id, api_key, domain)
    repo = os.environ.get("GH_REPO", "")
    token = os.environ.get("GH_TOKEN", "")
    if not repo or not token:
        print("GH_REPO / GH_TOKEN が未設定です。", file=sys.stderr)
        return 1
    merged_days = int(os.environ.get("MERGED_DAYS") or 7)

    issues = fetch_issues(base, api_key, project_key)
    prs = gh_prs(repo, token, merged_days)
    lines = find_mismatches(issues, prs)
    print(f"課題 {len(issues)} 件 / PR {len(prs)} 件を突き合わせ: ズレ {len(lines)} 件")
    for line in lines:
        print(line)
    # ズレが無ければ空ファイル。呼び出し側は空なら投稿しない。
    message = build_message(lines, repo) if lines else ""
    if out_path:
        with open(out_path, "w", encoding="utf-8") as f:
            f.write(message)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
