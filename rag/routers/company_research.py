"""企業研究AIの文章の登録・検索（SOCAIAGENT-725）。Backend からだけ呼ばれる（内部認証あり）。"""
from __future__ import annotations

import logging
import os
from typing import List, Optional

import tiktoken
from fastapi import APIRouter
from pydantic import BaseModel, Field

import company_research_store as store
from services.embed import embed_texts

logger = logging.getLogger("main")

router = APIRouter()

MAX_DOCUMENTS = 20
MAX_TOP_K = 20


class ResearchDocument(BaseModel):
    document_id: int = Field(gt=0)
    source_type: str = Field(min_length=1, max_length=32)
    title: str = Field(default="", max_length=255)
    url: str = Field(default="", max_length=500)
    as_of: Optional[str] = Field(default=None, max_length=10)  # YYYY-MM-DD
    body: str = Field(max_length=200_000)


class IndexRequest(BaseModel):
    company_id: int = Field(gt=0)
    documents: List[ResearchDocument] = Field(min_length=1, max_length=MAX_DOCUMENTS)


class QueryRequest(BaseModel):
    company_ids: List[int] = Field(min_length=1, max_length=2)  # 1社、比較なら2社
    question: str = Field(min_length=1, max_length=500)
    top_k: int = Field(default=6, gt=0, le=MAX_TOP_K)


def _usage(texts: List[str]) -> dict:
    """ベクトル化のトークン量。RAG は api_call_logs を持たないので Backend が記録する（ES 添削と同じ形）。"""
    model = os.getenv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small")
    try:
        enc = tiktoken.encoding_for_model(model)
    except KeyError:
        enc = tiktoken.get_encoding("cl100k_base")
    tokens = sum(len(enc.encode(t, disallowed_special=())) for t in texts)
    return {"model": model, "prompt_tokens": tokens, "completion_tokens": 0, "calls": 1 if texts else 0}


@router.post("/company-research/index")
def index_documents(req: IndexRequest) -> dict:
    chunked = [(d, store.split_text(d.body)) for d in req.documents]
    all_chunks = [c for _, chunks in chunked for c in chunks]
    vectors = embed_texts(all_chunks) if all_chunks else []
    pos = 0
    total = 0
    for d, chunks in chunked:
        total += store.replace_document(
            req.company_id,
            d.document_id,
            chunks,
            vectors[pos:pos + len(chunks)],
            {"source_type": d.source_type, "title": d.title, "url": d.url, "as_of": d.as_of},
        )
        pos += len(chunks)
    logger.info("company research indexed company_id=%s documents=%d chunks=%d", req.company_id, len(req.documents), total)
    return {"chunks": total, "usage": _usage(all_chunks)}


@router.post("/company-research/query")
def query_documents(req: QueryRequest) -> dict:
    vector = embed_texts([req.question])[0]
    hits = store.query(req.company_ids, vector, req.top_k)
    return {"hits": hits, "usage": _usage([req.question])}


@router.delete("/company-research/{company_id}")
def delete_company(company_id: int) -> dict:
    store.delete_company(company_id)
    return {"deleted": True}
