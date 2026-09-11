"""#1094 企業向け学生セマンティック検索の単体テスト。"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import student_profile_store as sps


def _mock_collection(count: int = 0, query_result: dict | None = None) -> MagicMock:
    collection = MagicMock()
    collection.count.return_value = count
    if query_result is not None:
        collection.query.return_value = query_result
    return collection


# ── ベクトルストア ──────────────────────────────────────────────────


def test_upsert_student_uses_stable_doc_id():
    """1学生1ドキュメント: 再登録時は同じIDへupsertされ重複しない。"""
    collection = _mock_collection()
    with patch.object(sps, "get_chroma_client", return_value=MagicMock(
        get_or_create_collection=MagicMock(return_value=collection)
    )):
        sps.upsert_student(42, "取得資格: 基本情報", [0.1, 0.2])

    kwargs = collection.upsert.call_args.kwargs
    assert kwargs["ids"] == ["student-42"]
    assert kwargs["documents"] == ["取得資格: 基本情報"]
    assert kwargs["metadatas"][0]["user_id"] == 42


def test_delete_student_is_idempotent():
    """同意撤回時の削除。存在しなくても例外にしない。"""
    collection = _mock_collection()
    with patch.object(sps, "get_chroma_client", return_value=MagicMock(
        get_or_create_collection=MagicMock(return_value=collection)
    )):
        sps.delete_student(42)
    collection.delete.assert_called_once_with(ids=["student-42"])


def test_query_students_empty_collection_returns_empty():
    """コレクションが空ならクエリを投げずに空を返す。"""
    collection = _mock_collection(count=0)
    with patch.object(sps, "get_chroma_client", return_value=MagicMock(
        get_or_create_collection=MagicMock(return_value=collection)
    )):
        assert sps.query_students([0.1], 10) == []
    collection.query.assert_not_called()


def test_query_students_orders_by_relevance():
    """距離が小さいほど関連度スコアが高く、返却順は関連度順を保つ。"""
    collection = _mock_collection(count=3, query_result={
        "metadatas": [[{"user_id": 5}, {"user_id": 2}, {"user_id": 9}]],
        "distances": [[0.1, 0.4, 0.9]],
    })
    with patch.object(sps, "get_chroma_client", return_value=MagicMock(
        get_or_create_collection=MagicMock(return_value=collection)
    )):
        hits = sps.query_students([0.1], 10)

    assert [h["user_id"] for h in hits] == [5, 2, 9]
    assert hits[0]["score"] > hits[1]["score"] > hits[2]["score"]


def test_query_students_caps_n_results_to_collection_size():
    """top_k がコレクション件数を超えても Chroma へは件数上限で渡す。"""
    collection = _mock_collection(count=2, query_result={
        "metadatas": [[{"user_id": 1}, {"user_id": 2}]],
        "distances": [[0.1, 0.2]],
    })
    with patch.object(sps, "get_chroma_client", return_value=MagicMock(
        get_or_create_collection=MagicMock(return_value=collection)
    )):
        sps.query_students([0.1], 100)
    assert collection.query.call_args.kwargs["n_results"] == 2


def test_query_students_skips_rows_without_user_id():
    """user_id 欠損のメタデータは結果から除外する。"""
    collection = _mock_collection(count=2, query_result={
        "metadatas": [[{"user_id": 5}, {}]],
        "distances": [[0.1, 0.2]],
    })
    with patch.object(sps, "get_chroma_client", return_value=MagicMock(
        get_or_create_collection=MagicMock(return_value=collection)
    )):
        hits = sps.query_students([0.1], 10)
    assert [h["user_id"] for h in hits] == [5]


# ── HTTP エンドポイント ─────────────────────────────────────────────


def test_student_index_endpoint(client):
    with patch("routers.student_search.embed_texts", return_value=[[0.1, 0.2]]), \
            patch("routers.student_search.upsert_student") as upsert:
        res = client.post("/student-search/index", json={"user_id": 7, "text": "React"})
    assert res.status_code == 200
    upsert.assert_called_once()


def test_student_index_rejects_empty_text(client):
    res = client.post("/student-search/index", json={"user_id": 7, "text": ""})
    assert res.status_code == 422


def test_student_index_rejects_invalid_user_id(client):
    res = client.post("/student-search/index", json={"user_id": 0, "text": "React"})
    assert res.status_code == 422


def test_student_query_endpoint(client):
    with patch("routers.student_search.embed_texts", return_value=[[0.1, 0.2]]), \
            patch("routers.student_search.query_students",
                  return_value=[{"user_id": 5, "score": 0.9}]):
        res = client.post("/student-search/query", json={"query": "React"})
    assert res.status_code == 200
    assert res.json()["hits"] == [{"user_id": 5, "score": 0.9}]


def test_student_query_rejects_empty_query(client):
    res = client.post("/student-search/query", json={"query": ""})
    assert res.status_code == 422


def test_student_query_caps_top_k(client):
    """top_k は MAX_TOP_K で頭打ちにし、過大なリクエストで負荷が跳ねないようにする。"""
    with patch("routers.student_search.embed_texts", return_value=[[0.1]]), \
            patch("routers.student_search.query_students", return_value=[]) as q:
        res = client.post("/student-search/query", json={"query": "React", "top_k": 99999})
    assert res.status_code == 200
    assert q.call_args.args[1] == 200


def test_student_query_embedding_failure_returns_503(client):
    with patch("routers.student_search.embed_texts", side_effect=RuntimeError("openai down")):
        res = client.post("/student-search/query", json={"query": "React"})
    assert res.status_code == 503


def test_student_delete_endpoint(client):
    with patch("routers.student_search.delete_student") as delete:
        res = client.delete("/student-search/7")
    assert res.status_code == 200
    delete.assert_called_once_with(7)


def test_student_search_requires_internal_token():
    """内部認証トークンなしのアクセスは 401（既存のフェイルクローズ方針を踏襲）。"""
    from fastapi.testclient import TestClient
    import main

    unauthenticated = TestClient(main.app)
    res = unauthenticated.post("/student-search/query", json={"query": "React"})
    assert res.status_code == 401
