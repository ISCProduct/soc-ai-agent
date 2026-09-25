#!/usr/bin/env python3
"""本番へ反映された「個別PR」を集めて、更新情報の取り込み用JSONを標準出力へ出す。

main へ直接マージされるのは `release:` 傘PRだけで、その本文は運用担当者向けに
書かれる。「起動ジョブの失敗通知が初めて有効になる」「スキーマ変更あり」のような
記述がそのまま要約され、学生向けの更新情報として表示されてしまった。

傘PRは要約対象にせず、squash マージのコミット題名に残る元PR番号 `(#1234)` を辿り、
**中身の個別PR**を送る。個別PRのタイトルは `feat:` / `fix:` / `ops:` のように
主題が書かれているため、Backend 側の開発者向け判定が効く。

close_deployed_issues と同じ抽出ロジック(extract_pr_numbers)を使う。
別々に持つと、片方だけ直したときに拾う範囲がずれる。
"""

import json
import os
import subprocess
import sys

sys.path.insert(0, ".github/scripts")
from resolve_deployed_issues import extract_pr_numbers

REPO = os.environ.get("GH_REPO") or os.environ.get("GITHUB_REPOSITORY", "")

# 1回のデプロイで取り込む個別PRの上限。
# 傘PR1本に数十件ぶら下がるため、際限なく引くとAPIを叩きすぎる。
MAX_PRS = 60


def run(args: list[str]) -> str:
    r = subprocess.run(args, capture_output=True, text=True)
    return r.stdout if r.returncode == 0 else ""


def commit_subjects() -> list[str]:
    """直近のリリースマージに含まれるコミット題名を返す。"""
    out = run(["git", "log", "--format=%s", "HEAD~1..HEAD", "--no-merges"])
    subjects = out.splitlines()
    if not subjects:
        # マージコミット自体しか無い場合は、マージした側の履歴を辿る。
        subjects = run(["git", "log", "--format=%s", "HEAD^1..HEAD^2"]).splitlines()
    return subjects


def fetch_pr(number: int) -> dict | None:
    out = run([
        "gh", "pr", "view", str(number), "--repo", REPO,
        "--json", "number,title,body,mergedAt",
    ])
    if not out.strip():
        return None
    try:
        pr = json.loads(out)
    except json.JSONDecodeError:
        return None
    if not pr.get("mergedAt"):
        return None
    return {
        "pr_number": pr["number"],
        "title": pr.get("title") or "",
        "body": pr.get("body") or "",
        "merged_at": pr["mergedAt"],
    }


def main() -> int:
    numbers = extract_pr_numbers(commit_subjects())[:MAX_PRS]
    print(f"元PR {len(numbers)} 件を検出", file=sys.stderr)

    sources = []
    for n in numbers:
        pr = fetch_pr(n)
        if pr is not None:
            sources.append(pr)

    print(f"取り込み対象 {len(sources)} 件", file=sys.stderr)
    json.dump(sources, sys.stdout, ensure_ascii=False)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
