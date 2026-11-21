"""RAG の堅牢性（SOCAIAGENT-701）。背景は Notion の PRD/DesignDoc 参照。

例外文に載る文字列はすべて合成データ。
"""
import inspect
import json
import logging
from unittest.mock import MagicMock, patch

import pytest
from fastapi import HTTPException

from routers import health as health_router
from routers import student_search as student_router
from routers import vector as vector_router

# 例外の本文に載っていたら困る文字列（合成）
SECRET = "SECRET-試験花子-hanako@example.com"


def _boom(*_args, **_kwargs):
    raise RuntimeError(SECRET)


# ---- Mid-1 / Mid-6: /healthz ----


def test_healthz_is_async():
    """スレッドプールが埋まっていても返せるよう、/healthz はイベントループ上で動く。"""
    assert inspect.iscoroutinefunction(health_router.healthz)


def test_healthz_503_without_internal_token(client, monkeypatch):
    """内部トークンが空だと全 API が 503 になるので、/healthz も 503 にしてタスクを入れ替えさせる。"""
    monkeypatch.setattr(health_router, "ping_chroma", lambda: (True, "ok"))
    monkeypatch.setenv("RAG_INTERNAL_TOKEN", "  ")
    res = client.get("/healthz")
    assert res.status_code == 503
    assert res.json()["internal_auth"] == {"configured": False}


def test_healthz_200_with_internal_token(client, monkeypatch):
    monkeypatch.setattr(health_router, "ping_chroma", lambda: (True, "ok"))
    res = client.get("/healthz")
    assert res.status_code == 200
    assert res.json()["internal_auth"] == {"configured": True}


# ---- Mid-3 / Low-8: 例外文を応答・ログに出さない ----


def test_health_hides_vector_store_error_text(client, monkeypatch):
    import vector_store

    monkeypatch.setattr(vector_store, "get_chroma_client", _boom)
    monkeypatch.setattr(health_router, "ping_chroma", vector_store.ping_chroma)
    res = client.get("/health")
    assert res.json()["vector_store"]["ok"] is False
    assert SECRET not in res.text


def test_es_review_500_hides_exception_text(caplog):
    from services.es_review import _run_es_review

    client = MagicMock()
    client.chat.completions.create.side_effect = _boom
    caplog.set_level(logging.DEBUG)
    with patch("main.OpenAI", return_value=client), pytest.raises(HTTPException) as exc_info:
        _run_es_review(es_text="学生時代に力を入れたことは、", question_type="自己PR", company_name="", context_docs=[])
    assert exc_info.value.status_code == 500
    assert SECRET not in str(exc_info.value.detail)
    assert SECRET not in caplog.text


@pytest.mark.parametrize(
    "method,path,body,target,attr",
    [
        ("get", "/vector/status", None, vector_router, "get_index_status"),
        ("post", "/vector/reembed", {"company_name": "架空商事", "refresh": False}, vector_router, "delete_company_documents"),
        ("post", "/student-search/index", {"user_id": 1, "text": "自己PR"}, student_router, "embed_texts"),
        ("post", "/student-search/query", {"query": "Go"}, student_router, "embed_texts"),
        ("delete", "/student-search/1", None, student_router, "delete_student"),
    ],
)
def test_routes_hide_exception_text(client, monkeypatch, caplog, method, path, body, target, attr):
    monkeypatch.setattr(target, attr, _boom)
    caplog.set_level(logging.DEBUG)
    kwargs = {"json": body} if body is not None else {}
    res = getattr(client, method)(path, **kwargs)
    assert res.status_code == 503
    assert SECRET not in res.text
    assert SECRET not in caplog.text


def test_index_status_hides_collection_error_text(monkeypatch):
    import vector_store

    collection = MagicMock()
    collection.count.return_value = 1
    collection.get.side_effect = _boom
    chroma = MagicMock()
    chroma.list_collections.return_value = [vector_store.physical_collection_name(n) for n in vector_store.ALL_COLLECTIONS]
    chroma.get_collection.return_value = collection
    monkeypatch.setattr(vector_store, "get_chroma_client", lambda: chroma)
    out = vector_store.get_index_status("架空商事", company_original="架空商事")
    assert out["collections"] and all(c["error"] == "RuntimeError" for c in out["collections"])
    assert SECRET not in json.dumps(out, ensure_ascii=False, default=str)


# ---- Mid-4: 入力の上限 ----


@pytest.mark.parametrize(
    "path,body",
    [
        ("/resume/review", {"resume_text": "x", "company_name": "あ" * 201}),
        ("/resume/review", {"resume_text": "x", "company_name": "架空商事", "job_title": "あ" * 101}),
        ("/company/hints", {"company_name": "あ" * 201}),
        ("/es/review", {"es_text": "x", "company_name": "あ" * 201}),
        ("/es/review", {"es_text": "x", "tech_stack": "あ" * 2001}),
        ("/company/context", {"company_name": "あ" * 201, "content": "x"}),
        ("/company/context", {"company_name": "架空商事", "content": "x", "context_type": "other"}),
        ("/vector/reembed", {"company_name": "あ" * 201}),
        ("/student-search/index", {"user_id": 1, "text": "あ" * 20001}),
    ],
)
def test_inputs_over_limit_are_rejected(client, path, body):
    assert client.post(path, json=body).status_code == 422


def test_company_context_accepts_allowed_context_types():
    from models import CompanyContextRequest

    for t in ("jobs", "persona", "general"):
        assert CompanyContextRequest(company_name="架空商事", content="x", context_type=t).context_type == t


SESSION = {"id": 1, "application_status": "offered", "utterances": [{"role": "user", "text": "文化祭の実行委員をしました"}]}


@pytest.mark.parametrize("path", ["/training/export", "/training/export_to_file?output_filename=t.jsonl"])
def test_training_rejects_too_many_sessions(client, monkeypatch, tmp_path, path):
    import training_api

    monkeypatch.setenv("RAG_TRAINING_EXPORT_DIR", str(tmp_path))
    monkeypatch.setattr(training_api, "TRAINING_MAX_SESSIONS", 2)
    assert client.post(path, json=[SESSION] * 3).status_code == 413
    assert client.post(path, json=[SESSION] * 2).status_code == 200


@pytest.mark.parametrize("path", ["/training/export", "/training/export_to_file?output_filename=t.jsonl"])
def test_training_rejects_too_large_body(client, monkeypatch, tmp_path, path):
    import training_api

    monkeypatch.setenv("RAG_TRAINING_EXPORT_DIR", str(tmp_path))
    monkeypatch.setattr(training_api, "TRAINING_MAX_BODY_BYTES", 200)
    big = dict(SESSION, utterances=[{"role": "user", "text": "あ" * 200}])
    assert client.post(path, json=[big]).status_code == 413
    assert client.post(path, json=[SESSION]).status_code == 200


# ---- Low-7: /training/export_to_file ----


def test_export_to_file_writes_same_records_as_export(client, monkeypatch, tmp_path):
    """ファイルに書く形は /training/export と同じ（to_openai_prompt の prompt / completion）。"""
    monkeypatch.setenv("RAG_TRAINING_EXPORT_DIR", str(tmp_path))
    streamed = client.post("/training/export", json=[SESSION]).text
    assert client.post("/training/export_to_file?output_filename=t.jsonl", json=[SESSION]).status_code == 200
    written = (tmp_path / "t.jsonl").read_text(encoding="utf-8")
    assert written == streamed
    assert set(json.loads(written.splitlines()[0])) == {"prompt", "completion"}


def test_export_to_file_keeps_old_file_on_failure(client, monkeypatch, tmp_path):
    """書き込みの途中で失敗しても、既存のファイルを壊さず、一時ファイルも残さない。"""
    import training_api

    monkeypatch.setenv("RAG_TRAINING_EXPORT_DIR", str(tmp_path))
    (tmp_path / "t.jsonl").write_text("old\n", encoding="utf-8")

    def broken(_sessions, _mask):
        yield {"prompt": "p", "completion": "c"}
        raise OSError("disk full")

    monkeypatch.setattr(training_api, "_training_records", broken)
    res = client.post("/training/export_to_file?output_filename=t.jsonl", json=[SESSION])
    assert res.status_code == 500
    assert (tmp_path / "t.jsonl").read_text(encoding="utf-8") == "old\n"
    assert sorted(p.name for p in tmp_path.iterdir()) == ["t.jsonl"]


# ---- Low-10: 再埋め込みのロック ----


def test_reembed_lock_is_released_from_registry(client, monkeypatch):
    """使い終わった企業のロックは残さない（企業名の数だけ増え続けない）。"""
    monkeypatch.setattr(vector_router, "delete_company_documents", lambda *a, **k: {"deleted": 0, "collections": {}})
    for i in range(3):
        assert client.post("/vector/reembed", json={"company_name": f"架空商事{i}", "refresh": False}).status_code == 200
    assert len(vector_router._reembed_running) == 0

