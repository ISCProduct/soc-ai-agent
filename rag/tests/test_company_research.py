"""企業研究AIの文章の分割・登録・検索（SOCAIAGENT-725）。本文は合成データ。"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import company_research_store as store


def test_split_text_respects_size_and_overlap():
    text = "当社は合成データの会社です。" * 100  # 1400字
    chunks = store.split_text(text, size=300, overlap=50)
    assert len(chunks) >= 4
    assert all(len(c) <= 300 for c in chunks)
    # 隣どうしは重なる（文脈が切れない）
    assert chunks[1][:20] in chunks[0]


def test_split_text_long_sentence_and_empty():
    assert store.split_text("") == []
    chunks = store.split_text("あ" * 1000, size=300, overlap=50)
    assert all(len(c) <= 300 for c in chunks)
    assert "".join(chunks).count("あ") >= 1000


def _collection():
    col = MagicMock()
    client = MagicMock(get_or_create_collection=MagicMock(return_value=col))
    return col, client


def test_index_replaces_document_chunks(client):
    col, chroma = _collection()
    with patch.object(store, "get_chroma_client", return_value=chroma), \
         patch("routers.company_research.embed_texts", side_effect=lambda ts: [[0.1, 0.2] for _ in ts]):
        res = client.post("/company-research/index", json={
            "company_id": 5,
            "documents": [{"document_id": 11, "source_type": "edinet", "title": "事業の内容", "url": "https://example.com/x",
                           "as_of": "2025-03-31", "body": "合成の事業説明です。" * 10}],
        })
    assert res.status_code == 200, res.text
    body = res.json()
    assert body["chunks"] >= 1
    assert body["usage"]["calls"] == 1 and body["usage"]["prompt_tokens"] > 0
    # 新しいチャンクを先に入れてから、余った古いチャンクだけを消す（登録に失敗しても検索できなくならない）
    assert col.upsert.called
    n = res.json()["chunks"]
    col.delete.assert_called_once_with(where={"$and": [{"document_id": 11}, {"chunk_index": {"$gte": n}}]})
    meta = col.upsert.call_args.kwargs["metadatas"][0]
    assert meta["company_id"] == 5 and meta["document_id"] == 11 and meta["source_type"] == "edinet"


def test_query_filters_by_company(client):
    col, chroma = _collection()
    col.query.return_value = {
        "documents": [["自社の文章", "他社の文章"]],
        "metadatas": [[{"company_id": 5, "title": "事業の内容"}, {"company_id": 99, "title": "他社"}]],
        "distances": [[0.1, 0.2]],
    }
    with patch.object(store, "get_chroma_client", return_value=chroma), \
         patch("routers.company_research.embed_texts", return_value=[[0.3, 0.4]]):
        res = client.post("/company-research/query", json={"company_ids": [5], "question": "何をしている会社？"})
    assert res.status_code == 200, res.text
    assert col.query.call_args.kwargs["where"] == {"company_id": 5}
    hits = res.json()["hits"]
    # where をすり抜けた他社の文章も返さない
    assert [h["text"] for h in hits] == ["自社の文章"]
    assert abs(hits[0]["score"] - 0.9) < 1e-9


def test_query_two_companies_uses_in(client):
    col, chroma = _collection()
    col.query.return_value = {"documents": [[]], "metadatas": [[]], "distances": [[]]}
    with patch.object(store, "get_chroma_client", return_value=chroma), \
         patch("routers.company_research.embed_texts", return_value=[[0.3]]):
        client.post("/company-research/query", json={"company_ids": [5, 6], "question": "違いは？"})
    assert col.query.call_args.kwargs["where"] == {"company_id": {"$in": [5, 6]}}


def test_requires_internal_token():
    from fastapi.testclient import TestClient
    import main
    res = TestClient(main.app).post("/company-research/query", json={"company_ids": [5], "question": "x"})
    assert res.status_code in (401, 403)


def test_split_text_overlap_never_exceeds_size():
    # 200字の文のあとに280字の文が続いても、size=300 を超えない（レビュー指摘）
    text = "あ" * 199 + "。" + "い" * 279 + "。"
    chunks = store.split_text(text, size=300, overlap=50)
    assert all(len(c) <= 300 for c in chunks), [len(c) for c in chunks]


def test_index_accepts_special_token_text(client):
    col, chroma = _collection()
    with patch.object(store, "get_chroma_client", return_value=chroma), \
         patch("routers.company_research.embed_texts", side_effect=lambda ts: [[0.1] for _ in ts]):
        res = client.post("/company-research/index", json={
            "company_id": 5,
            "documents": [{"document_id": 12, "source_type": "edinet", "body": "合成の本文 <|endoftext|> です。"}],
        })
    assert res.status_code == 200, res.text
