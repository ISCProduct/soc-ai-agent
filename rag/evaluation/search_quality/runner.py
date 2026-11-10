"""固定セットで検索品質を測る（#1773）。

本番と同じ関数（学生検索は upsert_student / query_students、企業リサーチは
set_cached_documents / get_cached_documents）を通し、Chroma のクライアントだけを評価用に
差し替える。検索アルゴリズムそのものには手を入れない。
"""
from __future__ import annotations

import contextlib
from typing import Any, Dict, Iterator, List, Sequence

import vector_store
from student_profile_store import query_students, upsert_student

from .dataset import Dataset
from .embedders import Embedder
from .metrics import evaluate_query, mean_scores

STUDENT_KS = (1, 3, 5, 10)
# 企業リサーチは本番（services/cache.get_cached_context）が上位5件だけを使う
COMPANY_RESEARCH_N_RESULTS = 5
COMPANY_KS = (1, 3, 5)


@contextlib.contextmanager
def use_chroma_client(client: Any) -> Iterator[None]:
    """検索経路が使う Chroma クライアントを一時的に差し替える。"""
    with vector_store._chroma_lock:
        previous = vector_store._chroma_client
        vector_store._chroma_client = client
    try:
        yield
    finally:
        with vector_store._chroma_lock:
            vector_store._chroma_client = previous


def evaluate_student_search(dataset: Dataset, embed: Embedder, client: Any, ks: Sequence[int] = STUDENT_KS) -> Dict[str, Any]:
    """学生検索。文書 ID を学生 ID（1始まりの連番）に割り当てて登録し、クエリごとに上位を取る。"""
    user_ids = {doc.id: index for index, doc in enumerate(dataset.documents, start=1)}
    doc_ids = {user_id: doc_id for doc_id, user_id in user_ids.items()}
    per_query: List[Dict[str, Any]] = []
    with use_chroma_client(client):
        embeddings = embed([doc.text for doc in dataset.documents])
        for doc, embedding in zip(dataset.documents, embeddings):
            upsert_student(user_ids[doc.id], doc.text, embedding)
        query_embeddings = embed([query.query for query in dataset.queries])
        for query, embedding in zip(dataset.queries, query_embeddings):
            hits = query_students(embedding, max(ks))
            ranked = [doc_ids[hit["user_id"]] for hit in hits]
            per_query.append({"id": query.id, "ranked": ranked, **evaluate_query(ranked, query.relevant, ks)})
    return _summary(dataset, per_query)


def evaluate_company_research(dataset: Dataset, embed: Embedder, client: Any, ks: Sequence[int] = COMPANY_KS) -> Dict[str, Any]:
    """企業リサーチ。企業・職種ごとにキャッシュへ保存し、本番と同じ絞り込み（企業・職種、職種が
    無ければ同じ企業）と件数で取る。他社の文書が混ざった件数も数える（混ざってはいけない）。"""
    by_text = {doc.text: doc for doc in dataset.documents}
    groups: Dict[tuple, List] = {}
    for doc in dataset.documents:
        groups.setdefault((doc.company, doc.role), []).append(doc)
    per_query: List[Dict[str, Any]] = []
    foreign = 0
    with use_chroma_client(client):
        for (company, role), docs in groups.items():
            cache_key = vector_store.build_cache_key("company_research", company, role)
            vector_store.set_cached_documents(
                cache_key, [doc.text for doc in docs], embed([doc.text for doc in docs]), source="search_quality_eval"
            )
        for query in dataset.queries:
            cache_key = vector_store.build_cache_key("company_research", query.company, query.role)
            texts = vector_store.get_cached_documents(cache_key, embed([query.query])[0], n_results=COMPANY_RESEARCH_N_RESULTS)
            ranked = []
            for text in texts:
                doc = by_text.get(text)
                if doc is None:
                    continue
                if doc.company != query.company:
                    foreign += 1
                ranked.append(doc.id)
            per_query.append({"id": query.id, "ranked": ranked, **evaluate_query(ranked, query.relevant, ks)})
    summary = _summary(dataset, per_query)
    summary["foreign_documents"] = foreign
    return summary


def _summary(dataset: Dataset, per_query: List[Dict[str, Any]]) -> Dict[str, Any]:
    scores = [{k: v for k, v in row.items() if k not in ("id", "ranked")} for row in per_query]
    return {
        "dataset": dataset.name,
        "documents": len(dataset.documents),
        "queries": len(dataset.queries),
        "mean": {k: round(v, 4) for k, v in mean_scores(scores).items()},
        "per_query": per_query,
    }


def compare_to_baseline(report: Dict[str, Any], baseline: Dict[str, Any], tolerance: float = 0.0) -> List[str]:
    """基準値より下がった指標を返す。tolerance は許容する下げ幅。"""
    regressions: List[str] = []
    for target, summary in report.items():
        base = baseline.get(target, {})
        for metric, value in summary.get("mean", {}).items():
            base_value = base.get("mean", {}).get(metric)
            if base_value is not None and value < base_value - tolerance - 1e-9:
                regressions.append(f"{target} {metric}: {value:.4f} < 基準 {base_value:.4f}")
        if summary.get("foreign_documents", 0) > base.get("foreign_documents", 0):
            regressions.append(f"{target} 他社の文書が混ざった: {summary['foreign_documents']} 件")
    return regressions
