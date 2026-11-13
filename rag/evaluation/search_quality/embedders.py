"""評価で使う埋め込み（#1773）。

- hash: 文字 bigram のハッシュで作る決定的な埋め込み。API キーも費用も要らず、CI で
  検索経路（絞り込み・鮮度・件数の頭打ち・順位付け）の回帰を見るために使う。品質の絶対値の
  目安にはならない。
- openai: 本番と同じ services.embed.embed_texts。設定変更の前後比較はこちらで行う。
"""
from __future__ import annotations

import hashlib
import math
import unicodedata
from typing import Callable, List

Embedder = Callable[[List[str]], List[List[float]]]

HASH_DIMENSIONS = 1024


def _hash_vector(text: str, dimensions: int) -> List[float]:
    normalized = "".join(unicodedata.normalize("NFKC", text).lower().split())
    vector = [0.0] * dimensions
    grams = [normalized[i : i + 2] for i in range(max(len(normalized) - 1, 1))]
    for gram in grams:
        # Python の hash() はプロセスごとに変わるので使わない
        digest = hashlib.sha1(gram.encode("utf-8")).digest()
        vector[int.from_bytes(digest[:4], "big") % dimensions] += 1.0
    norm = math.sqrt(sum(v * v for v in vector))
    return [v / norm for v in vector] if norm else vector


def hash_embedder(dimensions: int = HASH_DIMENSIONS) -> Embedder:
    def embed(texts: List[str]) -> List[List[float]]:
        return [_hash_vector(text, dimensions) for text in texts]

    return embed


def openai_embedder() -> Embedder:
    """本番と同じ埋め込み（OPENAI_API_KEY と OPENAI_EMBEDDING_MODEL を使う）。"""
    from services.embed import embed_texts

    return embed_texts


def get_embedder(name: str) -> Embedder:
    if name == "hash":
        return hash_embedder()
    if name == "openai":
        return openai_embedder()
    raise ValueError(f"unknown embedder: {name}")
