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
    return all(meta.get(k) == v for k, v in where.items())


class FakeCollection:
    """テストの chromadb はスタブなので、使う操作（upsert / get / delete / count）だけを持つ偽物。"""

    def __init__(self, name: str, metadata=None) -> None:
        self.name = name
        self.metadata = metadata
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

    def get_or_create_collection(self, name, metadata=None):
        # 本物の Chroma と同じく、既存のコレクションへの metadata（距離関数）の指定は無視する
        return self.collections.setdefault(name, FakeCollection(name, metadata))

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
    ids = vs._doc_ids(key, len(docs))
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
    _seed(chroma, "company_context", "A社", "エンジニア", ["別の役割の旧文書"], "resume_review")
    _seed(chroma, "company_context", "B社", "指定なし", ["B社の文書"], "company_research")

    outcome = vs.replace_documents(key, ["新"], [EMB], source="web_search", doc_type="company_research")

    assert outcome == {"collection": "company_context", "replaced": 1, "deleted": 3}
    assert _docs(chroma, "company_context", "A社") == ["新"]
    # 別の企業は消さない（company と company_hash で絞る）
    assert _docs(chroma, "company_context", "B社") == ["B社の文書"]


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

    with patch.object(chroma, "get_or_create_collection", side_effect=lambda name, **kw: Unconfirmed(real(name, **kw))):
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
    def __init__(self, name, metadata=None, client=None):
        super().__init__(name, metadata)
        self.client = client

    def modify(self, name=None, metadata=None):
        if name:
            if self.client.fail_rename_to == name:
                self.client.fail_rename_to = None  # 1回だけ失敗させる
                raise RuntimeError("rename failed")
            assert name not in self.client.collections, f"{name} は既にある"
            del self.client.collections[self.name]
            self.name = name
            self.client.collections[name] = self

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
    def __init__(self) -> None:
        super().__init__()
        self.fail_rename_to = None

    def get_or_create_collection(self, name, metadata=None):
        return self.collections.setdefault(name, FakeFullCollection(name, metadata, self))

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


# --- 距離関数を cosine にする（SOCAIAGENT-554） -----------------------------------


def test_new_company_collections_are_created_with_cosine(chroma):
    key = vs.build_cache_key("company_research", "A社", company_original="A社")
    vs.set_cached_documents(key, ["本文"], [EMB], source="web_search", doc_type="company_research")
    vs.replace_documents(vs.build_cache_key("es_review", "B社", company_original="B社"), ["ES"], [EMB],
                         source="web_search", doc_type="es_review")

    assert chroma.collections["company_context"].metadata == {"hnsw:space": "cosine"}
    assert chroma.collections["es_review"].metadata == {"hnsw:space": "cosine"}


def test_existing_l2_collection_is_opened_as_is(chroma):
    # 既存の l2 のコレクションに cosine を指定しても、距離関数は変わらない（本物の Chroma も同じ。再構築で切り替える）
    chroma.get_or_create_collection("company_context")
    key = vs.build_cache_key("company_research", "A社", company_original="A社")
    vs.set_cached_documents(key, ["本文"], [EMB], source="web_search", doc_type="company_research")

    assert vs.collection_space(chroma.collections["company_context"]) == "l2"


@pytest.mark.parametrize("metadata,space", [(None, "l2"), ({"hnsw:space": "cosine"}, "cosine"), ({}, "l2"), ("壊れた値", "l2")])
def test_collection_space(metadata, space):
    class Col:
        pass

    col = Col()
    col.metadata = metadata
    assert vs.collection_space(col) == space


def test_rebuild_defaults_to_cosine_and_refuses_existing_target_with_other_space(monkeypatch):
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    monkeypatch.setattr(vs, "get_chroma_client", lambda: client)
    monkeypatch.delenv("RAG_COLLECTION_SUFFIX", raising=False)
    _seed(client, "company_context", "A社", "指定なし", ["研究"], "company_research")

    rb.copy_collections(client, "", "g2")
    assert client.collections["company_context_g2"].metadata == {"hnsw:space": "cosine"}

    # 写し先が l2 で既にあると、指定が黙って無視されるので止める
    client.get_or_create_collection("company_context_g3")
    with pytest.raises(RuntimeError, match="l2"):
        rb.copy_collections(client, "", "g3")


def _seed_students(client, n, metadata=None):
    col = client.get_or_create_collection("student_profiles", metadata=metadata)
    col.upsert(ids=[f"student-{i}" for i in range(n)], documents=[""] * n,
               embeddings=[[float(i), 1.0] for i in range(n)], metadatas=[{"user_id": i} for i in range(n)])
    return col


def test_rebuild_student_profiles_swaps_to_cosine_and_keeps_old(monkeypatch):
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    monkeypatch.setattr(rb, "BATCH", 2)  # ページングも通す
    _seed_students(client, 3)

    assert rb.rebuild_student_profiles(client)["status"] == "dry_run"
    assert "student_profiles__cos554" not in client.collections  # 確認だけでは書かない

    out = rb.rebuild_student_profiles(client, apply=True)

    assert out["status"] == "swapped" and out["copied"] == 3
    new = client.collections["student_profiles"]
    assert new.metadata == {"hnsw:space": "cosine"}
    assert sorted(new.rows) == ["student-0", "student-1", "student-2"]
    assert new.rows["student-2"]["embedding"] == [2.0, 1.0] and new.rows["student-2"]["metadata"] == {"user_id": 2}
    assert vs.collection_space(client.collections["student_profiles__l2old554"]) == "l2"
    assert rb.collection_status(client)["student_profiles"] == {"space": "cosine", "count": 3}

    # 2回目は何もしない。旧いコレクションは --drop-student-old で消す
    assert rb.rebuild_student_profiles(client, apply=True)["status"] == "already_cosine"
    assert rb.drop_student_old(client) == ["student_profiles__l2old554"]
    assert rb.drop_student_old(client) == []


def test_rebuild_student_profiles_restores_name_when_rename_fails():
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    _seed_students(client, 2)
    client.fail_rename_to = "student_profiles"  # 写しを student_profiles にする改名だけが失敗する

    with pytest.raises(RuntimeError):
        rb.rebuild_student_profiles(client, apply=True)

    # 元の名前に戻し、学生検索が空のコレクションを作らないようにする
    assert vs.collection_space(client.collections["student_profiles"]) == "l2"
    assert len(client.collections["student_profiles"].rows) == 2


def test_rebuild_student_profiles_stops_when_old_collection_is_left():
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    _seed_students(client, 1)
    client.get_or_create_collection("student_profiles__l2old554")
    with pytest.raises(RuntimeError, match="drop-student-old"):
        rb.rebuild_student_profiles(client, apply=True)


def _interrupt_after_old_rename(client):
    """元を旧い名前にした直後に止まった状態を作る（写しはそろっている）。"""
    from scripts import rebuild_collections as rb

    _seed_students(client, 3)
    swap = client.get_or_create_collection(rb.STUDENT_SWAP, metadata={"hnsw:space": "cosine"})
    rb._copy_all(client.collections["student_profiles"], swap)
    client.collections["student_profiles"].modify(name=rb.STUDENT_OLD)
    return rb


@pytest.mark.parametrize("service_created_empty", [False, True])
def test_rebuild_student_profiles_resumes_interrupted_swap(service_created_empty):
    client = FakeFullChroma()
    rb = _interrupt_after_old_rename(client)
    if service_created_empty:
        # 中断のあとに rag-review が起動し、空の cosine の student_profiles を作った
        client.get_or_create_collection("student_profiles", metadata={"hnsw:space": "cosine"})

    assert rb.rebuild_student_profiles(client) == {"status": "interrupted", "action": "resume", "old_count": 3}
    out = rb.rebuild_student_profiles(client, apply=True)

    assert out["status"] == "resumed"
    assert vs.collection_space(client.collections["student_profiles"]) == "cosine"
    assert len(client.collections["student_profiles"].rows) == 3
    assert rb.STUDENT_SWAP not in client.collections


def test_rebuild_student_profiles_rolls_back_when_swap_is_incomplete():
    client = FakeFullChroma()
    rb = _interrupt_after_old_rename(client)
    del client.collections[rb.STUDENT_SWAP].rows["student-0"]  # 写しが足りない

    out = rb.rebuild_student_profiles(client, apply=True)

    assert out == {"status": "rolled_back", "count": 3}
    assert vs.collection_space(client.collections["student_profiles"]) == "l2"
    assert len(client.collections["student_profiles"].rows) == 3
    # もう一度動かせば写し直して入れ替える
    assert rb.rebuild_student_profiles(client, apply=True)["status"] == "swapped"


def test_delete_student_also_removes_from_old_collection(monkeypatch):
    import student_profile_store as sps
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    monkeypatch.setattr(sps, "get_chroma_client", lambda: client)
    _seed_students(client, 2)
    rb.rebuild_student_profiles(client, apply=True)

    sps.delete_student(1)

    # 切り戻し用の旧いコレクションからも消える（切り戻しで同意撤回・退会した学生が戻らない）
    assert "student-1" not in client.collections["student_profiles"].rows
    assert "student-1" not in client.collections[rb.STUDENT_OLD].rows
    assert "student-0" in client.collections[rb.STUDENT_OLD].rows

    rb.drop_student_old(client)
    sps.delete_student(0)  # 旧いコレクションが無くても例外にしない
    assert "student-0" not in client.collections["student_profiles"].rows


@pytest.mark.parametrize("error", [ConnectionError("chroma down"), RuntimeError("timeout")])
def test_delete_student_raises_when_old_collection_delete_fails(monkeypatch, error):
    """旧いコレクションがあるのに消せないときは、握りつぶさずに失敗を返す（Backend にログが出る）。"""
    import student_profile_store as sps
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    monkeypatch.setattr(sps, "get_chroma_client", lambda: client)
    _seed_students(client, 2)
    rb.rebuild_student_profiles(client, apply=True)

    def broken_get(name, *a, **kw):
        raise error

    monkeypatch.setattr(client, "get_collection", broken_get)
    with pytest.raises(type(error)):
        sps.delete_student(1)


def test_delete_student_ignores_old_collection_dropped_after_listing(monkeypatch):
    """一覧を見たあとに旧いコレクションが消された（コレクションが無い）ときは、何もしない。"""
    import student_profile_store as sps
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    monkeypatch.setattr(sps, "get_chroma_client", lambda: client)
    _seed_students(client, 2)
    rb.rebuild_student_profiles(client, apply=True)

    def missing(name, *a, **kw):
        raise ValueError(f"Collection {name} does not exist.")

    monkeypatch.setattr(client, "get_collection", missing)
    assert sps.delete_student(1) == 1
    assert "student-1" not in client.collections["student_profiles"].rows


def test_prune_students_removes_unlisted_students_from_current_and_old(monkeypatch, tmp_path):
    """止めていた間に公開をやめた学生を、DB の一覧に合わせて今と旧いコレクションから消す。"""
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    monkeypatch.setattr(rb, "BATCH", 2)  # ページングも通す
    _seed_students(client, 5)
    rb.rebuild_student_profiles(client, apply=True)
    keep = tmp_path / "visible.txt"
    keep.write_text("\n".join(["# 公開してよい学生", "0", "2", "", "4", ""]), encoding="utf-8")

    assert rb.prune_students(client, rb._read_user_ids(str(keep))) == {
        "apply": False, "student_profiles": 2, rb.STUDENT_OLD: 2,
    }
    assert len(client.collections["student_profiles"].rows) == 5  # 確認だけでは消さない

    monkeypatch.setattr(rb.vs, "get_chroma_client", lambda: client)
    assert rb.main(["--prune-students", str(keep), "--apply"]) == 0
    for name in ("student_profiles", rb.STUDENT_OLD):
        assert sorted(client.collections[name].rows) == ["student-0", "student-2", "student-4"]


def test_prune_students_refuses_empty_list():
    from scripts import rebuild_collections as rb

    client = FakeFullChroma()
    _seed_students(client, 2)
    with pytest.raises(ValueError):
        rb.prune_students(client, set(), apply=True)
    assert len(client.collections["student_profiles"].rows) == 2
