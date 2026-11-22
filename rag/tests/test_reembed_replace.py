"""企業データの再埋め込みの置き換え（SOCAIAGENT-658）。

以前は「削除 → 再取得」の順で、再取得が例外・空だと企業データが空のまま残った。
「新しく書く → 書けたことを確かめる → 旧文書を消す」の順になっていることを、本物の Chroma で確かめる。
"""
from typing import Any, Dict, List
from unittest.mock import patch

import pytest

import vector_store as vs

EMB = [0.1, 0.2, 0.3]


def _matches(meta: Dict[str, Any], where: Dict[str, Any]) -> bool:
    if not where:
        return True
    if "$and" in where:
        return all(_matches(meta, w) for w in where["$and"])
    return all(
        meta.get(k) in v["$in"] if isinstance(v, dict) and "$in" in v else meta.get(k) == v
        for k, v in where.items()
    )


class FakeCollection:
    """テストの chromadb はスタブなので、使う操作（upsert / get / delete / count）だけを持つ偽物。"""

    def __init__(self, name: str) -> None:
        self.name = name
        self.rows: Dict[str, Dict[str, Any]] = {}

    def upsert(self, ids, documents, embeddings, metadatas):
        for i, doc, meta in zip(ids, documents, metadatas):
            self.rows[i] = {"document": doc, "metadata": dict(meta)}

    def get(self, ids=None, where=None, include=None):
        picked = [i for i in self.rows if (ids is None or i in ids) and _matches(self.rows[i]["metadata"], where or {})]
        return {"ids": picked, "documents": [self.rows[i]["document"] for i in picked]}

    def delete(self, ids=None, where=None):
        for i in [i for i in self.rows if (ids is None or i in ids) and _matches(self.rows[i]["metadata"], where or {})]:
            del self.rows[i]

    def count(self):
        return len(self.rows)


class FakeChroma:
    def __init__(self) -> None:
        self.collections: Dict[str, FakeCollection] = {}

    def get_or_create_collection(self, name):
        return self.collections.setdefault(name, FakeCollection(name))

    def get_collection(self, name):
        return self.collections[name]

    def list_collections(self) -> List[str]:
        return list(self.collections)


@pytest.fixture
def chroma(monkeypatch):
    client = FakeChroma()
    monkeypatch.setattr(vs, "get_chroma_client", lambda: client)
    monkeypatch.delenv("RAG_COLLECTION_SUFFIX", raising=False)
    return client


def _seed(client, collection, company, role, docs, doc_type, company_original=None):
    key = vs.build_cache_key(doc_type, company, role, company_original=company_original or company)
    col = client.get_or_create_collection(vs.physical_collection_name(collection))
    ids = vs._doc_ids(key, len(docs), doc_type)
    meta = vs.parse_cache_key(key)
    col.upsert(
        ids=ids, documents=docs, embeddings=[EMB] * len(docs),
        metadatas=[{"company": meta["company"], "company_hash": meta.get("company_hash", ""), "role": role,
                    "doc_type": doc_type, "source": "seed", "fetched_at": "2026-11-01T00:00:00+00:00",
                    "cache_key": key} for _ in docs],
    )
    return key


def _docs(client, collection, company):
    col = client.get_or_create_collection(vs.physical_collection_name(collection))
    got = col.get(where={"company": company}, include=["documents"])
    return sorted(got.get("documents") or [])


# --- replace_documents ---------------------------------------------------------


def test_replace_documents_removes_old_chunks_and_other_roles(chroma):
    key = _seed(chroma, "company_context", "A社", "指定なし", ["旧1", "旧2", "旧3"], "company_research")
    _seed(chroma, "company_context", "A社", "エンジニア", ["別の役割の旧文書"], "company_research")
    _seed(chroma, "company_context", "B社", "指定なし", ["B社の文書"], "company_research")

    outcome = vs.replace_documents(key, ["新"], [EMB], source="web_search", doc_type="company_research")

    assert outcome == {"collection": "company_context", "replaced": 1, "deleted": 3}
    assert _docs(chroma, "company_context", "A社") == ["新"]
    # 別の企業は消さない（company と company_hash で絞る）
    assert _docs(chroma, "company_context", "B社") == ["B社の文書"]


def test_replace_documents_keeps_other_doc_types_and_legacy_docs(chroma):
    # company_context は resume_review と company_research が共有している。再取得は指定した用途だけを
    # 置き換え、別の用途と doc_type を持たない旧文書は残す（SOCAIAGENT-659。delete_company_documents と同じ範囲）
    key = _seed(chroma, "company_context", "A社", "指定なし", ["旧1", "旧2"], "company_research")
    _seed(chroma, "company_context", "A社", "エンジニア", ["添削用"], "resume_review")
    col = chroma.get_or_create_collection(vs.physical_collection_name("company_context"))
    col.upsert(ids=["legacy"], documents=["doc_type の無い旧文書"], embeddings=[EMB],
               metadatas=[{"company": "A社", "company_hash": vs._company_hash("A社")}])

    outcome = vs.replace_documents(key, ["新"], [EMB], source="web_search", doc_type="company_research")

    assert outcome["deleted"] == 1
    assert _docs(chroma, "company_context", "A社") == ["doc_type の無い旧文書", "新", "添削用"]


def test_replace_documents_purges_given_doc_types(chroma):
    # 再埋め込みは resume_review を company_research として取り直すので、両方の旧文書を入れ替える
    key = _seed(chroma, "company_context", "A社", "指定なし", ["旧研究"], "company_research")
    _seed(chroma, "company_context", "A社", "エンジニア", ["旧添削1", "旧添削2"], "resume_review")
    col = chroma.get_or_create_collection(vs.physical_collection_name("company_context"))
    col.upsert(ids=["legacy"], documents=["doc_type の無い旧文書"], embeddings=[EMB],
               metadatas=[{"company": "A社", "company_hash": vs._company_hash("A社")}])

    outcome = vs.replace_documents(
        key, ["新"], [EMB], source="web_search", doc_type="company_research",
        purge_doc_types=["company_research", "resume_review"],
    )

    # company_research の旧文書は同じ ID に上書きされ、resume_review の2件が消える
    assert outcome["deleted"] == 2
    assert _docs(chroma, "company_context", "A社") == ["doc_type の無い旧文書", "新"]


def test_replace_documents_keeps_old_when_write_cannot_be_confirmed(chroma):
    key = _seed(chroma, "company_context", "A社", "指定なし", ["旧"], "company_research")
    real = chroma.get_or_create_collection

    class Unconfirmed:
        def __init__(self, col):
            self._col = col

        def __getattr__(self, name):
            return getattr(self._col, name)

        def upsert(self, **kwargs):
            pass  # 書けなかったことにする

        def get(self, **kwargs):
            if "ids" in kwargs:
                return {"ids": []}
            return self._col.get(**kwargs)

    with patch.object(chroma, "get_or_create_collection", side_effect=lambda name: Unconfirmed(real(name))):
        with pytest.raises(vs.ReplaceDocumentsError):
            vs.replace_documents(key, ["新"], [EMB], doc_type="company_research")
    assert _docs(chroma, "company_context", "A社") == ["旧"]


def test_collection_suffix_switches_physical_name(chroma, monkeypatch):
    monkeypatch.setenv("RAG_COLLECTION_SUFFIX", "g2")
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    vs.replace_documents(key, ["新"], [EMB], doc_type="es_review")
    assert "es_review_g2" in chroma.list_collections()


# --- POST /vector/reembed ------------------------------------------------------


def _patch_fetch(monkeypatch, research=None, hints=None, es=None):
    """Web 検索の結果を差し替える。値が例外ならそれを投げる。"""
    import main

    def run_async(fn, company, role):
        value = research if role == "指定なし" else es
        if isinstance(value, Exception):
            raise value
        return value

    def run_hints(company, role):
        if isinstance(hints, Exception):
            raise hints
        return hints

    monkeypatch.setattr(main, "_run_async", run_async)
    monkeypatch.setattr(main, "_run_hints_web_search", run_hints)
    monkeypatch.setattr(main, "embed_texts", lambda texts: [EMB for _ in texts])


def test_reembed_web_search_exception_returns_502_and_keeps_data(client, chroma, monkeypatch):
    """以前の順（削除 → 再取得）だと、ここで企業データが空になっていた。"""
    _seed(chroma, "es_review", "A社", "指定なし", ["旧ES"], "es_review")
    _patch_fetch(monkeypatch, es=RuntimeError("search down"))

    res = client.post("/vector/reembed", json={"company_name": "A社", "doc_type": "es_review", "refresh": True})

    assert res.status_code == 502
    assert _docs(chroma, "es_review", "A社") == ["旧ES"]


def test_reembed_empty_result_keeps_old_data(client, chroma, monkeypatch):
    _seed(chroma, "es_review", "A社", "指定なし", ["旧ES"], "es_review")
    _patch_fetch(monkeypatch, es="")

    body = client.post("/vector/reembed", json={"company_name": "A社", "doc_type": "es_review", "refresh": True}).json()

    assert body["refreshed"] is False and body["deleted"] == 0
    assert body["results"] == [{"doc_type": "es_review", "status": "unchanged", "deleted": 0, "error": None}]
    assert _docs(chroma, "es_review", "A社") == ["旧ES"]


def test_reembed_success_replaces_without_leftovers(client, chroma, monkeypatch):
    _seed(chroma, "es_review", "A社", "指定なし", ["旧1", "旧2"], "es_review")
    _patch_fetch(monkeypatch, es="新ES")

    body = client.post("/vector/reembed", json={"company_name": "A社", "doc_type": "es_review", "refresh": True}).json()

    assert body["refreshed"] is True and body["sources"] == ["es_review"]
    assert _docs(chroma, "es_review", "A社") == ["新ES"]


@pytest.mark.parametrize("doc_type", ["resume_review", None])
def test_reembed_resume_review_or_all_replaces_resume_review_docs(client, chroma, monkeypatch, doc_type):
    # resume_review は company_research として取り直す。resume_review の旧文書も入れ替えないと、
    # 応答は replaced なのに、TTL 切れまで旧文書が返り続ける（レビューの Major 1）
    _seed(chroma, "company_context", "A社", "指定なし", ["旧研究"], "company_research")
    _seed(chroma, "company_context", "A社", "エンジニア", ["旧添削"], "resume_review")
    _patch_fetch(monkeypatch, research="新研究", hints="新ヒント", es="新ES")

    payload = {"company_name": "A社", "refresh": True, **({"doc_type": doc_type} if doc_type else {})}
    body = client.post("/vector/reembed", json=payload).json()

    assert {r["doc_type"]: r["status"] for r in body["results"]}["company_research"] == "replaced"
    assert _docs(chroma, "company_context", "A社") == ["新研究"]


def test_reembed_company_research_keeps_resume_review_docs(client, chroma, monkeypatch):
    # company_research だけを指定したときは、resume_review の文書は入れ替えない
    _seed(chroma, "company_context", "A社", "指定なし", ["旧研究"], "company_research")
    _seed(chroma, "company_context", "A社", "エンジニア", ["添削"], "resume_review")
    _patch_fetch(monkeypatch, research="新研究")

    client.post("/vector/reembed", json={"company_name": "A社", "doc_type": "company_research", "refresh": True})

    assert _docs(chroma, "company_context", "A社") == ["新研究", "添削"]


def test_reembed_partial_failure_replaces_only_successful_types(client, chroma, monkeypatch):
    _seed(chroma, "company_context", "A社", "指定なし", ["旧研究"], "company_research")
    _seed(chroma, "interview_hints", "A社", "一般職", ["旧ヒント"], "interview_hints")
    _seed(chroma, "es_review", "A社", "指定なし", ["旧ES"], "es_review")
    _patch_fetch(monkeypatch, research="新研究", hints=RuntimeError("down"), es="新ES")

    res = client.post("/vector/reembed", json={"company_name": "A社", "refresh": True})

    assert res.status_code == 200
    body = res.json()
    assert body["status"] == "partial"
    assert {r["doc_type"]: r["status"] for r in body["results"]} == {
        "company_research": "replaced", "interview_hints": "failed", "es_review": "replaced",
    }
    assert _docs(chroma, "company_context", "A社") == ["新研究"]
    assert _docs(chroma, "interview_hints", "A社") == ["旧ヒント"]  # 失敗した分は残す
    assert _docs(chroma, "es_review", "A社") == ["新ES"]


def test_reembed_rejects_concurrent_run_for_same_company(client, chroma, monkeypatch):
    from routers import vector as vector_router

    _patch_fetch(monkeypatch, es="新ES")
    lock = vector_router._company_lock(vs.build_cache_key("lock", "A社", company_original="A社"))
    assert lock.acquire(blocking=False)
    try:
        res = client.post("/vector/reembed", json={"company_name": "A社", "doc_type": "es_review", "refresh": True})
        assert res.status_code == 409
    finally:
        lock.release()


def test_reembed_without_refresh_only_deletes(client, chroma):
    _seed(chroma, "es_review", "A社", "指定なし", ["旧ES"], "es_review")
    body = client.post("/vector/reembed", json={"company_name": "A社", "doc_type": "es_review", "refresh": False}).json()
    assert body["deleted"] == 1
    assert _docs(chroma, "es_review", "A社") == []


# --- 全件再構築（scripts/rebuild_collections.py） -------------------------------


class FakeFullCollection(FakeCollection):
    def __init__(self, name, metadata=None):
        super().__init__(name)
        self.metadata = metadata

    def upsert(self, ids, documents, embeddings, metadatas):
        for i, doc, emb, meta in zip(ids, documents, embeddings, metadatas):
            self.rows[i] = {"document": doc, "embedding": emb, "metadata": dict(meta)}

    def get(self, ids=None, where=None, include=None, limit=None, offset=0):
        picked = [i for i in self.rows if (ids is None or i in ids) and _matches(self.rows[i]["metadata"], where or {})]
        picked = picked[offset:offset + limit] if limit else picked
        return {
            "ids": picked,
            "documents": [self.rows[i]["document"] for i in picked],
            "embeddings": [self.rows[i].get("embedding", EMB) for i in picked],
            "metadatas": [self.rows[i]["metadata"] for i in picked],
        }


class FakeFullChroma(FakeChroma):
    def get_or_create_collection(self, name, metadata=None):
        return self.collections.setdefault(name, FakeFullCollection(name, metadata))

    def delete_collection(self, name):
        del self.collections[name]


def test_rebuild_copies_to_new_generation_and_keeps_old(monkeypatch):
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    monkeypatch.setattr(vs, "get_chroma_client", lambda: client)
    monkeypatch.delenv("RAG_COLLECTION_SUFFIX", raising=False)
    monkeypatch.setattr(rb, "BATCH", 2)  # ページングも通す
    _seed(client, "company_context", "A社", "指定なし", ["研究1", "研究2", "研究3"], "company_research")
    _seed(client, "es_review", "A社", "指定なし", ["ES"], "es_review")

    copied = rb.copy_collections(client, "", "g2", space="cosine")

    assert copied == {"company_context": 3, "interview_hints": 0, "es_review": 1}
    assert client.collections["company_context_g2"].metadata == {"hnsw:space": "cosine"}
    assert sorted(r["document"] for r in client.collections["company_context_g2"].rows.values()) == ["研究1", "研究2", "研究3"]
    # 写している間も旧世代はそのまま（検索できる）
    assert len(client.collections["company_context"].rows) == 3


def test_rebuild_refuses_to_drop_current_generation(monkeypatch):
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    monkeypatch.setenv("RAG_COLLECTION_SUFFIX", "g2")
    client.get_or_create_collection("company_context_g2")
    with pytest.raises(ValueError):
        rb.drop_generation(client, "g2")
    client.get_or_create_collection("company_context")
    assert rb.drop_generation(client, "") == ["company_context"]
