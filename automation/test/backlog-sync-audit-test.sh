#!/bin/bash
# backlog_sync_audit.py のズレ判定をテストする。
#
# ここが黙って壊れると「ズレ0件」として通知が飛ばず、検知の仕組みが沈黙する。
# pr-to-backlog.yml が success のまま何もしない問題を見つけるために入れたものなので、
# 検知側が沈黙するのは元の問題に戻るのと同じ。
#
# 使い方: ./automation/test/backlog-sync-audit-test.sh

set -uo pipefail

cd "$(dirname "$0")/../.."

python3 - <<'PY'
import sys

sys.path.insert(0, ".github/scripts")
from backlog_sync_audit import build_message, find_mismatches, linked_pr_numbers, rank

PR = "https://github.com/ISCProduct/soc-ai-agent/pull/"


def issue(key, status, *prs):
    desc = "## 概要\n何か\n"
    if prs:
        desc += "\n## GitHub PR\n" + "\n".join(PR + str(n) for n in prs)
    return {"issueKey": key, "status": {"name": status}, "description": desc}


def pr(number, *, state="open", draft=False, base="develop", merged=None, title="feat: 何か"):
    return {"number": number, "state": state, "draft": draft, "base": base,
            "mergedAt": merged, "title": title}


failures = 0


def check(name, got, want):
    global failures
    if got != want:
        print(f"NG: {name}\n    実際: {got}\n    期待: {want}")
        failures += 1
    else:
        print(f"ok: {name}")


# --- 検知すべきもの ---

# SOCAIAGENT-474 の実例。PR は開いているのに課題が「処理中」で止まっていた。
check(
    "オープンPRがあるのに課題がPR確認待ちより手前",
    find_mismatches([issue("SOCAIAGENT-474", "処理中", 1974)], [pr(1974)]),
    ["- SOCAIAGENT-474 は「処理中」ですが #1974 はレビュー待ちです"],
)

# キーがどこにも書かれておらず同期が効かなかったPR。
check(
    "どの課題にも紐づいていないオープンPR",
    find_mismatches([], [pr(1970, title="fix(company): 企業名を囲む")]),
    ["- #1970 どの課題にも紐づいていません（fix(company): 企業名を囲む）"],
)

# マージ済みなのに進んでいない（SOCAIAGENT-450 / 473 の実例）。
check(
    "developへマージ済みなのに未対応",
    find_mismatches(
        [issue("SOCAIAGENT-450", "未対応", 1954)],
        [pr(1954, state="closed", merged="2026-11-04T05:43:00Z")],
    ),
    ["- SOCAIAGENT-450 は「未対応」ですが #1954 は develop にマージ済みです"
     "（「ステージング環境反映」以降のはず）"],
)
check(
    "mainへマージ済みなのにリリース待ち",
    find_mismatches(
        [issue("SOCAIAGENT-9", "リリース待ち", 10)],
        [pr(10, state="closed", base="main", merged="2026-11-04T00:00:00Z")],
    ),
    ["- SOCAIAGENT-9 は「リリース待ち」ですが #10 は main にマージ済みです（「完了」以降のはず）"],
)

# PR確認待ちなのに見るものが無い（424 / 445 / 453 / 379 の実例）。
check(
    "PR確認待ちだがPRが紐づいていない",
    find_mismatches([issue("SOCAIAGENT-424", "PR確認待ち")], []),
    ["- SOCAIAGENT-424 は「PR確認待ち」ですが PR が紐づいていません"],
)
check(
    "PR確認待ちだが紐づくPRが閉じている",
    find_mismatches(
        [issue("SOCAIAGENT-5", "PR確認待ち", 11)],
        [pr(11, state="closed")],
    ),
    ["- SOCAIAGENT-5 は「PR確認待ち」ですが #11 は閉じています"],
)

# --- 検知してはいけないもの ---

check(
    "正常（PR確認待ち × オープンPR）",
    find_mismatches([issue("SOCAIAGENT-1", "PR確認待ち", 100)], [pr(100)]),
    [],
)
check(
    "ドラフトPRは課題が処理中でも正常",
    find_mismatches([issue("SOCAIAGENT-2", "処理中", 101)], [pr(101, draft=True)]),
    [],
)
check(
    "ドラフトPRはキーが無くても指摘しない",
    find_mismatches([], [pr(1912, draft=True)]),
    [],
)
check(
    "完了済みの課題に追いPRが出ているのは正常",
    find_mismatches([issue("SOCAIAGENT-347", "完了", 1821)], [pr(1821)]),
    [],
)
check(
    "ステージング反映済みの課題に追いPRが出ているのは正常",
    find_mismatches([issue("SOCAIAGENT-384", "ステージング環境反映", 1972, 1973)],
                    [pr(1972), pr(1973)]),
    [],
)
check(
    "feature宛のマージはステータスを問わない",
    find_mismatches(
        [issue("SOCAIAGENT-3", "処理中", 12)],
        [pr(12, state="closed", base="feature/other", merged="2026-11-04T00:00:00Z")],
    ),
    [],
)

# --- 部品 ---

check(
    "説明欄から複数のPR番号を順に拾う",
    linked_pr_numbers(f"## GitHub PR\n{PR}1972\n{PR}1973\n{PR}1972"),
    [1972, 1973],
)
check("知らないステータスは比較しない", rank("なにか"), -1)
check("ステータスの前後", rank("処理中") < rank("PR確認待ち"), True)

# 2000文字の上限を越えないこと。越えると Discord に拒否されて1通も届かない。
many = [f"- SOCAIAGENT-{n} は「処理中」ですが #{n} はレビュー待ちです" for n in range(200, 300)]
msg = build_message(many, "ISCProduct/soc-ai-agent")
check("長い報告でも2000文字に収まる", len(msg) < 2000, True)
check("切り捨てた件数を残す", "ほか" in msg, True)

if failures:
    print(f"FAIL: {failures} 件")
    sys.exit(1)
print("PASS (ズレ判定 17 ケース)")
PY
