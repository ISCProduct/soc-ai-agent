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
import time
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from backlog_client import (  # noqa: E402
    advance_issue_status,
    bl_request,
    load_backlog_env,
    resolve_bl_base,
    resolve_status_id,
)
from resolve_deployed_issues import extract_pr_numbers  # noqa: E402

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

def pr_url_re(repo: str) -> re.Pattern[str]:
    """`https://github.com/<repo>/pull/N` だけを拾う正規表現。

    owner/repo を捨てて番号だけ見ると、別リポジトリの参考リンクが索引に入り、
    同じ番号のローカルPRが「紐づいている」ことになって検知漏れになる。
    github-issue-to-backlog.yml は Issue 本文をそのまま説明欄へ写すので、
    参考リンクは実際に説明欄へ入ってくる。
    """
    return re.compile(r"https://github\.com/" + re.escape(repo) + r"/pull/(\d+)")

# Discord の 1 通は 2000 文字。余裕を見て切る。
MESSAGE_CHARS = 1700

# PR 一覧の1ページの件数。100 だと1ページ 2MB を超え、読み落としが起きやすい。
PR_PAGE_SIZE = 50


def rank(status_name: str) -> int:
    """ステータスの位置。知らない名前は -1（比較しない）。"""
    try:
        return STATUS_ORDER.index((status_name or "").strip())
    except ValueError:
        return -1


def linked_pr_numbers(description: str, repo: str) -> list[int]:
    """説明欄に書かれた、このリポジトリの PR 番号。重複は除き、書かれた順に返す。"""
    nums: list[int] = []
    for m in pr_url_re(repo).finditer(description or ""):
        n = int(m.group(1))
        if n not in nums:
            nums.append(n)
    return nums


def find_mismatches(
    issues: list[dict], prs: list[dict], repo: str, fixes: dict[str, str] | None = None
) -> list[str]:
    """ズレの一覧を人が読む1行ずつにして返す。ズレが無ければ空。

    issues: Backlog の課題（issueKey / status.name / description）
    prs:    GitHub の PR（number / draft / state / base / mergedAt / title /
            プロモーションPRなら commitSubjects）
    どちらも API の形をそのまま受ける。テストから素の dict を渡せるようにしている。
    fixes を渡すと、検知3(マージ済みなのに課題が進んでいない)について
    「課題キー → 進めるべきステータス名」を詰める。同じ課題が複数PRに出たら先のほうを採る。
    """
    by_pr: dict[int, list[dict]] = {}
    for issue in issues:
        for n in linked_pr_numbers(issue.get("description") or "", repo):
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
        #    ここで continue しない。課題に直接紐づかないプロモーションPRを
        #    検知3へ通す必要がある。
        if not linked and is_open and not pr.get("draft"):
            lines.append(f"- #{num} どの課題にも紐づいていません（{title[:50]}）")

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
        #    develop→release→main のプロモーションPR自身はどの課題にも紐づかない
        #    （pr-to-backlog.yml が URL を追記するのは own_key の課題だけ）。
        #    squash マージのコミット題名に残る元PR番号 "(#1234)" から元PRへ展開し、
        #    その課題を見る。これが無いと release・main 昇格の同期漏れを拾えない。
        if pr.get("mergedAt"):
            want = MERGED_MIN_STATUS.get(pr.get("base") or "")
            related = list(linked)
            for origin in extract_pr_numbers(pr.get("commitSubjects") or []):
                for issue in by_pr.get(origin, []):
                    if issue not in related:
                        related.append(issue)
            for issue in related:
                key = issue["issueKey"]
                name = (issue.get("status") or {}).get("name", "")
                cur = rank(name)
                if want and 0 <= cur < rank(want):
                    if fixes is not None and (key not in fixes or rank(want) > rank(fixes[key])):
                        fixes[key] = want
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
        nums = linked_pr_numbers(issue.get("description") or "", repo)
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


def rollup_candidates(issues: list[dict]) -> list[dict]:
    """子課題が1件以上「完了」で、自分はまだ「完了」でない親課題。

    ここは候補を挙げるだけ。全部の子が完了かは、呼び出し側が子を全件取って確かめる
    （issues は対象ステータスだけの一覧なので、ここの情報では「全部」と言えない）。
    """
    by_id = {i["id"]: i for i in issues if i.get("id") is not None}
    parents: list[dict] = []
    for i in issues:
        pid = i.get("parentIssueId")
        parent = by_id.get(pid) if pid else None
        if not parent or parent in parents:
            continue
        if (i.get("status") or {}).get("name") != "完了":
            continue
        pname = (parent.get("status") or {}).get("name", "")
        if pname != "完了" and rank(pname) >= 0:
            parents.append(parent)
    return parents


def all_done(children: list[dict]) -> bool:
    """子が1件以上あり、すべて「完了」。"""
    return bool(children) and all((c.get("status") or {}).get("name") == "完了" for c in children)


def build_fix_message(fixed: list[str]) -> str:
    """自動で直した課題の報告。直した事実を黙って済ませない。"""
    if not fixed:
        return ""
    return f"🔧 **Backlog を自動で更新 {len(fixed)} 件**\n" + "\n".join(fixed)


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


def gh_get(url: str, token: str, attempts: int = 3):
    """GitHub API を叩いて JSON を返す。読み込み失敗は読み直す。

    PR 一覧は1ページ 1MB 超になり、`IncompleteRead` で途中で切れることが実際にあった。
    1回の読み落としで突き合わせ全体が落ちると、ズレの報告がその日だけ出なくなる。
    """
    last: Exception | None = None
    for i in range(attempts):
        req = urllib.request.Request(url)
        req.add_header("Authorization", f"Bearer {token}")
        req.add_header("Accept", "application/vnd.github+json")
        req.add_header("X-GitHub-Api-Version", "2022-11-28")
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                return json.loads(resp.read())
        except Exception as e:  # noqa: BLE001  読み落とし・切断は読み直す
            last = e
            print(f"GitHub API の取得に失敗（{i + 1}/{attempts}）: {e}", file=sys.stderr)
            time.sleep(2 * (i + 1))
    raise last if last else RuntimeError("GitHub API の取得に失敗しました")


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
            # per_page は 50。100 だと1ページ 2MB を超えて読み落としが起きやすい。
            prs = gh_get(
                f"https://api.github.com/repos/{repo}/pulls"
                f"?state={state}&per_page={PR_PAGE_SIZE}&page={page}"
                f"&sort=updated&direction=desc",
                token,
            )
            if not prs:
                break
            # sort=updated&direction=desc なので、更新が since より古いページに
            # 入ったらそれ以降は対象外。merged_at で打ち切ると、古いマージ済みPRに
            # コメントが付いて前へ並んだときに対象期間のPRを取り落とす
            # （merged_at <= updated_at なので、更新日時で見れば取りこぼさない）。
            stop = False
            for p in prs:
                merged_at = p.get("merged_at")
                if state == "closed":
                    if (p.get("updated_at") or "") < since:
                        stop = True
                        continue
                    if not merged_at or merged_at < since:
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
            if stop or len(prs) < PR_PAGE_SIZE:
                break
            page += 1
    return out


def gh_commit_subjects(repo: str, token: str, number: int) -> list[str]:
    """PR のコミット題名。取れなければ空（展開できないだけで、他の検知は続ける）。"""
    try:
        commits = gh_get(
            f"https://api.github.com/repos/{repo}/pulls/{number}/commits?per_page=250", token
        )
    except Exception as e:  # noqa: BLE001
        print(f"#{number} のコミット取得に失敗: {e}", file=sys.stderr)
        return []
    return [(c.get("commit", {}).get("message") or "").split("\n", 1)[0] for c in commits]


def apply_fixes(
    base: str, api_key: str, project_key: str, issues: list[dict], fixes: dict[str, str]
) -> list[str]:
    """検知3のズレと、子が全部完了の親を、Backlog の API で直す。

    後退はさせない(advance_issue_status)。更新できた課題だけ issues の状態も書き換え、
    報告の取り直しと親の判定に使う。直した内容は報告用の行で返す。
    """
    fixed: list[str] = []
    by_key = {i["issueKey"]: i for i in issues}

    def move(key: str, name: str) -> None:
        sid = resolve_status_id(base, api_key, project_key, name)
        if sid is None:
            return
        before = ((by_key[key].get("status") or {}).get("name")) or "?"
        if advance_issue_status(base, api_key, key, sid, name) == "updated":
            by_key[key]["status"] = {"id": sid, "name": name}
            fixed.append(f"- {key} 「{before}」→「{name}」")

    for key, want in sorted(fixes.items()):
        if key in by_key:
            move(key, want)

    for parent in rollup_candidates(issues):
        children = bl_request(
            base, api_key, "GET", "/issues",
            {"parentIssueId[]": [parent["id"]], "count": 100}, fatal=False,
        ) or []
        if all_done(children):
            move(parent["issueKey"], "完了")
            if fixed and fixed[-1].startswith(f"- {parent['issueKey']} "):
                fixed[-1] += "（子課題がすべて完了）"
    return fixed


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
    # プロモーションPR(release/main 宛)だけコミット題名を取る。元PRへの展開に使う。
    for pr in prs:
        if pr.get("mergedAt") and pr.get("base") in ("release", "main"):
            pr["commitSubjects"] = gh_commit_subjects(repo, token, pr["number"])
    fixes: dict[str, str] = {}
    lines = find_mismatches(issues, prs, repo, fixes)
    fixed: list[str] = []
    if os.environ.get("AUTO_FIX") == "true":
        fixed = apply_fixes(base, api_key, project_key, issues, fixes)
        # 直した課題はもうズレではない。報告から外すため、状態を直して取り直す。
        lines = find_mismatches(issues, prs, repo)
    else:
        for key, want in sorted(fixes.items()):
            print(f"[dry-run] {key} を「{want}」へ進められます")
    print(f"課題 {len(issues)} 件 / PR {len(prs)} 件を突き合わせ: ズレ {len(lines)} 件 / 自動更新 {len(fixed)} 件")
    for line in lines + fixed:
        print(line)
    # 直した分もズレも無ければ空ファイル。呼び出し側は空なら投稿しない。
    parts = [build_fix_message(fixed), build_message(lines, repo) if lines else ""]
    message = "\n\n".join(p for p in parts if p)
    if out_path:
        with open(out_path, "w", encoding="utf-8") as f:
            f.write(message)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
