"""企業研究AIの文章のベクトルストア操作（SOCAIAGENT-725）。

Backend が company_documents（公的資料・公式サイトの文章）を送り、ここで分割してベクトル化する。
文章の正本は Backend の MySQL にあり、このコレクションは作り直せる（消えても取り直しは要らない）。
検索は必ず企業IDで絞る。絞り忘れると別の会社の文章で答えてしまう。
"""
from __future__ import annotations

import logging
import re
from datetime import datetime, timezone
from typing import Any, Dict, List

from vector_store import get_chroma_client, physical_collection_name

logger = logging.getLogger("rag-review")

COLLECTION_COMPANY_RESEARCH = "company_research"

# 1チャンクの目安の文字数と、前のチャンクとの重なり。有報の1節（数百〜数千字）を数個に分ける長さ。
CHUNK_CHARS = 600
CHUNK_OVERLAP = 100

_SENTENCE_END = re.compile(r"(?<=[。！？\n])")


def _collection() -> Any:
    # 距離は cosine（関連度を 1 - 距離 で読めるように）。既存コレクションと違い、作るときに指定する。
    return get_chroma_client().get_or_create_collection(
        physical_collection_name(COLLECTION_COMPANY_RESEARCH), metadata={"hnsw:space": "cosine"}
    )


def split_text(text: str, size: int = CHUNK_CHARS, overlap: int = CHUNK_OVERLAP) -> List[str]:
    """文の切れ目（。！？改行）で区切りながら、size 字前後のチャンクに分ける。

    1文が size を超えるときは機械的に切る。隣どうしは overlap 字重ねて、文脈が切れないようにする。
    """
    text = re.sub(r"[ \t　]+", " ", text or "").strip()
    if not text:
        return []
    sentences = [s for s in _SENTENCE_END.split(text) if s.strip()]
    chunks: List[str] = []
    buf = ""
    for s in sentences:
        while len(s) > size:  # 長すぎる1文
            if buf:
                chunks.append(buf)
                buf = ""
            chunks.append(s[:size])
            s = s[size - overlap:]
        if len(buf) + len(s) > size and buf:
            chunks.append(buf)
            buf = buf[-overlap:]
        buf += s
    if buf.strip():
        chunks.append(buf)
    return [c.strip() for c in chunks if c.strip()]


def replace_document(company_id: int, document_id: int, chunks: List[str], embeddings: List[List[float]], meta: Dict[str, Any]) -> int:
    """1文書のチャンクを置き換える（古いチャンクを消してから入れる）。入れたチャンク数を返す。"""
    collection = _collection()
    collection.delete(where={"document_id": int(document_id)})
    if not chunks:
        return 0
    now = datetime.now(timezone.utc).isoformat()
    collection.add(
        ids=[f"doc-{document_id}-{i}" for i in range(len(chunks))],
        documents=chunks,
        embeddings=embeddings,
        metadatas=[
            {
                **{k: v for k, v in meta.items() if v is not None},
                "company_id": int(company_id),
                "document_id": int(document_id),
                "chunk_index": i,
                "indexed_at": now,
            }
            for i in range(len(chunks))
        ],
    )
    return len(chunks)


def delete_company(company_id: int) -> None:
    """企業のチャンクをすべて消す（停止・削除した企業）。存在しなくても正常終了する。"""
    _collection().delete(where={"company_id": int(company_id)})


def query(company_ids: List[int], embedding: List[float], top_k: int) -> List[Dict[str, Any]]:
    """企業IDで絞って、質問に近いチャンクを関連度順に返す。"""
    if not company_ids:
        return []
    ids = [int(c) for c in company_ids]
    where: Dict[str, Any] = {"company_id": ids[0]} if len(ids) == 1 else {"company_id": {"$in": ids}}
    collection = _collection()
    result = collection.query(
        query_embeddings=[embedding],
        n_results=top_k,
        where=where,
        include=["documents", "metadatas", "distances"],
    )
    docs = (result.get("documents") or [[]])[0]
    metas = (result.get("metadatas") or [[]])[0]
    dists = (result.get("distances") or [[]])[0]
    hits: List[Dict[str, Any]] = []
    for text, meta, dist in zip(docs, metas, dists):
        meta = meta or {}
        # 念のため企業IDをもう一度確かめる（where の書き間違いで他社の文章が混ざらないように）
        if int(meta.get("company_id", -1)) not in ids:
            continue
        hits.append({"text": text, "score": 1.0 - float(dist), **meta})
    return hits
