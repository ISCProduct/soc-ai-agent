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
import backlog_sync_audit as A
from backlog_sync_audit import (
    all_done, apply_fixes, build_fix_message, build_message, find_mismatches,
    linked_pr_numbers, rank, rollup_candidates,
)

REPO = "ISCProduct/soc-ai-agent"


def find_mismatches_(issues, prs):
    """リポジトリ名を毎回書かないための薄い包み。"""
    return find_mismatches(issues, prs, REPO)

PR = f"https://github.com/{REPO}/pull/"


def issue(key, status, *prs):
    desc = "## 概要\n何か\n"
    if prs:
        desc += "\n## GitHub PR\n" + "\n".join(PR + str(n) for n in prs)
    return {"issueKey": key, "status": {"name": status}, "description": desc}


def pr(number, *, state="open", draft=False, base="develop", merged=None,
       title="feat: 何か", subjects=None):
    return {"number": number, "state": state, "draft": draft, "base": base,
            "mergedAt": merged, "title": title, "commitSubjects": subjects or []}


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
    find_mismatches_([issue("SOCAIAGENT-474", "処理中", 1974)], [pr(1974)]),
    ["- SOCAIAGENT-474 は「処理中」ですが #1974 はレビュー待ちです"],
)

# キーがどこにも書かれておらず同期が効かなかったPR。
check(
    "どの課題にも紐づいていないオープンPR",
    find_mismatches_([], [pr(1970, title="fix(company): 企業名を囲む")]),
    ["- #1970 どの課題にも紐づいていません（fix(company): 企業名を囲む）"],
)

# マージ済みなのに進んでいない（SOCAIAGENT-450 / 473 の実例）。
check(
    "developへマージ済みなのに未対応",
    find_mismatches_(
        [issue("SOCAIAGENT-450", "未対応", 1954)],
        [pr(1954, state="closed", merged="2026-11-04T05:43:00Z")],
    ),
    ["- SOCAIAGENT-450 は「未対応」ですが #1954 は develop にマージ済みです"
     "（「ステージング環境反映」以降のはず）"],
)
check(
    "mainへマージ済みなのにリリース待ち",
    find_mismatches_(
        [issue("SOCAIAGENT-9", "リリース待ち", 10)],
        [pr(10, state="closed", base="main", merged="2026-11-04T00:00:00Z")],
    ),
    ["- SOCAIAGENT-9 は「リリース待ち」ですが #10 は main にマージ済みです（「完了」以降のはず）"],
)

# PR確認待ちなのに見るものが無い（424 / 445 / 453 / 379 の実例）。
check(
    "PR確認待ちだがPRが紐づいていない",
    find_mismatches_([issue("SOCAIAGENT-424", "PR確認待ち")], []),
    ["- SOCAIAGENT-424 は「PR確認待ち」ですが PR が紐づいていません"],
)
check(
    "PR確認待ちだが紐づくPRが閉じている",
    find_mismatches_(
        [issue("SOCAIAGENT-5", "PR確認待ち", 11)],
        [pr(11, state="closed")],
    ),
    ["- SOCAIAGENT-5 は「PR確認待ち」ですが #11 は閉じています"],
)

# --- 検知してはいけないもの ---

check(
    "正常（PR確認待ち × オープンPR）",
    find_mismatches_([issue("SOCAIAGENT-1", "PR確認待ち", 100)], [pr(100)]),
    [],
)
check(
    "ドラフトPRは課題が処理中でも正常",
    find_mismatches_([issue("SOCAIAGENT-2", "処理中", 101)], [pr(101, draft=True)]),
    [],
)
check(
    "ドラフトPRはキーが無くても指摘しない",
    find_mismatches_([], [pr(1912, draft=True)]),
    [],
)
check(
    "完了済みの課題に追いPRが出ているのは正常",
    find_mismatches_([issue("SOCAIAGENT-347", "完了", 1821)], [pr(1821)]),
    [],
)
check(
    "ステージング反映済みの課題に追いPRが出ているのは正常",
    find_mismatches_([issue("SOCAIAGENT-384", "ステージング環境反映", 1972, 1973)],
                    [pr(1972), pr(1973)]),
    [],
)
check(
    "feature宛のマージはステータスを問わない",
    find_mismatches_(
        [issue("SOCAIAGENT-3", "処理中", 12)],
        [pr(12, state="closed", base="feature/other", merged="2026-11-04T00:00:00Z")],
    ),
    [],
)

# --- 部品 ---

check(
    "説明欄から複数のPR番号を順に拾う",
    linked_pr_numbers(f"## GitHub PR\n{PR}1972\n{PR}1973\n{PR}1972", REPO),
    [1972, 1973],
)
# 別リポジトリの参考リンクを拾うと、同番号のローカルPRが「紐づき済み」になって
# 検知漏れになる。
check(
    "別リポジトリのPR URLは索引に入れない",
    linked_pr_numbers("参考: https://github.com/upstream/library/pull/1970", REPO),
    [],
)
check(
    "別リポジトリのリンクだけの課題では未リンクPRを見逃さない",
    find_mismatches_(
        [issue("SOCAIAGENT-7", "完了")
         | {"description": "参考: https://github.com/upstream/library/pull/1970"}],
        [pr(1970, title="fix: 何か")],
    ),
    ["- #1970 どの課題にも紐づいていません（fix: 何か）"],
)

# プロモーションPR(develop→release→main)は課題に直接紐づかない。
# squash コミット題名の "(#1234)" から元PRへ展開しないと昇格の同期漏れを拾えない。
check(
    "プロモーションPRを元PRへ展開して検知する",
    find_mismatches_(
        [issue("SOCAIAGENT-8", "ステージング環境反映", 1900)],
        [pr(1900, state="closed", merged="2026-11-01T00:00:00Z"),
         pr(1949, state="closed", base="main", merged="2026-11-04T00:00:00Z",
            title="chore: release を main へ",
            subjects=["feat: 何か (#1900)", "Merge pull request #1948 from ISCProduct/release"])],
    ),
    ["- SOCAIAGENT-8 は「ステージング環境反映」ですが #1949 は main にマージ済みです"
     "（「完了」以降のはず）"],
)
check(
    "プロモーションPRの元PRが完了済みなら指摘しない",
    find_mismatches_(
        [issue("SOCAIAGENT-9", "完了", 1900)],
        [pr(1949, state="closed", base="main", merged="2026-11-04T00:00:00Z",
            subjects=["feat: 何か (#1900)"])],
    ),
    [],
)
check("知らないステータスは比較しない", rank("なにか"), -1)
check("ステータスの前後", rank("処理中") < rank("PR確認待ち"), True)

# 2000文字の上限を越えないこと。越えると Discord に拒否されて1通も届かない。
many = [f"- SOCAIAGENT-{n} は「処理中」ですが #{n} はレビュー待ちです" for n in range(200, 300)]
msg = build_message(many, "ISCProduct/soc-ai-agent")
check("長い報告でも2000文字に収まる", len(msg) < 2000, True)
check("切り捨てた件数を残す", "ほか" in msg, True)

# --- 自動更新 ---

# 検知3のズレは、進めるべきステータスが fixes に入る。
fx = {}
find_mismatches(
    [issue("SOCAIAGENT-10", "ステージング環境反映", 100), issue("SOCAIAGENT-11", "処理中", 100)],
    [pr(100, state="closed", base="main", merged="2026-11-12T00:00:00Z")], REPO, fx)
check("main マージ済みなら完了へ進める", fx, {"SOCAIAGENT-10": "完了", "SOCAIAGENT-11": "完了"})
fx = {}
find_mismatches([issue("SOCAIAGENT-12", "完了", 101)],
    [pr(101, state="closed", base="main", merged="2026-11-12T00:00:00Z")], REPO, fx)
check("既に完了なら何もしない", fx, {})

def st(name): return {"name": name}
def bi(id_, key, status, parent=None):
    return {"id": id_, "issueKey": key, "status": st(status), "parentIssueId": parent}

parent = bi(1, "SOCAIAGENT-1", "ステージング環境反映")
check("子が完了の親は候補", [i["issueKey"] for i in rollup_candidates([parent, bi(2, "SOCAIAGENT-2", "完了", 1)])], ["SOCAIAGENT-1"])
check("子が完了でなければ候補にしない", rollup_candidates([parent, bi(2, "SOCAIAGENT-2", "処理中", 1)]), [])
check("親が完了済みなら候補にしない", rollup_candidates([bi(1, "SOCAIAGENT-1", "完了"), bi(2, "SOCAIAGENT-2", "完了", 1)]), [])
check("全部完了", all_done([{"status": st("完了")}, {"status": st("完了")}]), True)
check("1件でも未完了なら全部完了でない", all_done([{"status": st("完了")}, {"status": st("処理中")}]), False)
check("子が0件は全部完了としない", all_done([]), False)

# 追いPRが開いている課題は、過去のマージだけで自動更新しない。
fx = {}
find_mismatches(
    [issue("SOCAIAGENT-13", "PR確認待ち", 100, 102)],
    [pr(100, state="closed", base="main", merged="2026-11-12T00:00:00Z"), pr(102)], REPO, fx)
check("追いPRが開いていれば自動更新しない", fx, {})

# apply_fixes: API は差し替える。実際に PATCH された先を記録する。
patched = []
live = {}        # 課題キー → 現在のステータス名（更新直前の GET が返す）
children = {}    # 親の id → 子の一覧
get_fail = set() # GET に失敗させる課題キー

def fake_req(base, key, method, path, data=None, **k):
    if method == "GET" and path == "/issues":
        pid = data["parentIssueId[]"][0]
        return children.get(pid, [])[data["offset"]:data["offset"] + 100]
    if method == "GET":
        ik = path.rsplit("/", 1)[1]
        return None if ik in get_fail else {"status": st(live[ik])}
    patched.append((path.rsplit("/", 1)[1], data["statusId"]))
    return {}

A.bl_request = fake_req
A.resolve_status_id = lambda base, key, proj, name, **k: {"完了": 4}.get(name)

def run(issues, fixes):
    patched.clear()
    return apply_fixes("b", "k", "SOCAIAGENT", issues, fixes)

live.update({"SOCAIAGENT-1": "ステージング環境反映", "SOCAIAGENT-2": "ステージング環境反映"})
children[1] = [bi(2, "SOCAIAGENT-2", "完了", 1), bi(3, "SOCAIAGENT-3", "完了", 1)]
issues = [bi(1, "SOCAIAGENT-1", "ステージング環境反映"), bi(2, "SOCAIAGENT-2", "ステージング環境反映", 1),
          bi(3, "SOCAIAGENT-3", "完了", 1)]
out = run(issues, {"SOCAIAGENT-2": "完了"})
check("子を直してから親も完了にする", patched, [("SOCAIAGENT-2", 4), ("SOCAIAGENT-1", 4)])
check("報告に親の理由を残す", out[-1].endswith("（子課題がすべて完了）"), True)

live["SOCAIAGENT-1"] = "ステージング環境反映"
children[1] = [bi(3, "SOCAIAGENT-3", "完了", 1), bi(4, "SOCAIAGENT-4", "未対応", 1)]
run([bi(1, "SOCAIAGENT-1", "ステージング環境反映"), bi(3, "SOCAIAGENT-3", "完了", 1)], {})
check("未完了の子がいる親は動かさない", patched, [])

# マージ由来の更新でも、親(子がいる課題)は子が全部完了のときだけ。
run([bi(1, "SOCAIAGENT-1", "ステージング環境反映")], {"SOCAIAGENT-1": "完了"})
check("マージ由来でも未完了の子がいる親は動かさない", patched, [])

# 子が101件。先頭100件が完了でも、2ページ目に未完了がいれば親は動かさない。
children[1] = [bi(100 + n, f"SOCAIAGENT-{100 + n}", "完了", 1) for n in range(100)] + [bi(300, "SOCAIAGENT-300", "処理中", 1)]
run([bi(1, "SOCAIAGENT-1", "ステージング環境反映"), bi(100, "SOCAIAGENT-100", "完了", 1)], {})
check("子が100件を超えても全ページ見る", patched, [])
children[1] = [bi(100 + n, f"SOCAIAGENT-{100 + n}", "完了", 1) for n in range(101)]
run([bi(1, "SOCAIAGENT-1", "ステージング環境反映"), bi(100, "SOCAIAGENT-100", "完了", 1)], {})
check("101件すべて完了なら親も完了", patched, [("SOCAIAGENT-1", 4)])

# 更新直前の GET が失敗したら更新しない。取れた現在値が既に完了なら戻さない。
children[1] = []
get_fail.add("SOCAIAGENT-1")
run([bi(1, "SOCAIAGENT-1", "ステージング環境反映")], {"SOCAIAGENT-1": "完了"})
check("更新直前の取得に失敗したら更新しない", patched, [])
get_fail.clear()
live["SOCAIAGENT-1"] = "完了"
run([bi(1, "SOCAIAGENT-1", "ステージング環境反映")], {"SOCAIAGENT-1": "完了"})
check("既に完了なら PATCH しない", patched, [])

check("直した分が無ければ報告は空", build_fix_message([]), "")
many_fixed = [f"- SOCAIAGENT-{n} 「ステージング環境反映」→「完了」" for n in range(100, 300)]
total = build_fix_message(many_fixed) + "\n\n" + build_message(many, "ISCProduct/soc-ai-agent", 1700 - len(build_fix_message(many_fixed)) - 2)
check("自動更新が多くても通知全体が2000文字に収まる", len(total) < 2000, True)

if failures:
    print(f"FAIL: {failures} 件")
    sys.exit(1)
print("PASS (ズレ判定と自動更新)")
PY
