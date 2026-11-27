"""学生プロフィールのベクトルストア操作（#1094 スカウト向けセマンティック検索）。

企業向け学生検索で「リーダーシップ経験があってReactができる学生」のような
自然文クエリを扱うため、学生が公開に同意した範囲のテキストのみを
専用コレクションへ埋め込む。同意撤回・退会時は delete_student で必ず削除する。
"""
from __future__ import annotations

import logging
from datetime import datetime, timezone
from typing import Any, Dict, List

from vector_store import COLLECTION_METADATA, collection_space, get_chroma_client, to_cosine_distance

logger = logging.getLogger("rag-review")

COLLECTION_STUDENT_PROFILES = "student_profiles"
# cosine へ切り替える前の l2 のコレクション（SOCAIAGENT-554。scripts/rebuild_collections.py --student-profiles が作る）。
# 切り戻し用に7日残すので、その間の削除（同意撤回・退会）はこちらにも反映する
COLLECTION_STUDENT_PROFILES_OLD = "student_profiles__l2old554"


def _collection() -> Any:
    # 新しく作るときは cosine（SOCAIAGENT-554）。既存の l2 のコレクションは l2 のまま開く
    return get_chroma_client().get_or_create_collection(COLLECTION_STUDENT_PROFILES, metadata=dict(COLLECTION_METADATA))


def relevance_score(distance: float, space: str) -> float:
    """距離を 0<score<=1 の関連度へ写す。距離関数（l2 / cosine）によらず、同じ2つの埋め込みなら同じ値にする。

    以前は l2（ユークリッド距離の2乗 d）のコレクションで 1/(1+d) にしていた。長さ1の埋め込みでは d = 2 × cosine 距離なので、
    1/(1 + 2 × cosine 距離) にすると、l2 のコレクションでは今までと同じ値になり、cosine のコレクションへ切り替えても
    関連度の下限（STUDENT_SEARCH_MIN_SCORE、既定 0.38）の意味が変わらない。
    """
    return 1.0 / (1.0 + 2.0 * to_cosine_distance(distance, space))


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
    """学生のベクトルを削除する。存在しない場合も正常終了する（冪等）。

    cosine へ切り替える前のコレクション（切り戻し用に残しているもの）があれば、そこからも消す。
    消さないと、切り戻したときに同意撤回・退会した学生が検索に戻る（SOCAIAGENT-554）。
    """
    collection = _collection()
    collection.delete(ids=[_doc_id(user_id)])
    _delete_from_old(user_id)
    logger.info("student profile deleted user_id=%s", user_id)
    return 1


def _delete_from_old(user_id: int) -> None:
    """切り替え前のコレクションからも消す。無ければ何もしない。"""
    client = get_chroma_client()
    try:
        old = client.get_collection(COLLECTION_STUDENT_PROFILES_OLD)
    except Exception:
        return  # 切り替え前のコレクションが無い（切り替えていない・消し終わった）
    old.delete(ids=[_doc_id(user_id)])


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
    space = collection_space(collection)

    hits: List[Dict[str, Any]] = []
    for meta, distance in zip(metadatas, distances):
        user_id = (meta or {}).get("user_id")
        if user_id is None:
            continue
        # Chroma の距離は小さいほど近い。距離関数によらない関連度へ写す
        score = relevance_score(float(distance), space)
        if score < min_score:
            continue
        hits.append({"user_id": int(user_id), "score": score})
    if len(hits) < len(metadatas):
        logger.info("student query kept %d/%d hits (min_score=%.3f)", len(hits), len(metadatas), min_score)
    return hits
