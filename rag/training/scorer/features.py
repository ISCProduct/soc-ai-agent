"""採点モデルの特徴量（文字 n-gram のハッシュ）。

Go の推論（#2171）が同じ入力から同じ特徴量を作れるよう、各手順は Go の標準ライブラリ
（unicode/utf8・hash/fnv・math）と、リポジトリが既に使っている golang.org/x/text/unicode/norm で
同じ結果になるものだけを使う。手順を変えたら FEATURE_SPEC_VERSION を上げ、
rag/testdata/scorer_feature_golden.json を作り直す（README.md の「特徴量の手順」）。
"""

from __future__ import annotations

import math
import unicodedata
from collections import Counter
from dataclasses import dataclass
from typing import Iterable, Mapping

# FEATURE_SPEC_VERSION は特徴量の手順の版。モデルのヘッダに書き、Go は知らない版のモデルを読まない。
FEATURE_SPEC_VERSION = 1

# HASH_DIM は特徴量の次元（2^16）。重みは 2^16 × 5軸 × 2バイト ≈ 640KB で、2.7MB の目安に収まる（#1834 の 5.）。
# 2 のべき乗にしているのは、添字をハッシュの下位ビット（h & (HASH_DIM - 1)）で取れるようにするため。
HASH_DIM_BITS = 16
HASH_DIM = 1 << HASH_DIM_BITS

# NGRAM_MIN / NGRAM_MAX は文字 n-gram の範囲（1〜3文字）。
# 日本語は単語の区切りが無く、形態素解析は Go に同じものが無いので、文字単位にする。
# 1〜2文字だけでは「理由は」「例えば」のような言い回しを捉えにくく、4文字以上は面接1回ぶんの
# 件数では出現がまばらで、ハッシュの衝突を増やすだけになるため 3 までにした。
NGRAM_MIN = 1
NGRAM_MAX = 3

# 名前空間。質問と回答で同じ n-gram を別の特徴にする（回答の言い回しと、質問の言い回しを混ぜないため）。
# ハッシュにかける文字列は「名前空間 + ':' + n-gram」。名前空間は必ず先頭の2文字なので、組み合わせは一意に戻せる。
NAMESPACE_QUESTION = "q"
NAMESPACE_ANSWER = "a"

# FNV-1a 32bit の定数（Go の hash/fnv の New32a と同じ）。
FNV32_OFFSET_BASIS = 0x811C9DC5
FNV32_PRIME = 0x01000193

# WHITESPACE は空白として扱う文字。Python の str.isspace() と Go の unicode.IsSpace は範囲が違う
# （Python は U+001C〜U+001F も空白にする）ので、Go の unicode.IsSpace と同じ集合を明示する。
WHITESPACE = frozenset(
    # 見えない文字の取り違えを防ぐため、ASCII の空白以外はエスケープで書く
    "\t\n\v\f\r \u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000"
)

_REPLACEMENT_CHAR = "\ufffd"


def fnv1a32(data: bytes) -> int:
    """FNV-1a 32bit のハッシュ値を返す（Go の fnv.New32a().Sum32() と同じ）。"""
    h = FNV32_OFFSET_BASIS
    for b in data:
        h ^= b
        h = (h * FNV32_PRIME) & 0xFFFFFFFF
    return h


def normalize(text: str) -> str:
    """特徴量を作る前の正規化。

    1. 対になっていないサロゲート（U+D800〜U+DFFF）を U+FFFD にする。Python の json は "\\ud800" のような
       エスケープをそのまま文字列にするが、Go の encoding/json は U+FFFD に置き換えるため、それに合わせる。
    2. NFKC（全角英数→半角、半角カナ→全角、「㍿」→「株式会社」など）。
    3. ASCII の A〜Z だけを小文字にする。str.lower() と Go の strings.ToLower は、一部の文字
       （「İ」など）で結果が違うため、両方で必ず同じになる ASCII に限る（全角英字は 2. で ASCII になる）。
    4. WHITESPACE の連続を半角スペース1つにし、前後の空白を落とす。
    """
    text = "".join(_REPLACEMENT_CHAR if 0xD800 <= ord(c) <= 0xDFFF else c for c in text)
    text = unicodedata.normalize("NFKC", text)
    text = "".join(chr(ord(c) + 32) if "A" <= c <= "Z" else c for c in text)
    out: list[str] = []
    pending_space = False
    for c in text:
        if c in WHITESPACE:
            pending_space = True
            continue
        if pending_space and out:
            out.append(" ")
        pending_space = False
        out.append(c)
    return "".join(out)


def char_ngrams(text: str) -> list[str]:
    """正規化済みの文字列から、NGRAM_MIN〜NGRAM_MAX 文字の n-gram を出現順に返す。

    単位は Unicode のコードポイント（Go の []rune と同じ）。結合文字や ZWJ でつないだ絵文字も、
    コードポイントごとに数える。n より短い文字列からは、その長さの n-gram は出ない。
    """
    grams: list[str] = []
    for n in range(NGRAM_MIN, NGRAM_MAX + 1):
        for i in range(len(text) - n + 1):
            grams.append(text[i : i + n])
    return grams


def feature_index(namespace: str, gram: str) -> int:
    """n-gram の特徴の添字。「名前空間:n-gram」の UTF-8 の FNV-1a 32bit の下位 HASH_DIM_BITS ビット。"""
    return fnv1a32(f"{namespace}:{gram}".encode("utf-8")) & (HASH_DIM - 1)


@dataclass(frozen=True)
class SparseFeatures:
    """1サンプルの特徴量。添字の昇順に並ぶ。"""

    indices: tuple[int, ...]
    counts: tuple[int, ...]
    values: tuple[float, ...]


def turn_texts(turns: Iterable[Mapping[str, str]]) -> list[tuple[str, str]]:
    """turns（[{"q": 質問, "a": 回答}]）を、(名前空間, 文字列) の並びにする。"""
    out: list[tuple[str, str]] = []
    for turn in turns:
        out.append((NAMESPACE_QUESTION, turn.get("q", "")))
        out.append((NAMESPACE_ANSWER, turn.get("a", "")))
    return out


def extract(turns: Iterable[Mapping[str, str]]) -> SparseFeatures:
    """面接1回ぶんの turns から特徴量を作る。

    - n-gram は質問・回答の1つずつの中で作る（ターンをまたいだ n-gram は作らない）。
    - 値は 1 + ln(出現回数)（同じ言い回しの繰り返しで1つの特徴が大きくなりすぎないようにする）。
    - 最後にベクトル全体を L2 ノルムで割る（面接の長さで予測がずれないようにする）。ノルムの和は
      添字の昇順に float64 で足す（Go も同じ順に足せば同じ値になる）。
    - 特徴が1つも無い（空の turns・空白だけの文字列）ときは空の特徴量を返す。
    """
    counter: Counter[int] = Counter()
    for namespace, text in turn_texts(turns):
        for gram in char_ngrams(normalize(text)):
            counter[feature_index(namespace, gram)] += 1
    indices = tuple(sorted(counter))
    counts = tuple(counter[i] for i in indices)
    raw = [1.0 + math.log(c) for c in counts]
    sq = 0.0
    for v in raw:
        sq += v * v
    norm = math.sqrt(sq)
    values = tuple(v / norm for v in raw) if norm > 0 else ()
    return SparseFeatures(indices=indices, counts=counts, values=values)


def unicode_version() -> str:
    """NFKC に使った Unicode の版（実行している Python の unicodedata の版）。

    NFKC の結果は Unicode の版で変わる（例：U+A7F2 は Unicode 13 では未割り当てでそのまま残り、
    Unicode 14 以降は「C」になる）。Python の版ごとに Unicode の版が違い（3.10 は 13.0.0、3.12 は 15.0.0）、
    Go の golang.org/x/text/unicode/norm も Go の版で変わる（Go 1.26 までは 15.0.0、1.27 からは 17.0.0）。
    そのため、学習したときの版をモデルのヘッダ（features.unicode_version）に書き、Go は norm.Version と
    一致しなければモデルを使わない（#2326 のレビュー）。ゴールデンには版で変わらない文字だけを入れているので、
    この値は spec() には入れない（spec() はゴールデンにも書くため）。
    """
    return unicodedata.unidata_version


def spec() -> dict[str, object]:
    """モデルのヘッダに書く、特徴量の設定。Go はこれが自分の実装と一致するかを確かめてから読む。"""
    return {
        "spec_version": FEATURE_SPEC_VERSION,
        "hash": "fnv1a32",
        "dim": HASH_DIM,
        "ngram_min": NGRAM_MIN,
        "ngram_max": NGRAM_MAX,
        "unit": "codepoint",
        "normalization": ["surrogate_to_fffd", "nfkc", "ascii_lower", "collapse_whitespace", "trim"],
        "namespaces": {"question": NAMESPACE_QUESTION, "answer": NAMESPACE_ANSWER},
        "value": "1+ln(count), l2_normalized",
    }
