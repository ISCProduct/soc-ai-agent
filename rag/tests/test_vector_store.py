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

    def _where_value(where, key):
        # _build_where は複数キーを{"$and": [{k: v}, ...]}でラップするため、
        # フラット/$and両方の形状からキーを取り出せるようにする。
        if key in where:
            return where[key]
        for clause in where.get("$and", []):
            if key in clause:
                return clause[key]
        return None

    def query_side_effect(**kwargs):
        where = kwargs.get("where") or {}
        if _where_value(where, "role") == "別職種":
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


def test_query_with_where_falls_back_to_scoped_filter_on_error():
    # whereクエリ自体が例外を投げる環境でも、フィルタを完全に外すのではなく
    # fallback_where(companyのみ)でスコープを維持したまま再試行すること。
    collection = MagicMock()
    calls = []

    def query_side_effect(**kwargs):
        calls.append(kwargs.get("where"))
        if kwargs.get("where") == {"$and": [{"company": "Acme"}, {"role": "エンジニア"}]}:
            raise ValueError("Expected where to have exactly one operator")
        return {"documents": [["Acme社のメモ"]], "metadatas": [[{"fetched_at": None}]]}

    collection.query.side_effect = query_side_effect

    docs = vs._query_with_where(
        collection,
        [0.1, 0.2, 0.3],
        where=vs._build_where({"company": "Acme", "role": "エンジニア"}),
        n_results=5,
        fallback_where=vs._build_where({"company": "Acme"}),
    )

    assert calls == [
        {"$and": [{"company": "Acme"}, {"role": "エンジニア"}]},
        {"company": "Acme"},
    ]
    assert docs == ["Acme社のメモ"]


def test_build_where_wraps_multi_key_filters():
    # ChromaDBはフラットな複数キーのwhereを許可しない(1オペレータのみ)ため、
    # 2キー以上は$andでラップする必要がある。
    assert vs._build_where({"company": "Acme"}) == {"company": "Acme"}
    assert vs._build_where({}) == {}
    assert vs._build_where({"company": "Acme", "role": "エンジニア"}) == {
        "$and": [{"company": "Acme"}, {"role": "エンジニア"}]
    }


def test_describe_backend_http(monkeypatch):
    monkeypatch.setattr(vs, "CHROMA_HOST", "chroma")
    monkeypatch.setattr(vs, "CHROMA_PORT", 8000)
    assert "http://chroma:8000" in vs.describe_backend()


def test_describe_backend_persistent(monkeypatch):
    monkeypatch.setattr(vs, "CHROMA_HOST", "")
    monkeypatch.setattr(vs, "CHROMA_DATA_DIR", "/tmp/chroma")
    assert "persistent:/tmp/chroma" in vs.describe_backend()


def test_build_cache_key():
    assert vs.build_cache_key("interview_hints", "Acme", "一般職") == "hints::Acme::一般職"
    assert vs.build_cache_key("es_review", "Acme") == "Acme::es_review"
    assert vs.build_cache_key("company_research", "Acme", "指定なし") == "Acme::指定なし"


def test_company_context_fallback_not_only_hints():
    vs.reset_chroma_client_for_tests()
    collection = MagicMock()
    collection.count.return_value = 1
    fetched = datetime.now(timezone.utc).isoformat()

    def query_side_effect(**kwargs):
        where = kwargs.get("where") or {}
        if where.get("role") == "エンジニア":
            return {"documents": [[]], "metadatas": [[]]}
        return {
            "documents": [["企業共通メモ"]],
            "metadatas": [[{
                "company": "Acme",
                "role": "指定なし",
                "fetched_at": fetched,
                "source": "job_fetch",
            }]],
        }

    collection.query.side_effect = query_side_effect
    col_obj = MagicMock()
    col_obj.name = vs.COLLECTION_COMPANY_CONTEXT
    client = MagicMock()
    client.list_collections.return_value = [col_obj]
    client.get_collection.return_value = collection

    with patch.object(vs, "get_chroma_client", return_value=client):
        docs = vs.get_cached_documents(
            "Acme::エンジニア",
            query_embedding=[0.1, 0.2, 0.3],
            allow_company_fallback=True,
        )
    assert docs == ["企業共通メモ"]


def test_get_index_status_and_delete():
    vs.reset_chroma_client_for_tests()
    collection = MagicMock()
    collection.count.side_effect = [3, 3, 1]
    collection.get.return_value = {
        "metadatas": [{
            "company": "Acme",
            "source": "job_fetch",
            "doc_type": "company_research",
            "fetched_at": "2026-01-01T00:00:00+00:00",
        }],
        "ids": ["id1"],
    }
    col_obj = MagicMock()
    col_obj.name = vs.COLLECTION_COMPANY_CONTEXT
    client = MagicMock()
    client.list_collections.return_value = [col_obj]
    client.get_collection.return_value = collection

    with patch.object(vs, "get_chroma_client", return_value=client):
        status = vs.get_index_status("Acme")
        deleted = vs.delete_company_documents("Acme", doc_type="company_research")

    assert status["total_documents"] >= 0
    assert any(c["name"] == vs.COLLECTION_COMPANY_CONTEXT for c in status["collections"])
    assert "deleted" in deleted
    assert collection.delete.called


def test_collection_names_accepts_string_list():
    """Chroma 0.6 HttpClient は list_collections が str のリストを返す。"""
    client = MagicMock()
    client.list_collections.return_value = ["company_context", "interview_hints"]
    assert vs._collection_names(client) == {"company_context", "interview_hints"}


def test_get_index_status_with_string_collections():
    collection = MagicMock()
    collection.count.return_value = 2
    client = MagicMock()
    client.list_collections.return_value = ["company_context", "interview_hints", "es_review"]
    client.get_collection.return_value = collection
    with patch.object(vs, "get_chroma_client", return_value=client):
        status = vs.get_index_status()
    assert status["total_documents"] == 6
    assert all(c["exists"] for c in status["collections"])


def test_set_cached_documents_doc_type_override():
    vs.reset_chroma_client_for_tests()
    collection = MagicMock()
    client = MagicMock()
    client.get_or_create_collection.return_value = collection
    with patch.object(vs, "get_chroma_client", return_value=client):
        vs.set_cached_documents(
            "Acme::指定なし",
            ["doc"],
            [[0.1]],
            source="web_search",
            doc_type="resume_review",
        )
    meta = collection.upsert.call_args.kwargs["metadatas"][0]
    assert meta["doc_type"] == "resume_review"
    assert meta["source"] == "web_search"
    client.get_or_create_collection.assert_called_with(vs.COLLECTION_COMPANY_CONTEXT)
