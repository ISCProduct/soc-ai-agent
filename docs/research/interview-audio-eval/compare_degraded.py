#!/usr/bin/env python3
"""sttbench の結果JSONから「clean からの悪化」を数える（#1603）。

`RESULTS_degraded.md` の headline（「劣化で悪化 N/216」「うち固有名詞を落とした N」
「補助語なし 12/54 → あり 5/54」）を出すための突き合わせ。
以前は手元のアドホックな処理で数えていてリポジトリに残っておらず、
音声を揃えた人でも headline を再計算できなかった。

    python3 compare_degraded.py off.json                 # 1本を集計
    python3 compare_degraded.py off.json --with on.json   # 補助語なし/ありを並べる
    python3 compare_degraded.py off.json --keyword-cases-only
    python3 compare_degraded.py --selftest                # 判定ロジックの自己テスト

入力は `go run ./cmd/sttbench -out <リポジトリ外>` が書く JSON。
**認識本文が入るのでリポジトリ外に置いたまま渡すこと。**

## 「悪化」の定義（ここが結論を左右するので明示する）

- 比較相手は正解テキストではなく **同じケースの clean 条件の結果**。
  正解と比べると `30パーセント` → `30%` のような表記ゆれを劣化のせいに
  数えてしまう（最初の集計はこれで「48件が壊れている」と誤った）。
- 悪化 = 次のどちらか。
  1. CER が clean より悪い
  2. clean が拾えていたキーワードを落とした（固有名詞の脱落）
- clean 側か劣化側が失敗（APIエラー / 認識失敗）しているケースは判定不能として
  別に数える。悪化にも非悪化にも入れない。
- 分母は clean を除いた件数（8ケース × 27条件 = 216）。clean 自身は比較相手。
"""

from __future__ import annotations

import argparse
import json
import sys
from dataclasses import dataclass, field
from pathlib import Path

CLEAN = "clean"


@dataclass
class Compare:
    """1モデルぶんの突き合わせ結果。"""

    cases: int = 0  # 分母（clean を除き、clean と対応が取れたもの）
    worse: int = 0
    keyword_lost: int = 0
    unjudgeable: int = 0  # どちらかが失敗して比較できない
    no_clean: int = 0  # 対応する clean が結果に無い（-condition で絞ったときなど）
    worse_ids: list[str] = field(default_factory=list)


def split_id(case_id: str) -> tuple[str, str]:
    """`<ケースID>__<condition>` を分解する。`__` が無ければ condition は空。"""
    base, sep, cond = case_id.rpartition("__")
    if not sep:
        return case_id, ""
    return base, cond


def usable(c: dict) -> bool:
    """CER やキーワードを比較してよい結果か。

    api_error は本PRで追加したフィールド。それ以前の JSON には無いので
    error 文字列の有無でも判定する（残高切れの run を取り違えないため）。
    """
    return not c.get("api_error") and not c.get("recognition_failed") and not c.get("error")


def keyword_count(c: dict) -> int:
    return len(c.get("keywords_hit") or []) + len(c.get("keywords_miss") or [])


def compare_model(cases: list[dict], keyword_cases_only: bool = False) -> Compare:
    """各ケースを自分の clean 結果と比べて悪化件数を数える。"""
    cleans: dict[str, dict] = {}
    for c in cases:
        base, cond = split_id(c["id"])
        if cond == CLEAN:
            cleans[base] = c

    out = Compare()
    for c in cases:
        base, cond = split_id(c["id"])
        if cond in ("", CLEAN):
            continue
        cl = cleans.get(base)
        if cl is None:
            out.no_clean += 1
            continue
        # 固有名詞の比較だけしたいときは、キーワードを持つケースに絞る
        if keyword_cases_only and keyword_count(cl) == 0:
            continue
        out.cases += 1
        if not usable(cl) or not usable(c):
            out.unjudgeable += 1
            continue
        lost = set(cl.get("keywords_hit") or []) - set(c.get("keywords_hit") or [])
        if lost:
            out.keyword_lost += 1
        if c.get("cer", 0.0) > cl.get("cer", 0.0) or lost:
            out.worse += 1
            out.worse_ids.append(c["id"])
    return out


def load(path: str) -> dict[str, list[dict]]:
    """結果JSONを モデル名 -> ケース一覧 に落とす。"""
    report = json.loads(Path(path).read_text(encoding="utf-8"))
    return {name: (s.get("cases") or []) for name, s in (report.get("models") or {}).items()}


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("base", nargs="?", help="sttbench の結果JSON（補助語なし側）")
    ap.add_argument("--with", dest="other", default="", help="並べて比べる結果JSON（補助語あり側）")
    ap.add_argument("--keyword-cases-only", action="store_true", help="キーワードを持つケースだけに絞る")
    ap.add_argument("--selftest", action="store_true", help="判定ロジックの自己テストだけ走らせる")
    args = ap.parse_args()

    if args.selftest:
        selftest()
        return
    if not args.base:
        ap.error("結果JSON を渡してください（または --selftest）")

    runs = [("補助語なし", load(args.base))]
    if args.other:
        runs.append(("補助語あり", load(args.other)))

    print(f"{'モデル':26s} {'run':10s} {'分母':>5s} {'悪化':>5s} {'固有名詞落ち':>12s} {'判定不能':>8s}")
    for label, models in runs:
        for name in sorted(models):
            r = compare_model(models[name], args.keyword_cases_only)
            print(f"{name:26s} {label:10s} {r.cases:5d} {r.worse:5d} {r.keyword_lost:12d} {r.unjudgeable:8d}")
            if r.no_clean:
                print(f"{'':26s} （clean と対応が取れないケース {r.no_clean} 件は分母から除外）")

    for label, models in runs:
        for name in sorted(models):
            r = compare_model(models[name], args.keyword_cases_only)
            if r.worse_ids:
                print(f"\n[{label} / {name}] 悪化したケース:")
                for cid in r.worse_ids:
                    print(f"  {cid}")


def selftest() -> None:
    """判定ロジックの自己テスト。音声もAPIキーも要らない。"""

    def case(cid: str, cer: float, hits: list[str], **kw) -> dict:
        return {"id": cid, "cer": cer, "keywords_hit": hits, "keywords_miss": [], **kw}

    cases = [
        case("a__clean", 0.01, ["Go", "Docker"]),
        case("a__noisy-white-snr0", 0.20, ["Docker"]),  # 悪化 + 固有名詞落ち
        case("a__quiet-12db", 0.01, ["Go", "Docker"]),  # 同じ → 悪化ではない
        case("a__reverb-weak", 0.00, ["Go", "Docker"]),  # cleanより良い → 悪化ではない
        case("a__lowband-phone", 0.05, ["Go", "Docker"]),  # CERだけ悪化
        case("a__fast-1.6x", 0.0, [], api_error=True, error="HTTP 429"),  # 判定不能
        case("b__noisy-pink-snr0", 0.30, []),  # cleanが無い
    ]
    r = compare_model(cases)
    assert r.cases == 5, r
    assert r.worse == 2, r
    assert r.keyword_lost == 1, r
    assert r.unjudgeable == 1, r
    assert r.no_clean == 1, r
    assert r.worse_ids == ["a__noisy-white-snr0", "a__lowband-phone"], r

    # キーワードを持たないケースは --keyword-cases-only で分母から外れる
    nokw = [case("c__clean", 0.01, []), case("c__noisy-white-snr0", 0.20, [])]
    assert compare_model(nokw).cases == 1
    assert compare_model(nokw, keyword_cases_only=True).cases == 0

    # APIエラーが混ざっても悪化件数は水増しされない（残高切れの run を誤読しないため）
    errored = [case("d__clean", 0.01, ["Go"]), case("d__noisy-white-snr0", 0.0, [], error="HTTP 429")]
    r2 = compare_model(errored)
    assert (r2.worse, r2.unjudgeable) == (0, 1), r2

    print("selftest ok")


if __name__ == "__main__":
    try:
        main()
    except (OSError, json.JSONDecodeError) as e:
        sys.exit(f"結果JSONを読めませんでした: {e}")
