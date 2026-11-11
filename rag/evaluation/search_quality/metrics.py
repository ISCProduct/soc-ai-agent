"""検索品質の指標（#1773）。

正解は「関連あり / なし」の2値で持つ。順位指標は2値の関連度で計算する。
"""
from __future__ import annotations

import math
from typing import Dict, Iterable, List, Sequence


def precision_at_k(ranked: Sequence[str], relevant: Iterable[str], k: int) -> float:
    """上位 k 件のうち関連ありの割合。返った件数が k 未満でも分母は k にする
    （返さなかった枠を「外れ」と数え、件数を絞っただけで良く見えないようにする）。"""
    if k <= 0:
        raise ValueError("k must be positive")
    relevant_set = set(relevant)
    hits = sum(1 for doc_id in ranked[:k] if doc_id in relevant_set)
    return hits / k


def recall_at_k(ranked: Sequence[str], relevant: Iterable[str], k: int) -> float:
    """関連ありのうち、上位 k 件に入った割合。関連ありが0件のクエリは定義できないので弾く。"""
    if k <= 0:
        raise ValueError("k must be positive")
    relevant_set = set(relevant)
    if not relevant_set:
        raise ValueError("relevant must not be empty")
    hits = sum(1 for doc_id in ranked[:k] if doc_id in relevant_set)
    return hits / len(relevant_set)


def reciprocal_rank(ranked: Sequence[str], relevant: Iterable[str]) -> float:
    """最初の関連ありの順位の逆数。見つからなければ 0。"""
    relevant_set = set(relevant)
    for index, doc_id in enumerate(ranked, start=1):
        if doc_id in relevant_set:
            return 1.0 / index
    return 0.0


def ndcg_at_k(ranked: Sequence[str], relevant: Iterable[str], k: int) -> float:
    """2値の関連度での nDCG@k。理想の並び（関連ありが先頭に詰まる）を 1 とする。"""
    if k <= 0:
        raise ValueError("k must be positive")
    relevant_set = set(relevant)
    if not relevant_set:
        raise ValueError("relevant must not be empty")
    dcg = sum(
        1.0 / math.log2(index + 1)
        for index, doc_id in enumerate(ranked[:k], start=1)
        if doc_id in relevant_set
    )
    ideal = sum(1.0 / math.log2(index + 1) for index in range(1, min(k, len(relevant_set)) + 1))
    return dcg / ideal


def evaluate_query(ranked: Sequence[str], relevant: Sequence[str], ks: Sequence[int]) -> Dict[str, float]:
    """1クエリの指標をまとめて返す。"""
    scores: Dict[str, float] = {"mrr": reciprocal_rank(ranked, relevant)}
    for k in ks:
        scores[f"precision@{k}"] = precision_at_k(ranked, relevant, k)
        scores[f"recall@{k}"] = recall_at_k(ranked, relevant, k)
        scores[f"ndcg@{k}"] = ndcg_at_k(ranked, relevant, k)
    return scores


def mean_scores(per_query: List[Dict[str, float]]) -> Dict[str, float]:
    """クエリごとの指標を平均する（マクロ平均）。"""
    if not per_query:
        return {}
    keys = per_query[0].keys()
    return {key: sum(scores[key] for scores in per_query) / len(per_query) for key in keys}
