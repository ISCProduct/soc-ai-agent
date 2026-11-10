"""Go と同じゴールデンで、ES の字数定義がずれていないことを見る（#1957）。"""

import json
from pathlib import Path

import pytest

from services.es_review import _char_limit_bounds, count_es_chars

_GOLDEN = Path(__file__).resolve().parents[1] / "testdata" / "char_limit_cases.json"


def _verdict(text: str, limit: int, mode: str) -> str:
    if limit <= 0:
        return "unchecked"
    n = count_es_chars(text)
    low, high = _char_limit_bounds(limit, mode)
    if n > high:
        return "too_long"
    if n < low:
        return "too_short"
    return "ok"


def test_char_limit_golden_matches_es_review():
    doc = json.loads(_GOLDEN.read_text(encoding="utf-8"))
    cases = doc["cases"]
    assert cases, "ゴールデンにケースが無い"
    for i, case in enumerate(cases):
        assert count_es_chars(case["text"]) == case["chars"], i
        if case["limit"] > 0:
            assert _char_limit_bounds(case["limit"], case["mode"]) == (case["low"], case["high"]), i
        assert _verdict(case["text"], case["limit"], case["mode"]) == case["verdict"], i


@pytest.mark.parametrize("mode", ["within", "around"])
def test_golden_covers_mode(mode: str):
    doc = json.loads(_GOLDEN.read_text(encoding="utf-8"))
    assert any(case["mode"] == mode for case in doc["cases"])
