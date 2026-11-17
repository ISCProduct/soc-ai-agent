"""軽量採点モデルの特徴量（#2170）のテスト。

Go の推論（#2171）と同じ特徴量になることを、rag/testdata/scorer_feature_golden.json で確かめる。
"""

import json
import math

import pytest

from training.scorer import features, golden


def _load_golden() -> dict:
    return json.loads(golden.GOLDEN_PATH.read_text(encoding="utf-8"))


@pytest.mark.parametrize("data,expected", [(b"", 0x811C9DC5), (b"a", 0xE40C292C), (b"foobar", 0xBF9CF968)])
def test_fnv1a32_matches_reference_vectors(data, expected):
    """FNV-1a 32bit の参照実装の既知の値と一致する（FNV-1 や 64bit 版との取り違えを防ぐ）。"""
    assert features.fnv1a32(data) == expected


@pytest.mark.parametrize(
    "text,expected",
    [
        ("", ""),
        ("   ", ""),
        ("Hello  World", "hello world"),
        ("ＡＢＣ１２３", "abc123"),
        ("ｶﾀｶﾅ", "カタカナ"),
        ("㍿", "株式会社"),
        ("a　b\tc\n\nd", "a b c d"),
        (" 前後 ", "前後"),
        ("İSTANBUL", "İstanbul"),
        ("x\x1fy", "x\x1fy"),
        ("\ud800あ", "�あ"),
    ],
)
def test_normalize(text, expected):
    """NFKC・ASCII だけの小文字化・Go の unicode.IsSpace と同じ空白の畳み込み・サロゲートの置き換え。"""
    assert features.normalize(text) == expected


@pytest.mark.parametrize(
    "text,expected",
    [
        ("", []),
        ("a", ["a"]),
        ("ab", ["a", "b", "ab"]),
        ("abc", ["a", "b", "c", "ab", "bc", "abc"]),
        ("abcd", ["a", "b", "c", "d", "ab", "bc", "cd", "abc", "bcd"]),
        ("😀👍", ["😀", "👍", "😀👍"]),
    ],
)
def test_char_ngrams_ranges_from_one_to_three_codepoints(text, expected):
    """1〜3文字の n-gram をコードポイント単位で出す。4文字以上の n-gram は出さない。"""
    assert features.char_ngrams(text) == expected


def test_feature_index_uses_namespace_and_low_bits():
    """添字は「名前空間:n-gram」の FNV-1a の下位16ビット。質問と回答で別の添字になる。"""
    assert features.feature_index("a", "私") == features.fnv1a32("a:私".encode("utf-8")) & 0xFFFF
    assert features.feature_index("a", "私") != features.feature_index("q", "私")
    assert features.HASH_DIM == 65536


@pytest.mark.parametrize(
    "turns",
    [[], [{"q": "", "a": ""}], [{"q": " 　", "a": "\t\n"}]],
    ids=["no_turns", "empty_strings", "whitespace_only"],
)
def test_extract_returns_empty_when_no_ngrams(turns):
    f = features.extract(turns)
    assert f.indices == () and f.counts == () and f.values == ()


def test_extract_values_are_sublinear_and_l2_normalized():
    """値は 1 + ln(回数) を L2 正規化したもの。"""
    f = features.extract([{"q": "", "a": "ああ"}])  # 「あ」2回、「ああ」1回
    by_index = dict(zip(f.indices, f.counts))
    assert by_index[features.feature_index("a", "あ")] == 2
    assert by_index[features.feature_index("a", "ああ")] == 1
    raw = {features.feature_index("a", "あ"): 1 + math.log(2), features.feature_index("a", "ああ"): 1.0}
    norm = math.sqrt(sum(v * v for v in raw.values()))
    for i, v in zip(f.indices, f.values):
        assert v == pytest.approx(raw[i] / norm, abs=1e-15)
    assert sum(v * v for v in f.values) == pytest.approx(1.0)
    assert list(f.indices) == sorted(f.indices)


def test_extract_does_not_make_ngrams_across_turns():
    """ターンをまたいだ n-gram（前の回答の末尾 + 次の回答の先頭）は作らない。"""
    across = features.feature_index("a", "bc")
    within = {features.feature_index("a", g) for g in ["a", "b", "ab", "c", "d", "cd"]}
    assert across not in within  # 偶然の衝突でテストが空振りしないことの前提
    f = features.extract([{"q": "", "a": "ab"}, {"q": "", "a": "cd"}])
    assert across not in f.indices
    assert set(f.indices) == within


def test_extract_counts_question_and_answer_separately():
    f = features.extract([{"q": "はい", "a": "はい"}])
    assert features.feature_index("q", "はい") in f.indices
    assert features.feature_index("a", "はい") in f.indices


def test_golden_file_matches_implementation():
    """ゴールデンのファイルが、今の実装で作り直したものと一致する（手順を変えたのに作り直し忘れたら落ちる）。"""
    assert _load_golden() == golden.build_golden()


def test_golden_feature_values_match_extract():
    """ゴールデンの各ケースについて、添字と出現回数は完全一致、値は 1e-12 以内（Go と同じ比べ方）。"""
    doc = _load_golden()
    assert doc["spec"] == features.spec()
    for case in doc["cases"]:
        f = features.extract(case["turns"])
        got = list(zip(f.indices, f.counts, f.values))
        assert len(got) == len(case["features"]), case["name"]
        for (gi, gc, gv), (ei, ec, ev) in zip(got, case["features"]):
            assert (gi, gc) == (ei, ec), case["name"]
            assert abs(gv - ev) <= 1e-12, case["name"]
        assert [{"q": features.normalize(t["q"]), "a": features.normalize(t["a"])} for t in case["turns"]] == case[
            "normalized"
        ], case["name"]
    for v in doc["fnv1a32"]:
        assert features.fnv1a32(v["input"].encode("utf-8")) == v["hash"]
    for v in doc["normalize"]:
        assert features.normalize(v["input"]) == v["output"]
    for v in doc["feature_index"]:
        assert features.feature_index(v["namespace"], v["gram"]) == v["index"]


def test_golden_covers_required_inputs():
    """Issue の指示どおり、日本語・英数字・全角・空白・絵文字・空文字の入力を含む。"""
    texts = [t["a"] for c in _load_golden()["cases"] for t in c["turns"]]
    joined = "".join(texts)
    assert "" in texts
    assert any("぀" <= ch <= "ヿ" or "一" <= ch <= "鿿" for ch in joined)  # 日本語
    assert any(ch.isascii() and ch.isalnum() for ch in joined)  # 英数字
    assert any("！" <= ch <= "～" for ch in joined)  # 全角英数
    assert any(ch in features.WHITESPACE for ch in joined)  # 空白
    assert any(ord(ch) >= 0x1F300 for ch in joined)  # 絵文字
    cases_with_features = [c for c in _load_golden()["cases"] if c["features"]]
    assert len(cases_with_features) >= 5
