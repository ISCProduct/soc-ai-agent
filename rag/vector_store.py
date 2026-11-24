"""ベクトルストア（Chroma）クライアントと企業単位コレクション操作。

#573: 本番では CHROMA_HOST 経由の HttpClient、未設定時は PersistentClient にフォールバックする。
"""
from __future__ import annotations

import hashlib
import logging
import os
import re
import threading
import unicodedata
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Sequence, Tuple

import chromadb

logger = logging.getLogger("rag-review")

DEFAULT_CHROMA_DATA_DIR = "/app/chroma_db"
CHROMA_DATA_DIR = os.getenv("RAG_CHROMA_DATA_DIR", DEFAULT_CHROMA_DATA_DIR)
CHROMA_HOST = os.getenv("CHROMA_HOST", "").strip()
CHROMA_PORT = int(os.getenv("CHROMA_PORT", "8000"))
CACHE_TTL_SECONDS = int(os.getenv("RAG_SEARCH_CACHE_TTL_SECONDS", "86400"))

COLLECTION_COMPANY_CONTEXT = "company_context"
COLLECTION_INTERVIEW_HINTS = "interview_hints"
COLLECTION_ES_REVIEW = "es_review"

ALL_COLLECTIONS = (
    COLLECTION_COMPANY_CONTEXT,
    COLLECTION_INTERVIEW_HINTS,
    COLLECTION_ES_REVIEW,
)

DOC_TYPE_TO_COLLECTION = {
    "resume_review": COLLECTION_COMPANY_CONTEXT,
    "company_research": COLLECTION_COMPANY_CONTEXT,
    "interview_hints": COLLECTION_INTERVIEW_HINTS,
    "es_review": COLLECTION_ES_REVIEW,
}

_chroma_client: Any = None
_chroma_lock = threading.Lock()
# 文書の書き込み（set_cached_documents / replace_documents の upsert）と、backfill の
# 「読み直し → fetched_at_ts の update」を同じプロセスの中で重ねないためのロック（SOCAIAGENT-659）。
# backfill が読んだあとに新しい本文が書かれ、そこへ古い時刻を上書きしないようにする。
_write_lock = threading.RLock()


def reset_chroma_client_for_tests() -> None:
    """テスト用にシングルトンをクリアする。"""
    global _chroma_client
    with _chroma_lock:
        _chroma_client = None


def _collection_names(client: Any) -> set:
    """Chroma 0.6+ では list_collections が名前文字列のみ返す場合がある。"""
    names: set = set()
    for col in client.list_collections():
        if isinstance(col, str):
            names.add(col)
        else:
            name = getattr(col, "name", "") or ""
            if name:
                names.add(name)
    return names


def get_chroma_client() -> Any:
    """Chroma クライアントを返す。CHROMA_HOST があれば HttpClient。"""
    global _chroma_client
    if _chroma_client is None:
        with _chroma_lock:
            if _chroma_client is None:
                if CHROMA_HOST:
                    logger.info(
                        "chroma mode=http host=%s port=%d", CHROMA_HOST, CHROMA_PORT
                    )
                    _chroma_client = chromadb.HttpClient(host=CHROMA_HOST, port=CHROMA_PORT)
                else:
                    logger.info("chroma mode=persistent path=%s", CHROMA_DATA_DIR)
                    _chroma_client = chromadb.PersistentClient(path=CHROMA_DATA_DIR)
    return _chroma_client


def _sanitize_collection_name(name: str) -> str:
    """chromadb のコレクション名制約に合わせてサニタイズする (3-63文字, 英数字/_/-)。"""
    cleaned = re.sub(r"[^a-zA-Z0-9_-]", "_", name)
    cleaned = re.sub(r"^[^a-zA-Z0-9]+", "", cleaned)
    cleaned = re.sub(r"[^a-zA-Z0-9]+$", "", cleaned)
    if len(cleaned) < 3:
        cleaned = cleaned.ljust(3, "x")
    return cleaned[:63]


def collection_suffix() -> str:
    """コレクションの世代の接尾辞（RAG_COLLECTION_SUFFIX、SOCAIAGENT-658）。空なら接尾辞なし（今までの名前）。

    全件再構築は、新しい世代のコレクションへ作ってから、この値を切り替えて使う
    （scripts/rebuild_collections.py）。再構築の間も旧世代で検索できる。
    """
    return re.sub(r"[^a-zA-Z0-9_-]", "", os.getenv("RAG_COLLECTION_SUFFIX", "").strip())


def physical_collection_name(name: str, suffix: Optional[str] = None) -> str:
    """論理名（company_context など）に世代の接尾辞を付けた、Chroma 上の実際の名前。"""
    suffix = collection_suffix() if suffix is None else suffix
    return _sanitize_collection_name(f"{name}_{suffix}" if suffix else name)


def _company_hash(company_original: Optional[str]) -> str:
    """サニタイズ前の企業名をNFKC正規化してSHA-256ハッシュを返す (#938)。

    `_sanitize_company_name_for_query` は許可文字以外を区切りなしで単純除去するため、
    表記の異なる企業名（例: "H.I.S." と "HIS"）が同一のサニタイズ結果へ収束しうる。
    サニタイズ前の原文をハッシュ化してキャッシュキーへ付与することで、
    表示用の企業名が衝突してもキャッシュキー自体は分岐させる。
    company_original が空/未指定の場合は空文字を返し、従来のキー形式を維持する。
    """
    normalized = unicodedata.normalize("NFKC", (company_original or "").strip())
    if not normalized:
        return ""
    return hashlib.sha256(normalized.encode("utf-8")).hexdigest()


def _company_segment(company: str, company_original: Optional[str] = None) -> str:
    """キャッシュキーの企業セグメント（表示用企業名 [+ ~ハッシュ]）を組み立てる。"""
    company = (company or "unknown").strip() or "unknown"
    company_hash = _company_hash(company_original)
    return f"{company}~{company_hash}" if company_hash else company


def _split_company_segment(segment: str) -> Tuple[str, str]:
    """企業セグメントを (表示用企業名, ハッシュ) に分解する。ハッシュ無しの旧キーは空文字。"""
    if "~" in segment:
        company, company_hash = segment.split("~", 1)
        return company or "unknown", company_hash
    return segment or "unknown", ""


def parse_cache_key(cache_key: str) -> Dict[str, str]:
    """旧キャッシュキーをメタデータに分解する。

    例:
      hints::Acme~ab12cd34ef::エンジニア → interview_hints / company=Acme / company_hash=ab12cd34ef / role=エンジニア
      Acme~ab12cd34ef::es_review → es_review
      Acme::一般職 → company_context（ハッシュ無しの旧形式。company_hash=""）
    """
    key = (cache_key or "").strip()
    if key.startswith("hints::"):
        parts = key.split("::", 2)
        company_segment = parts[1] if len(parts) > 1 else ""
        role = parts[2] if len(parts) > 2 else "general"
        company, company_hash = _split_company_segment(company_segment)
        return {
            "collection": COLLECTION_INTERVIEW_HINTS,
            "doc_type": "interview_hints",
            "company": company or "unknown",
            "company_hash": company_hash,
            "role": role or "general",
            "cache_key": key,
        }

    parts = key.split("::", 1)
    company_segment = parts[0] if parts else "unknown"
    role = parts[1] if len(parts) > 1 else "general"
    company, company_hash = _split_company_segment(company_segment)
    if role == "es_review":
        return {
            "collection": COLLECTION_ES_REVIEW,
            "doc_type": "es_review",
            "company": company or "unknown",
            "company_hash": company_hash,
            "role": "general",
            "cache_key": key,
        }
    # デフォルトは企業調査。履歴書レビューは呼び出し側で doc_type を上書きする。
    return {
        "collection": COLLECTION_COMPANY_CONTEXT,
        "doc_type": "company_research",
        "company": company or "unknown",
        "company_hash": company_hash,
        "role": role or "general",
        "cache_key": key,
    }


def build_cache_key(
    doc_type: str,
    company: str,
    role: str = "general",
    company_original: Optional[str] = None,
) -> str:
    """doc_type / company / role から互換キャッシュキーを組み立てる。

    company_original（サニタイズ前の企業名）を渡すと、そのハッシュを企業セグメントに
    付与し、表記違いの企業名が同一キーに収束する衝突（#938）を防ぐ。
    """
    company_segment = _company_segment(company, company_original)
    role = (role or "general").strip() or "general"
    if doc_type == "interview_hints":
        return f"hints::{company_segment}::{role}"
    if doc_type == "es_review":
        return f"{company_segment}::es_review"
    return f"{company_segment}::{role}"


# 取得時刻が未来にずれていてよい幅（ホスト間の時計のずれ）。これを超える未来の日付は壊れているとみなす。
_FETCHED_AT_FUTURE_TOLERANCE_SECONDS = 300


def _parse_fetched_at(value: Any) -> Optional[datetime]:
    """fetched_at（ISO 8601）を UTC の datetime にする。読めなければ None。

    タイムゾーンの無い日時は UTC とみなす（書き込みは常に UTC で、コンテナも UTC）。
    TTL の判定（_is_fresh）と backfill の両方がこれを使い、扱いを食い違わせない。
    """
    if not value:
        return None
    try:
        ts = datetime.fromisoformat(str(value).replace("Z", "+00:00"))
        if ts.tzinfo is None:
            ts = ts.replace(tzinfo=timezone.utc)
        return ts.astimezone(timezone.utc)
    except (ValueError, TypeError, OverflowError, OSError):
        return None


def _now_ts() -> int:
    """今の UNIX 秒（整数）。テストで時刻を固定できるよう1か所にする。"""
    return int(datetime.now(timezone.utc).timestamp())


def _ttl_window(now_ts: int, ttl_seconds: int) -> Tuple[int, int]:
    """TTL 内とみなす fetched_at_ts の範囲（両端を含む）。

    下限は「今 - TTL」、上限は時計のずれの許容ぶんだけ未来（これを超える未来は壊れた文書）。
    検索の where（_with_ttl）と、検索で取れた文書の後段の確認（_doc_is_fresh）の両方がこれを使う。
    """
    return now_ts - ttl_seconds, now_ts + _FETCHED_AT_FUTURE_TOLERANCE_SECONDS


def _has_numeric_ts(meta: Dict[str, Any]) -> bool:
    value = meta.get("fetched_at_ts")
    return isinstance(value, (int, float)) and not isinstance(value, bool)


def fetched_at_to_ts(fetched_at: Any, now: Optional[datetime] = None) -> Optional[int]:
    """fetched_at を UNIX 秒にする。読めない・ありえない未来のときは None（補わない）。"""
    ts = _parse_fetched_at(fetched_at)
    if ts is None:
        return None
    now = now or datetime.now(timezone.utc)
    if ts.timestamp() - now.timestamp() > _FETCHED_AT_FUTURE_TOLERANCE_SECONDS:
        return None
    return int(ts.timestamp())


def _is_fresh(fetched_at: Optional[str], ttl_seconds: int = CACHE_TTL_SECONDS) -> bool:
    """キャッシュ文書が TTL 内かを、fetched_at（ISO 8601）で返す。

    fetched_at が読めない・ありえない未来の日付のときは古い扱い（False）にする（SOCAIAGENT-567）。
    新鮮扱いにすると TTL で二度と失効せず、古い企業情報がキャッシュに残り続ける。

    fetched_at が無い（取得時刻を記録する前の文書）ときは新鮮扱いにする。ただしこれは
    検索で取れた文書の後段の確認で、検索そのものは _with_ttl が fetched_at_ts で絞るので、
    fetched_at_ts を持たない文書は検索に当たらず古い扱いになる（SOCAIAGENT-659 の U1）。
    fetched_at_ts を持つ文書は、後段でも fetched_at_ts を where と同じ範囲で確かめる（_doc_is_fresh）。
    ここが効くのは、RAG_TTL_WHERE=0 で where を切ったときの、fetched_at_ts を持たない文書だけ。
    """
    if not fetched_at or ttl_seconds <= 0:
        return True
    ts = _parse_fetched_at(fetched_at)
    if ts is None:
        logger.warning("cache document has unreadable fetched_at=%r; treating as stale", fetched_at)
        return False
    age = (datetime.now(timezone.utc) - ts).total_seconds()
    if age < -_FETCHED_AT_FUTURE_TOLERANCE_SECONDS:
        logger.warning("cache document has future fetched_at=%r; treating as stale", fetched_at)
        return False
    return age <= ttl_seconds


def _doc_is_fresh(meta: Dict[str, Any], ttl_seconds: int = CACHE_TTL_SECONDS) -> bool:
    """検索で取れた文書の後段の確認。fetched_at_ts があれば where と同じ整数の範囲で判定する
    （fetched_at の小数を含む秒数で判定すると、TTL ちょうどの境界で where と1秒未満ずれる）。"""
    if ttl_seconds <= 0:
        return True
    if _has_numeric_ts(meta):
        low, high = _ttl_window(_now_ts(), ttl_seconds)
        return low <= meta["fetched_at_ts"] <= high
    return _is_fresh(meta.get("fetched_at"), ttl_seconds)


def _doc_ids(cache_key: str, count: int, doc_type: str = "") -> List[str]:
    """文書の ID。cache_key と用途（doc_type）から作る（SOCAIAGENT-659）。

    resume_review と company_research は、職種が同じなら cache_key も同じで、同じコレクションに並ぶ。
    以前は cache_key だけから ID を作っていたので、片方を書くともう片方の同じ番号のチャンクを上書きし、
    別の用途の断片が残っていた。doc_type を含めて、用途ごとに別の文書にする。
    doc_type を含めない旧 ID の文書は、次の書き込みの余りの掃除・再取得の削除で消える。
    """
    digest = hashlib.sha256(f"{doc_type}\x00{cache_key}".encode("utf-8")).hexdigest()[:16]
    return [f"{digest}_{i}" for i in range(count)]


def _company_filter(meta: Dict[str, str], *, role: Optional[str] = None) -> Dict[str, str]:
    """meta（parse_cache_keyの戻り値）から company [+ company_hash] [+ role] のフィルタ辞書を作る。

    company_hash が空（ハッシュ無しの旧キー）の場合は company のみで従来通り絞り込む。
    company_hash がある場合は必ず含めて、表記違いの企業名がwhere句で衝突しないようにする(#938)。
    """
    filters: Dict[str, str] = {"company": meta["company"]}
    if meta.get("company_hash"):
        filters["company_hash"] = meta["company_hash"]
    if role is not None:
        filters["role"] = role
    return filters


def get_cached_documents(
    cache_key: str,
    query_embedding: List[float],
    n_results: int = 5,
    allow_company_fallback: bool = True,
) -> List[str]:
    """企業単位コレクションから類似ドキュメントを取得する。"""
    meta = parse_cache_key(cache_key)
    collection_name = physical_collection_name(meta["collection"])
    try:
        client = get_chroma_client()
        existing = _collection_names(client)
        if collection_name not in existing:
            logger.info(
                "chromadb cache miss key=%s collection=%s reason=collection_not_found",
                cache_key,
                collection_name,
            )
            return []

        collection = client.get_collection(collection_name)
        if collection.count() == 0:
            return []

        docs = _query_with_where(
            collection,
            query_embedding,
            where=_build_where(_company_filter(meta, role=meta["role"])),
            n_results=n_results,
            # allow_company_fallback=Falseの場合、例外時の再試行でも職種条件を
            # 落とさない(会社単独スコープへの意図しないフォールバックを防ぐ)
            fallback_where=(
                _build_where(_company_filter(meta)) if allow_company_fallback else None
            ),
        )
        if docs:
            logger.info(
                "chromadb cache hit key=%s collection=%s mode=company_role docs=%d",
                cache_key,
                collection_name,
                len(docs),
            )
            return docs

        # 職種ミス時は同一企業の任意 role を再利用（#510 / Phase 3 横断）
        if allow_company_fallback:
            docs = _query_with_where(
                collection,
                query_embedding,
                where=_build_where(_company_filter(meta)),
                n_results=n_results,
            )
            if docs:
                logger.info(
                    "chromadb cache hit key=%s collection=%s mode=company_fallback docs=%d",
                    cache_key,
                    collection_name,
                    len(docs),
                )
                return docs

        logger.info("chromadb cache miss key=%s collection=%s", cache_key, collection_name)
        return []
    except Exception as exc:
        logger.exception("chromadb get failed key=%s error=%s", cache_key, exc)
        return []


def _build_where(filters: Dict[str, str]) -> Dict[str, Any]:
    """ChromaDBはフラットな複数キーのwhereを許可しない(1オペレータのみ)ため、
    2キー以上ある場合は$andでラップする。1キー以下ならそのまま返す。"""
    if len(filters) <= 1:
        return dict(filters)
    return {"$and": [{k: v} for k, v in filters.items()]}


def ttl_where_enabled() -> bool:
    """RAG_TTL_WHERE=0 なら TTL を where で絞らない（後段の _is_fresh だけにする。切り戻し用）。"""
    return os.getenv("RAG_TTL_WHERE", "1").strip().lower() not in ("0", "false", "off")


def _with_ttl(where: Dict[str, Any], ttl_seconds: int = CACHE_TTL_SECONDS) -> Dict[str, Any]:
    """where に「取得時刻（fetched_at_ts）が TTL 内」の条件を足す（SOCAIAGENT-659）。

    以前は n_results 件を取ってから期限切れを捨てていたので、期限切れの文書が類似度の上位を占めると、
    有効な文書があっても返らなかった。検索の条件で絞れば、有効な文書が n_results 件まで返る。
    fetched_at_ts（数値）が無い文書は条件に当たらず、古い扱いになる（既存の文書は rag-review の
    起動時の backfill（backfill_fetched_at_ts）が補う）。
    """
    if ttl_seconds <= 0 or not ttl_where_enabled():
        return where
    low, high = _ttl_window(_now_ts(), ttl_seconds)
    # 時計のずれを超える未来の取得時刻も壊れた文書として外す。
    # Chroma は1つの条件に演算子を1つしか持てないので、下限と上限を別の条件にする
    ttl_clauses = [
        {"fetched_at_ts": {"$gte": low}},
        {"fetched_at_ts": {"$lte": high}},
    ]
    if not where:
        return {"$and": ttl_clauses}
    if "$and" in where:
        return {"$and": [*where["$and"], *ttl_clauses]}
    return {"$and": [where, *ttl_clauses]}


def _query_with_where(
    collection: Any,
    query_embedding: List[float],
    where: Dict[str, Any],
    n_results: int,
    fallback_where: Optional[Dict[str, Any]] = None,
) -> List[str]:
    def _run(w: Dict[str, Any]) -> Any:
        return collection.query(
            query_embeddings=[query_embedding],
            n_results=n_results,
            where=_with_ttl(w),
            include=["documents", "metadatas"],
        )

    try:
        results = _run(where)
    except Exception as first_exc:
        # whereフィルタが失敗した場合でも、他社のデータが混入しないよう
        # fallback_where(通常はcompanyのみ)でスコープを維持したまま再試行する。
        # スコープを維持できない場合は、フィルタ無し検索(他社データ混入の
        # リスクがある)は行わずキャッシュミス扱いにする。
        # 以前は失敗を黙って握りつぶしていたので、キャッシュミスの原因が追えなかった（SOCAIAGENT-659）。
        if not fallback_where:
            logger.warning(
                "chromadb query failed; treated as cache miss where_keys=%s error=%s", _where_keys(where), first_exc
            )
            return []
        try:
            results = _run(fallback_where)
        except Exception as exc:
            logger.warning(
                "chromadb query failed twice; treated as cache miss where_keys=%s fallback_keys=%s error=%s",
                _where_keys(where), _where_keys(fallback_where), exc,
            )
            return []
        logger.info(
            "chromadb query failed; retried with fallback where_keys=%s fallback_keys=%s error=%s",
            _where_keys(where), _where_keys(fallback_where), first_exc,
        )

    documents: List[str] = results.get("documents", [[]])[0] or []
    metadatas: List[Dict[str, Any]] = results.get("metadatas", [[]])[0] or []
    fresh_docs: List[str] = []
    for doc, md in zip(documents, metadatas):
        if not doc:
            continue
        if _doc_is_fresh(md or {}):
            fresh_docs.append(doc)
    return fresh_docs


def _where_keys(where: Optional[Dict[str, Any]]) -> List[str]:
    """ログ用に where の項目名だけを返す（値は出さない）。"""
    if not where:
        return []
    keys: List[str] = []
    for key, value in where.items():
        if key in ("$and", "$or") and isinstance(value, list):
            for clause in value:
                keys.extend(_where_keys(clause))
        else:
            keys.append(key)
    return sorted(set(keys))


def set_cached_documents(
    cache_key: str,
    docs: List[str],
    embeddings: List[List[float]],
    source: str = "unknown",
    doc_type: Optional[str] = None,
) -> None:
    """ドキュメントを固定コレクションへ upsert する。"""
    if not docs or not embeddings or len(docs) != len(embeddings):
        return
    meta = parse_cache_key(cache_key)
    if doc_type:
        meta["doc_type"] = doc_type
        mapped = DOC_TYPE_TO_COLLECTION.get(doc_type)
        if mapped:
            meta["collection"] = mapped
    collection_name = physical_collection_name(meta["collection"])
    now = datetime.now(timezone.utc)
    fetched_at = now.isoformat()
    try:
        client = get_chroma_client()
        collection = client.get_or_create_collection(collection_name)
        ids = _doc_ids(cache_key, len(docs), meta["doc_type"])
        metadatas = [
            {
                "company": meta["company"],
                "company_hash": meta.get("company_hash", ""),
                "role": meta["role"],
                "doc_type": meta["doc_type"],
                "source": source,
                "fetched_at": fetched_at,
                # TTL を検索の where で絞るための数値（SOCAIAGENT-659）
                "fetched_at_ts": int(now.timestamp()),
                "cache_key": meta["cache_key"],
            }
            for _ in docs
        ]
        with _write_lock:
            collection.upsert(
                ids=ids,
                documents=docs,
                embeddings=embeddings,
                metadatas=metadatas,
            )
        logger.info(
            "chromadb upsert key=%s collection=%s docs=%d source=%s doc_type=%s",
            cache_key,
            collection_name,
            len(docs),
            source,
            meta["doc_type"],
        )
    except Exception as exc:
        logger.exception("chromadb set failed key=%s error=%s", cache_key, exc)
        return
    # 書き込みは済んでいるので、余りの掃除の失敗は「書き込みの失敗」とは別に記録する
    try:
        _delete_stale_chunks(collection, meta, set(ids), fetched_at)
    except Exception as exc:
        logger.warning("chromadb stale cleanup failed key=%s collection=%s error=%s", cache_key, collection_name, exc)


def _delete_stale_chunks(collection: Any, meta: Dict[str, str], written: set, fetched_at: str) -> None:
    """件数が減る更新（3チャンク → 2チャンク）で、前回の余ったチャンクを消す（SOCAIAGENT-659）。

    同じ cache_key・同じ用途（doc_type）で、今回書かなかった ID のうち、今回より前に書かれたもの
    （fetched_at が今回より古い・無い）だけを消す。fetched_at は常に UTC の isoformat で書くので、
    文字列の大小がそのまま時刻の前後になる（無い文書は空文字で、いちばん古い扱い）。

    今回より後に書かれた別のリクエストのチャンクは消さない。ただし同じキーを同時に書いたときの
    結果は「最後に書いたものが勝つ」で、両方の書き込みがきれいに残ることまでは保証しない。
    """
    stale_where = _build_where({"cache_key": meta["cache_key"], "doc_type": meta["doc_type"]})
    got = collection.get(where=stale_where, include=["metadatas"])
    stale = [
        doc_id
        for doc_id, m in zip(got.get("ids") or [], got.get("metadatas") or [])
        if doc_id not in written and str((m or {}).get("fetched_at") or "") < fetched_at
    ]
    if stale:
        collection.delete(ids=stale)


def upsert_by_doc_type(
    company: str,
    role: str,
    doc_type: str,
    docs: List[str],
    embeddings: List[List[float]],
    source: str = "unknown",
    company_original: Optional[str] = None,
) -> str:
    """company / role / doc_type で統一書き込みし、使用したキャッシュキーを返す。"""
    cache_key = build_cache_key(doc_type, company, role, company_original=company_original)
    set_cached_documents(
        cache_key, docs, embeddings, source=source, doc_type=doc_type
    )
    return cache_key


def delete_company_documents(
    company: str,
    doc_type: Optional[str] = None,
    company_original: Optional[str] = None,
) -> Dict[str, Any]:
    """企業に紐づくベクトルを削除する。doc_type 指定時はその用途のみ。

    company_original を渡すと保存時と同じ company_hash で絞り、
    サニタイズ後の表示名が衝突する別企業の文書を消さない。
    """
    company = (company or "").strip()
    if not company:
        return {"deleted": 0, "collections": {}}

    collections = ALL_COLLECTIONS
    if doc_type:
        mapped = DOC_TYPE_TO_COLLECTION.get(doc_type)
        collections = (mapped,) if mapped else ()

    client = get_chroma_client()
    existing = _collection_names(client)
    deleted_total = 0
    per_collection: Dict[str, int] = {}
    filters = _company_filter({"company": company, "company_hash": _company_hash(company_original)})
    if doc_type:
        # company_context は resume_review と company_research が共有している。doc_type を指定したら
        # その用途だけを消す（以前はコレクションごと企業の文書を消していた。SOCAIAGENT-659）。
        # doc_type を持たない旧文書は消さない（U2）
        filters["doc_type"] = doc_type
    where = _build_where(filters)

    for name in collections:
        physical = physical_collection_name(name)
        if physical not in existing:
            per_collection[name] = 0
            continue
        collection = client.get_collection(physical)
        try:
            before = collection.count()
            try:
                collection.delete(where=where)
            except Exception:
                got = collection.get(where=where, include=[])
                ids = got.get("ids") or []
                if ids:
                    collection.delete(ids=ids)
            after = collection.count()
            removed = max(0, before - after)
        except Exception as exc:
            logger.warning("chromadb delete failed collection=%s company=%s err=%s", name, company, exc)
            removed = 0
        per_collection[name] = removed
        deleted_total += removed

    return {"deleted": deleted_total, "collections": per_collection}


class ReplaceDocumentsError(RuntimeError):
    """書いた文書を確かめられなかった（旧データは消していない）。"""


def replace_documents(
    cache_key: str,
    docs: List[str],
    embeddings: List[List[float]],
    source: str = "unknown",
    doc_type: Optional[str] = None,
    purge_doc_types: Optional[Sequence[str]] = None,
) -> Dict[str, Any]:
    """企業の文書を「新しく書く → 書けたことを確かめる → 不要な旧文書を消す」の順で置き換える（SOCAIAGENT-658）。

    以前の再埋め込みは「削除 → 再取得」の順で、再取得が例外・空だと企業データが空のまま残った。
    ここでは新しい文書が全部読めることを確かめるまで、旧文書を消さない。失敗したら例外を投げ、
    旧文書はそのまま残る（set_cached_documents のように例外を握りつぶさない）。

    消すのは、同じコレクションにある同じ企業（company と company_hash で絞る）・同じ用途（doc_type）の
    文書のうち、今回書いた id 以外のもの。役割（role）の違う旧文書もここで消すので、以前の
    「その用途の企業の文書を全部消してから書く」と同じ結果になる。ただし消すのは書けた後だけ。
    company_context は resume_review と company_research が共有しているので、用途の違う文書は消さない。
    doc_type を持たない同じ企業の旧文書は消す（SOCAIAGENT-659 のレビュー）。残すと、backfill で
    fetched_at_ts が付いたあと、検索（会社と TTL で絞り、doc_type では絞らない）が再取得前の旧文書を返すため。

    purge_doc_types を渡すと、消す用途をその集合にする（既定は doc_type だけ）。再埋め込みは
    resume_review を company_research として取り直すので、両方を渡して resume_review の旧文書も入れ替える。
    戻り値: {"collection": 論理名, "replaced": 書いた件数, "deleted": 消した件数}
    """
    if not docs or not embeddings or len(docs) != len(embeddings):
        raise ValueError("docs と embeddings は同じ件数で、空でないこと")
    meta = parse_cache_key(cache_key)
    logical = DOC_TYPE_TO_COLLECTION.get(doc_type or "", meta["collection"])
    meta["doc_type"] = doc_type or meta["doc_type"]
    client = get_chroma_client()
    collection = client.get_or_create_collection(physical_collection_name(logical))
    ids = _doc_ids(cache_key, len(docs), meta["doc_type"])
    now = datetime.now(timezone.utc)
    fetched_at = now.isoformat()
    with _write_lock:
        collection.upsert(
            ids=ids,
            documents=docs,
            embeddings=embeddings,
            metadatas=[
                {
                    "company": meta["company"],
                    "company_hash": meta.get("company_hash", ""),
                    "role": meta["role"],
                    "doc_type": meta["doc_type"],
                    "source": source,
                    "fetched_at": fetched_at,
                    "fetched_at_ts": int(now.timestamp()),
                    "cache_key": meta["cache_key"],
                }
                for _ in docs
            ],
        )
    written = collection.get(ids=ids, include=[]).get("ids") or []
    if len(set(written)) != len(ids):
        raise ReplaceDocumentsError(
            f"written documents not found collection={logical} expected={len(ids)} found={len(written)}"
        )
    # Chroma の where では「doc_type が無い」を条件にできないので、同じ企業の文書を読み、
    # 消す用途の文書と doc_type を持たない旧文書を選ぶ
    where = _build_where(_company_filter(
        {"company": meta["company"], "company_hash": meta.get("company_hash", "")}
    ))
    purge = {t for t in (purge_doc_types or [meta.get("doc_type") or ""]) if t}
    got = collection.get(where=where, include=["metadatas"])
    written_ids = set(ids)
    stale = [
        doc_id
        for doc_id, m in zip(got.get("ids") or [], got.get("metadatas") or [])
        if doc_id not in written_ids and (str((m or {}).get("doc_type") or "") in purge or not (m or {}).get("doc_type"))
    ]
    if stale:
        collection.delete(ids=stale)
    logger.info(
        "chromadb replace key=%s collection=%s replaced=%d deleted=%d",
        cache_key, logical, len(ids), len(stale),
    )
    return {"collection": logical, "replaced": len(ids), "deleted": len(stale)}


def get_index_status(
    company: Optional[str] = None,
    company_original: Optional[str] = None,
) -> Dict[str, Any]:
    """インデックス状況（コレクション件数・任意で企業フィルタ概要）を返す。"""
    client = get_chroma_client()
    existing = _collection_names(client)
    collections_out: List[Dict[str, Any]] = []

    for name in ALL_COLLECTIONS:
        physical = physical_collection_name(name)
        info: Dict[str, Any] = {"name": name, "exists": physical in existing, "count": 0}
        if physical not in existing:
            collections_out.append(info)
            continue
        collection = client.get_collection(physical)
        info["count"] = collection.count()
        if company:
            where = _build_where(
                _company_filter({"company": company, "company_hash": _company_hash(company_original)})
            )
            try:
                try:
                    got = collection.get(
                        where=where,
                        include=["metadatas"],
                        limit=20,
                    )
                except TypeError:
                    got = collection.get(
                        where=where,
                        include=["metadatas"],
                    )
                metas = (got.get("metadatas") or [])[:20]
                info["company_count"] = len(metas)
                sources = sorted({
                    str((m or {}).get("source") or "")
                    for m in metas
                    if (m or {}).get("source")
                })
                doc_types = sorted({
                    str((m or {}).get("doc_type") or "")
                    for m in metas
                    if (m or {}).get("doc_type")
                })
                fetched = [
                    str((m or {}).get("fetched_at") or "")
                    for m in metas
                    if (m or {}).get("fetched_at")
                ]
                info["sources"] = sources
                info["doc_types"] = doc_types
                info["latest_fetched_at"] = max(fetched) if fetched else None
            except Exception as exc:
                logger.warning("chromadb status filter failed collection=%s err=%s", name, exc)
                info["company_count"] = None
                info["error"] = str(exc)
        collections_out.append(info)

    return {
        "backend": describe_backend(),
        "host": CHROMA_HOST or None,
        "port": CHROMA_PORT if CHROMA_HOST else None,
        "company": company,
        "collections": collections_out,
        "total_documents": sum(int(c.get("count") or 0) for c in collections_out),
    }


def ping_chroma() -> Tuple[bool, str]:
    """Chroma 接続確認。"""
    try:
        client = get_chroma_client()
        client.list_collections()
        return True, describe_backend()
    except Exception as exc:
        return False, str(exc)


def describe_backend() -> str:
    if CHROMA_HOST:
        return f"chromadb http://{CHROMA_HOST}:{CHROMA_PORT}"
    return f"chromadb persistent:{CHROMA_DATA_DIR}"


# --- fetched_at_ts の backfill（SOCAIAGENT-659） -----------------------------------
#
# 検索は TTL を where の fetched_at_ts で絞るので、fetched_at_ts を持たない既存の文書は検索に当たらない。
# rag-review の起動時（main.start_fetched_at_ts_backfill）と、手動のスクリプト
# （scripts/backfill_fetched_at_ts.py）がこれを使う。

BACKFILL_BATCH = 500


def _empty_backfill_stats() -> Dict[str, Any]:
    return {"scanned": 0, "already": 0, "filled": 0, "unreadable": 0}


def backfill_collection(collection: Any, apply: bool = False, batch: Optional[int] = None) -> Dict[str, Any]:
    """1つのコレクションの文書に fetched_at_ts を補い、件数（scanned / already / filled / unreadable）を返す。

    - fetched_at が読めて、ありえない未来でない文書だけに足す。読めない・無い文書は古い扱いのまま（U1）
    - 既に数値の fetched_at_ts がある文書には触らない（何度流しても同じ結果になる）
    - 先に ID を全部集めてから、ID で分けて読む。起動時は検索・書き込みと並行して動くので、
      offset で読み進めると、途中で文書が消えたときに読み飛ばす
    - update には fetched_at_ts だけを渡す（Chroma の update は項目ごとにマージする）。読んでから
      update するまでに同じ ID が書き直されても、新しい本文に古いメタデータを付けない
    - update の直前に、書き込みと同じロック（_write_lock）の中で読み直し、その間に数値の fetched_at_ts が
      付いた・fetched_at が変わった（新しい本文が書かれた）ID は上書きしない。古い時刻に戻すと、
      TTL 外になった新しい本文が検索から消えるため（レビューの Mid）
    """
    size = batch or BACKFILL_BATCH
    stats = _empty_backfill_stats()
    all_ids: List[str] = list(collection.get(include=[]).get("ids") or [])
    for start in range(0, len(all_ids), size):
        got = collection.get(ids=all_ids[start:start + size], include=["metadatas"])
        ids: List[str] = list(got.get("ids") or [])
        metadatas = got.get("metadatas") or [{} for _ in ids]
        planned: Dict[str, Tuple[Any, int]] = {}
        for doc_id, meta in zip(ids, metadatas):
            meta = meta or {}
            stats["scanned"] += 1
            if _has_numeric_ts(meta):
                stats["already"] += 1
                continue
            ts = fetched_at_to_ts(meta.get("fetched_at"))
            if ts is None:
                stats["unreadable"] += 1
                continue
            planned[doc_id] = (meta.get("fetched_at"), ts)
        if not planned:
            continue
        if not apply:
            stats["filled"] += len(planned)
            continue
        with _write_lock:
            # 読んでから今までに書き直された ID は除く（新しい本文には書き込み側が数値の時刻を付けている）
            now = collection.get(ids=list(planned), include=["metadatas"])
            update_ids: List[str] = []
            update_metas: List[Dict[str, Any]] = []
            for doc_id, meta in zip(now.get("ids") or [], now.get("metadatas") or []):
                meta = meta or {}
                fetched_at, ts = planned[doc_id]
                if _has_numeric_ts(meta) or meta.get("fetched_at") != fetched_at:
                    stats["already"] += 1
                    continue
                update_ids.append(doc_id)
                update_metas.append({"fetched_at_ts": ts})
            if update_ids:
                collection.update(ids=update_ids, metadatas=update_metas)
            stats["filled"] += len(update_ids)
    return stats


def backfill_fetched_at_ts(client: Any = None, apply: bool = False) -> Dict[str, Dict[str, Any]]:
    """今の世代（RAG_COLLECTION_SUFFIX）の企業データのコレクションをすべて補う。

    1つのコレクションで失敗しても、残りのコレクションは続けて補う。失敗したコレクションは
    結果に {"error": 例外の種類} を入れ、WARNING に残す（どこで止まったかを追えるように）。
    """
    client = client or get_chroma_client()
    existing = _collection_names(client)
    result: Dict[str, Dict[str, Any]] = {}
    for logical in ALL_COLLECTIONS:
        physical = physical_collection_name(logical)
        if physical not in existing:
            result[logical] = _empty_backfill_stats()
            continue
        try:
            result[logical] = backfill_collection(client.get_collection(physical), apply=apply)
        except Exception as exc:
            logger.warning("fetched_at_ts backfill failed collection=%s error=%s", physical, exc)
            result[logical] = {**_empty_backfill_stats(), "error": type(exc).__name__}
    return result


def backfill_failed_collections(result: Dict[str, Dict[str, Any]]) -> List[str]:
    """backfill_fetched_at_ts の結果のうち、失敗したコレクションの論理名。"""
    return [name for name, stats in result.items() if stats.get("error")]
