"""Go の推論（#2171）と特徴量が一致することを確かめるためのゴールデンベクタ。

rag/testdata/scorer_feature_golden.json に、入力と「正規化の結果・特徴の添字・出現回数・値」を書く。
pytest（tests/test_scorer_features.py）がこの実装と一致することを確かめ、Go のテストも同じファイルを読む。
特徴量の手順を変えたら、FEATURE_SPEC_VERSION を上げてから作り直す:

    python -m training.scorer.golden --write
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any, Sequence

from training.scorer import features

GOLDEN_PATH = Path(__file__).resolve().parents[2] / "testdata" / "scorer_feature_golden.json"

# FNV1A32_VECTORS は FNV-1a 32bit の既知の値（FNV の参照実装のテストベクタと同じ）。
# ハッシュ関数そのものの取り違え（FNV-1 と FNV-1a、64bit 版など）を、実装に依存せず見分けるために置く。
FNV1A32_VECTORS = (
    ("", 0x811C9DC5),
    ("a", 0xE40C292C),
    ("foobar", 0xBF9CF968),
)

# NORMALIZE_INPUTS は正規化だけを確かめる入力。Go と Python で扱いが分かれやすいものを選んだ。
NORMALIZE_INPUTS = (
    "",
    "   ",
    "Hello  World",
    "ＡＢＣ１２３",  # 全角英数 → 半角、さらに小文字
    "ｶﾀｶﾅ",  # 半角カナ → 全角
    "㍿",  # 互換文字 → 「株式会社」
    "a　b\tc\n\nd",  # 全角空白・タブ・改行の連続 → 半角スペース1つ
    " 前後の空白 ",
    "İSTANBUL",  # 「İ」（U+0130）は ASCII ではないので小文字にしない（str.lower() なら「i̇」になる）
    "x\x1fy",  # U+001F は Go の unicode.IsSpace では空白ではないので残す
    "😀👍",
)
# 対になっていないサロゲートの置き換えは、ここには入れない（tests/test_scorer_features.py の test_normalize で確かめる）。
# Go の encoding/json は読む時点で U+FFFD に置き換えるので、Go 側では同じ入力を作れないため。

# FEATURE_CASES は特徴量を確かめる入力（面接1回ぶんの turns）。日本語・英数字・全角・空白・絵文字・空文字を含む。
FEATURE_CASES: tuple[tuple[str, list[dict[str, str]]], ...] = (
    ("empty_turns", []),
    ("empty_strings", [{"q": "", "a": ""}]),
    ("whitespace_only", [{"q": " 　", "a": "\t\n"}]),
    ("single_char", [{"q": "", "a": "a"}]),
    ("japanese", [{"q": "自己紹介をお願いします。", "a": "私は大学でサークルの代表をしていました。"}]),
    ("ascii_and_fullwidth", [{"q": "Why?", "a": "ＡＩ と AI で 100 件"}]),
    ("emoji_and_zwj", [{"q": "趣味は？", "a": "料理です😀 家族👨‍👩‍👧と作ります"}]),
    ("repeated", [{"q": "", "a": "ああああ"}]),
    (
        "multi_turn",
        [
            {"q": "志望動機は？", "a": "理由は二つあります。"},
            {"q": "具体的には？", "a": "例えば、3か月で50人を集めました。"},
        ],
    ),
    ("same_text_question_and_answer", [{"q": "はい", "a": "はい"}]),
)


def _feature_entry(name: str, turns: list[dict[str, str]]) -> dict[str, Any]:
    f = features.extract(turns)
    return {
        "name": name,
        "turns": turns,
        "normalized": [{"q": features.normalize(t["q"]), "a": features.normalize(t["a"])} for t in turns],
        # [添字, 出現回数, 値]。添字の昇順。
        "features": [[i, c, v] for i, c, v in zip(f.indices, f.counts, f.values)],
    }


def build_golden() -> dict[str, Any]:
    """ゴールデンの中身を作る。"""
    return {
        "description": (
            "軽量採点モデルの特徴量のゴールデン（#2170）。rag/training/scorer/golden.py が作る。"
            "Python（tests/test_scorer_features.py）と Go の推論（#2171）の両方が、この値と一致することを確かめる。"
            "counts は完全一致、values は誤差 1e-12 以内で比べる。"
        ),
        "spec": features.spec(),
        "fnv1a32": [{"input": s, "hash": h} for s, h in FNV1A32_VECTORS],
        "normalize": [{"input": s, "output": features.normalize(s)} for s in NORMALIZE_INPUTS],
        "feature_index": [
            {"namespace": ns, "gram": g, "index": features.feature_index(ns, g)}
            for ns, g in (("a", "私"), ("q", "私"), ("a", "理由は"), ("a", "😀"))
        ],
        "cases": [_feature_entry(name, turns) for name, turns in FEATURE_CASES],
    }


def dumps(doc: dict[str, Any]) -> str:
    # レビューで中身を読めるよう、日本語や絵文字はエスケープせずに UTF-8 で書き、
    # 特徴の [添字, 出現回数, 値] は1行に1つにする。
    text = json.dumps(doc, ensure_ascii=False, indent=2)
    text = _TRIPLE.sub(lambda m: f"[{m.group(1)}, {m.group(2)}, {m.group(3)}]", text)
    return text + "\n"


_TRIPLE = re.compile(r"\[\s+(\d+),\s+(\d+),\s+([0-9.eE+-]+)\s+\]")


def main(argv: Sequence[str] | None = None) -> int:
    p = argparse.ArgumentParser(prog="python -m training.scorer.golden", description=__doc__)
    p.add_argument("--write", action="store_true", help=f"{GOLDEN_PATH} を書き直す（無ければ標準出力へ）")
    args = p.parse_args(argv)
    text = dumps(build_golden())
    if args.write:
        GOLDEN_PATH.write_text(text, encoding="utf-8", newline="\n")
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
