"""#573 ベクトルストア（Chroma）の単体テスト。"""
from __future__ import annotations

from datetime import datetime, timedelta, timezone
from unittest.mock import MagicMock, patch

import vector_store as vs


def test_parse_cache_key_hints():
    meta = vs.parse_cache_key("hints::Acme::エンジニア")
    assert meta["collection"] == vs.COLLECTION_INTERVIEW_HINTS
    assert meta["company"] == "Acme"
    assert meta["role"] == "エンジニア"
    assert meta["doc_type"] == "interview_hints"


def test_parse_cache_key_es_review():
    meta = vs.parse_cache_key("Acme::es_review")
    assert meta["collection"] == vs.COLLECTION_ES_REVIEW
    assert meta["doc_type"] == "es_review"


def test_parse_cache_key_company_context():
    meta = vs.parse_cache_key("Acme::一般職")
    assert meta["collection"] == vs.COLLECTION_COMPANY_CONTEXT
    assert meta["company"] == "Acme"
    assert meta["role"] == "一般職"


def test_is_fresh_ttl():
    now = datetime.now(timezone.utc)
    fresh = (now - timedelta(hours=1)).isoformat()
    stale = (now - timedelta(days=2)).isoformat()
    assert vs._is_fresh(fresh, ttl_seconds=86400) is True
    assert vs._is_fresh(stale, ttl_seconds=86400) is False


def test_set_and_get_cached_documents_roundtrip():
    vs.reset_chroma_client_for_tests()

    collection = MagicMock()
    collection.count.return_value = 1
    fetched = datetime.now(timezone.utc).isoformat()

    def query_side_effect(**kwargs):
        where = kwargs.get("where") or {}
        if where.get("role") == "別職種":
            return {"documents": [[]], "metadatas": [[]]}
        return {
            "documents": [["採用方針のメモ"]],
            "metadatas": [[{
                "company": "Acme",
                "role": "エンジニア",
                "fetched_at": fetched,
                "source": "web_search",
            }]],
        }

    collection.query.side_effect = query_side_effect

    col_obj = MagicMock()
    col_obj.name = vs.COLLECTION_INTERVIEW_HINTS
    client = MagicMock()
    client.list_collections.return_value = [col_obj]
    client.get_collection.return_value = collection
    client.get_or_create_collection.return_value = collection

    with patch.object(vs, "get_chroma_client", return_value=client):
        vs.set_cached_documents(
            "hints::Acme::エンジニア",
            ["採用方針のメモ"],
            [[0.1, 0.2, 0.3]],
            source="web_search",
        )
        docs = vs.get_cached_documents(
            "hints::Acme::別職種",
            query_embedding=[0.1, 0.2, 0.3],
            allow_company_fallback=True,
        )

    assert collection.upsert.called
    upsert_kwargs = collection.upsert.call_args.kwargs
    assert upsert_kwargs["metadatas"][0]["source"] == "web_search"
    assert "fetched_at" in upsert_kwargs["metadatas"][0]
    assert docs == ["採用方針のメモ"]
    assert collection.query.call_count >= 2


def test_describe_backend_http(monkeypatch):
    monkeypatch.setattr(vs, "CHROMA_HOST", "chroma")
    monkeypatch.setattr(vs, "CHROMA_PORT", 8000)
    assert "http://chroma:8000" in vs.describe_backend()


def test_describe_backend_persistent(monkeypatch):
    monkeypatch.setattr(vs, "CHROMA_HOST", "")
    monkeypatch.setattr(vs, "CHROMA_DATA_DIR", "/tmp/chroma")
    assert "persistent:/tmp/chroma" in vs.describe_backend()
