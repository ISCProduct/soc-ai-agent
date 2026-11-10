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


# STALE_FETCHED_AT は期限切れとして保存する文書の取得時刻（鮮度の期限より十分前）。
STALE_FETCHED_AT = "2000-01-01T00:00:00+00:00"


def _cache_key(company: str, role: str, company_original: str) -> str:
    """本番と同じキー生成。company_original があれば企業ハッシュを付け、表記違いの衝突を分ける（#938）。"""
    return vector_store.build_cache_key("company_research", company, role, company_original=company_original or None)


def _mark_stale(client: Any, cache_key: str, docs: List, embeddings: List[List[float]]) -> None:
    """stale の文書だけ、取得時刻を期限切れにして同じ ID で上書きする。

    set_cached_documents は取得時刻を常に「今」にするので、そのままでは鮮度の判定（_is_fresh）が
    壊れても結果が変わらない。期限切れの文書を混ぜ、それが返らないことを確かめる。
    """
    meta = vector_store.parse_cache_key(cache_key)
    collection = client.get_collection(vector_store._sanitize_collection_name(meta["collection"]))
    ids = vector_store._doc_ids(cache_key, len(docs))
    for doc_id, doc, embedding in zip(ids, docs, embeddings):
        if not doc.stale:
            continue
        collection.upsert(
            ids=[doc_id],
            documents=[doc.text],
            embeddings=[embedding],
            metadatas=[{
                "company": meta["company"],
                "company_hash": meta.get("company_hash", ""),
                "role": meta["role"],
                "doc_type": meta["doc_type"],
                "source": "search_quality_eval",
                "fetched_at": STALE_FETCHED_AT,
                "cache_key": meta["cache_key"],
            }],
        )


def evaluate_company_research(dataset: Dataset, embed: Embedder, client: Any, ks: Sequence[int] = COMPANY_KS) -> Dict[str, Any]:
    """企業リサーチ。企業・職種ごとにキャッシュへ保存し、本番と同じ絞り込み（企業［＋企業ハッシュ］・職種、
    職種が無ければ同じ企業）・鮮度・件数で取る。次の3つは0件でなければならない:
      - foreign_documents: 他社（企業名またはサニタイズ前の企業名が違う）の文書が混ざった件数
      - stale_documents: 期限切れの文書が返った件数
      - over_limit_queries: 上限（5件）を超えて返ったクエリの数
    """
    by_text = {doc.text: doc for doc in dataset.documents}
    groups: Dict[tuple, List] = {}
    for doc in dataset.documents:
        groups.setdefault((doc.company, doc.role, doc.company_original), []).append(doc)
    per_query: List[Dict[str, Any]] = []
    foreign = stale = over_limit = 0
    with use_chroma_client(client):
        for (company, role, company_original), docs in groups.items():
            cache_key = _cache_key(company, role, company_original)
            embeddings = embed([doc.text for doc in docs])
            vector_store.set_cached_documents(cache_key, [doc.text for doc in docs], embeddings, source="search_quality_eval")
            if any(doc.stale for doc in docs):
                _mark_stale(client, cache_key, docs, embeddings)
        for query in dataset.queries:
            cache_key = _cache_key(query.company, query.role, query.company_original)
            texts = vector_store.get_cached_documents(cache_key, embed([query.query])[0], n_results=COMPANY_RESEARCH_N_RESULTS)
            if len(texts) > COMPANY_RESEARCH_N_RESULTS:
                over_limit += 1
            ranked = []
            for text in texts:
                doc = by_text.get(text)
                if doc is None:
                    continue
                if doc.company_identity != query.company_identity:
                    foreign += 1
                if doc.stale:
                    stale += 1
                ranked.append(doc.id)
            per_query.append({"id": query.id, "ranked": ranked, **evaluate_query(ranked, query.relevant, ks)})
    summary = _summary(dataset, per_query)
    summary["foreign_documents"] = foreign
    summary["stale_documents"] = stale
    summary["over_limit_queries"] = over_limit
    return summary


# INVARIANTS は基準値に関係なく、0 でなければ失敗にする件数（検索経路の不変条件）。
INVARIANTS = {
    "foreign_documents": "他社の文書が混ざった",
    "stale_documents": "期限切れの文書が返った",
    "over_limit_queries": "上限を超えて返った",
}


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
        # 不変条件は基準値と比べない。0 を超えた基準値で更新されても、ずっと失敗にする
        for key, label in INVARIANTS.items():
            if summary.get(key, 0) > 0:
                regressions.append(f"{target} {label}: {summary[key]} 件（0 でなければならない）")
    return regressions
