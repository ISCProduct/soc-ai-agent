"""ベクトルストア管理エンドポイント。"""
from __future__ import annotations

import logging
import threading
from typing import Callable, Dict, List, Optional

from fastapi import APIRouter, HTTPException

from models import VectorReembedRequest, VectorReembedResponse, VectorReembedResult, VectorStatusResponse
from services.sanitize import _sanitize_company_name_for_query
from vector_store import build_cache_key, delete_company_documents, get_index_status, replace_documents

logger = logging.getLogger("main")

router = APIRouter()


@router.get("/vector/status", response_model=VectorStatusResponse)
def vector_status(company: Optional[str] = None) -> dict:
    """ベクトルDBのインデックス状況を返す（管理・監視用）。"""
    company_original = company.strip() if company else None
    company_filter = _sanitize_company_name_for_query(company_original) if company_original else None
    try:
        return get_index_status(company_filter, company_original=company_original)
    except Exception as exc:
        logger.exception("vector status failed error=%s", exc)
        raise HTTPException(status_code=503, detail=f"vector status failed: {exc}") from exc


# 同じ企業の再埋め込みを並行させない（SOCAIAGENT-658）。置き換えの途中に別の置き換えが入ると、
# 片方が書いた文書をもう片方が「旧文書」として消しうる。
_reembed_locks: Dict[str, threading.Lock] = {}
_reembed_locks_guard = threading.Lock()


def _company_lock(key: str) -> threading.Lock:
    with _reembed_locks_guard:
        return _reembed_locks.setdefault(key, threading.Lock())


def _purge_doc_types(requested: Optional[str], target: str) -> List[str]:
    """再取得した target で入れ替える（旧文書を消す）用途。

    resume_review は company_research と同じ文書を使うので、company_research として取り直す。
    resume_review を指定したとき・doc_type を省いたときは、resume_review の旧文書も入れ替える
    （company_research だけを消すと、resume_review の旧文書が TTL 切れまで返り続ける。SOCAIAGENT-659）。
    """
    if target == "company_research" and requested in (None, "resume_review"):
        return ["company_research", "resume_review"]
    return [target]


def _refresh_targets(doc_type: Optional[str]) -> List[str]:
    """再取得する doc_type（取得のしかたごと）。resume_review は company_research と同じ文書を使う。"""
    targets: List[str] = []
    if doc_type in (None, "company_research", "resume_review"):
        targets.append("company_research")
    if doc_type in (None, "interview_hints"):
        targets.append("interview_hints")
    if doc_type in (None, "es_review"):
        targets.append("es_review")
    return targets


@router.post("/vector/reembed", response_model=VectorReembedResponse)
def vector_reembed(request: VectorReembedRequest) -> VectorReembedResponse:
    """企業ベクトルを再取得して置き換える。refresh=false なら削除だけ。

    SOCAIAGENT-658: 以前は「削除 → 再取得」の順で、再取得が例外・空だと企業データが空のまま残った。
    いまは doc_type ごとに「再取得 → 新しく書く → 書けたことを確かめる → 旧文書を消す」の順にし、
    失敗・空の doc_type は旧データを残す（成功した doc_type だけ置き換える）。結果は results に出す。
    すべての doc_type が失敗したら 502。
    """
    import main as m

    safe_company = _sanitize_company_name_for_query(request.company_name)
    doc_type = (request.doc_type or "").strip() or None
    if doc_type and doc_type not in {
        "resume_review",
        "company_research",
        "interview_hints",
        "es_review",
    }:
        raise HTTPException(status_code=400, detail="invalid doc_type")

    lock = _company_lock(build_cache_key("lock", safe_company, company_original=request.company_name))
    if not lock.acquire(blocking=False):
        raise HTTPException(status_code=409, detail="reembed already running for this company")
    try:
        if not request.refresh:
            # 削除だけの依頼（今までどおり）
            try:
                deleted_info = delete_company_documents(
                    safe_company, doc_type=doc_type, company_original=request.company_name
                )
            except Exception as exc:
                logger.exception("vector delete failed company=%s error=%s", safe_company, exc)
                raise HTTPException(status_code=503, detail=f"vector delete failed: {exc}") from exc
            return VectorReembedResponse(
                status="ok",
                company=safe_company,
                deleted=int(deleted_info.get("deleted") or 0),
                collections=deleted_info.get("collections") or {},
                refreshed=False,
                sources=[],
            )

        fetchers: Dict[str, Callable[[], Optional[str]]] = {
            "company_research": lambda: m._run_async(m._run_web_search_pipeline, safe_company, "指定なし"),
            "interview_hints": lambda: m._run_hints_web_search(safe_company, "一般職"),
            "es_review": lambda: m._run_async(m._run_web_search_pipeline, safe_company, ""),
        }
        cache_keys = {
            "company_research": build_cache_key(
                "company_research", safe_company, "指定なし", company_original=request.company_name
            ),
            "interview_hints": build_cache_key(
                "interview_hints", safe_company, "一般職", company_original=request.company_name
            ),
            "es_review": build_cache_key("es_review", safe_company, company_original=request.company_name),
        }

        results: List[VectorReembedResult] = []
        collections: Dict[str, int] = {}
        for target in _refresh_targets(doc_type):
            try:
                text = fetchers[target]()
                if not text:
                    results.append(VectorReembedResult(doc_type=target, status="unchanged"))
                    continue
                embeddings = m.embed_texts([text])
                outcome = replace_documents(
                    cache_keys[target], [text], embeddings, source="web_search", doc_type=target,
                    purge_doc_types=_purge_doc_types(doc_type, target),
                )
            except Exception as exc:
                logger.exception("vector reembed failed company=%s doc_type=%s error=%s", safe_company, target, exc)
                # 例外の本文は内部情報を含みうるので、種類だけを返す
                results.append(VectorReembedResult(doc_type=target, status="failed", error=type(exc).__name__))
                continue
            collections[outcome["collection"]] = collections.get(outcome["collection"], 0) + outcome["deleted"]
            results.append(VectorReembedResult(doc_type=target, status="replaced", deleted=outcome["deleted"]))

        if results and all(r.status == "failed" for r in results):
            raise HTTPException(status_code=502, detail="reembed refresh failed; existing documents were kept")

        replaced = [r.doc_type for r in results if r.status == "replaced"]
        return VectorReembedResponse(
            status="ok" if all(r.status != "failed" for r in results) else "partial",
            company=safe_company,
            deleted=sum(r.deleted for r in results),
            collections=collections,
            refreshed=bool(replaced),
            sources=replaced,
            results=results,
        )
    finally:
        lock.release()
