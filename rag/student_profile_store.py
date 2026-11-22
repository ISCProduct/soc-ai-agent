"""学生プロフィールのベクトルストア操作（#1094 スカウト向けセマンティック検索）。

企業向け学生検索で「リーダーシップ経験があってReactができる学生」のような
自然文クエリを扱うため、学生が公開に同意した範囲のテキストのみを
専用コレクションへ埋め込む。同意撤回・退会時は delete_student で必ず削除する。
"""
from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any, Dict, List

from vector_store import get_chroma_client

logger = logging.getLogger("rag-review")

COLLECTION_STUDENT_PROFILES = "student_profiles"


def _collection() -> Any:
    return get_chroma_client().get_or_create_collection(COLLECTION_STUDENT_PROFILES)


def _doc_id(user_id: int) -> str:
    """1学生1ドキュメント。upsert で常に最新のプロフィールへ置き換える。"""
    return f"student-{user_id}"


def upsert_student(user_id: int, text: str, embedding: List[float]) -> None:
    """学生プロフィールを登録・更新する。

    本文（text）は保存しない（SOCAIAGENT-695）。検索は埋め込みと user_id だけで足り、本文は読み戻さない。
    documents を省くと Chroma が既定の埋め込み関数を持ち出す実装があるため、空文字を明示して渡す。
    """
    collection = _collection()
    collection.upsert(
        ids=[_doc_id(user_id)],
        documents=[""],
        embeddings=[embedding],
        metadatas=[
            {
                "user_id": int(user_id),
                "indexed_at": datetime.now(timezone.utc).isoformat(),
            }
        ],
    )
    logger.info("student profile upserted user_id=%s chars=%d", user_id, len(text))


def delete_student(user_id: int) -> int:
    """学生のベクトルを削除する。存在しない場合も正常終了する（冪等）。"""
    collection = _collection()
    collection.delete(ids=[_doc_id(user_id)])
    logger.info("student profile deleted user_id=%s", user_id)
    return 1


def query_students(embedding: List[float], top_k: int, min_score: float = 0.0) -> List[Dict[str, Any]]:
    """クエリベクトルに近い学生を関連度順で返す。

    min_score より関連度の低い学生は返さない（SOCAIAGENT-568）。候補の少ないクエリでも
    top_k 件を無条件に返すと、無関係な学生が上位に並ぶ。
    """
    collection = _collection()
    if collection.count() == 0:
        return []
    # Chroma は n_results がコレクション件数を超えるとエラーになる実装があるため頭打ちにする。
    n_results = min(top_k, collection.count())
    result = collection.query(
        query_embeddings=[embedding],
        n_results=n_results,
        include=["metadatas", "distances"],
    )
    metadatas = (result.get("metadatas") or [[]])[0]
    distances = (result.get("distances") or [[]])[0]

    hits: List[Dict[str, Any]] = []
    for meta, distance in zip(metadatas, distances):
        user_id = (meta or {}).get("user_id")
        if user_id is None:
            continue
        # Chroma の距離は小さいほど近い。1/(1+d) で 0<score<=1 の関連度へ写す。
        score = 1.0 / (1.0 + float(distance))
        if score < min_score:
            continue
        hits.append({"user_id": int(user_id), "score": score})
    if len(hits) < len(metadatas):
        logger.info("student query kept %d/%d hits (min_score=%.3f)", len(hits), len(metadatas), min_score)
    return hits
