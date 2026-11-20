"""ベクトルストアの削除・更新・TTL 判定の整合（SOCAIAGENT-659）。

1. doc_type を指定した削除は、その用途の文書だけを消す（doc_type を持たない旧文書も消さない。U2）
2. 件数が減る更新で、前回の余ったチャンクが残らない（別の用途の文書は消さない）
3. TTL を検索の where で絞るので、期限切れの文書が上位を占めても有効な文書が返る。
   fetched_at_ts が無い文書は古い扱い（U1）。backfill で補えば当たる（U3）
4. 検索の失敗をログに残す
"""
import logging
from datetime import datetime, timedelta, timezone

import pytest

import vector_store as vs
from evaluation.search_quality.memory_store import MemoryClient
from scripts import backfill_fetched_at_ts as backfill

NEAR = [1.0, 0.0, 0.0]
FAR = [0.0, 1.0, 0.0]


@pytest.fixture
def client(monkeypatch):
    c = MemoryClient()
    monkeypatch.setattr(vs, "get_chroma_client", lambda: c)
    monkeypatch.delenv("RAG_COLLECTION_SUFFIX", raising=False)
    monkeypatch.delenv("RAG_TTL_WHERE", raising=False)
    return c


def _col(client, logical):
    return client.get_or_create_collection(vs.physical_collection_name(logical))


def _put(client, logical, doc_id, doc, meta, emb=NEAR):
    _col(client, logical).upsert(ids=[doc_id], documents=[doc], embeddings=[emb], metadatas=[meta])


def _docs(client, logical, where=None):
    return sorted(_col(client, logical).get(where=where)["documents"])


def _ts(delta: timedelta) -> int:
    return int((datetime.now(timezone.utc) + delta).timestamp())


# --- 1. doc_type 指定の削除 -------------------------------------------------------


def test_delete_with_doc_type_keeps_other_doc_types_and_legacy_docs(client):
    base = {"company": "A社", "company_hash": vs._company_hash("A社")}
    _put(client, "company_context", "r1", "添削用", {**base, "doc_type": "resume_review"})
    _put(client, "company_context", "c1", "企業研究用", {**base, "doc_type": "company_research"})
    _put(client, "company_context", "old", "doc_type の無い旧文書", dict(base))

    result = vs.delete_company_documents("A社", doc_type="company_research", company_original="A社")

    assert result["deleted"] == 1
    assert _docs(client, "company_context") == ["doc_type の無い旧文書", "添削用"]


def test_delete_without_doc_type_still_removes_all_company_docs(client):
    base = {"company": "A社", "company_hash": vs._company_hash("A社")}
    _put(client, "company_context", "r1", "添削用", {**base, "doc_type": "resume_review"})
    _put(client, "company_context", "old", "旧文書", dict(base))
    _put(client, "company_context", "b1", "B社", {"company": "B社", "company_hash": vs._company_hash("B社")})

    vs.delete_company_documents("A社", company_original="A社")

    assert _docs(client, "company_context") == ["B社"]


# --- 2. 件数が減る更新 ------------------------------------------------------------


def test_shrinking_update_removes_leftover_chunks(client):
    key = vs.build_cache_key("company_research", "A社", "エンジニア", company_original="A社")
    vs.set_cached_documents(key, ["旧1", "旧2", "旧3"], [NEAR] * 3, doc_type="company_research")

    vs.set_cached_documents(key, ["新1", "新2"], [NEAR] * 2, doc_type="company_research")

    assert _docs(client, "company_context") == ["新1", "新2"]


def test_shrinking_update_keeps_other_doc_type_with_same_cache_key(client):
    # resume_review と company_research は同じ cache_key・同じコレクションに並ぶ
    key = vs.build_cache_key("company_research", "A社", "エンジニア", company_original="A社")
    assert key == vs.build_cache_key("resume_review", "A社", "エンジニア", company_original="A社")
    vs.set_cached_documents(key, ["添削1", "添削2", "添削3"], [NEAR] * 3, doc_type="resume_review")

    vs.set_cached_documents(key, ["研究1"], [NEAR], doc_type="company_research")

    assert _docs(client, "company_context", {"doc_type": "resume_review"}) == ["添削2", "添削3"]
    assert _docs(client, "company_context", {"doc_type": "company_research"}) == ["研究1"]


def test_writes_record_numeric_fetched_at_ts(client):
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    before = _ts(timedelta(0))
    vs.set_cached_documents(key, ["文書"], [NEAR], doc_type="es_review")

    meta = _col(client, "es_review").get()["metadatas"][0]
    assert isinstance(meta["fetched_at_ts"], int)
    assert before - 1 <= meta["fetched_at_ts"] <= _ts(timedelta(seconds=1))


def test_replace_documents_records_numeric_fetched_at_ts(client):
    # 再埋め込み（SOCAIAGENT-658）で書き直した文書も、TTL の where に当たる
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    vs.replace_documents(key, ["新"], [NEAR], source="web_search", doc_type="es_review")

    assert vs.get_cached_documents(key, NEAR) == ["新"]
    assert isinstance(_col(client, "es_review").get()["metadatas"][0]["fetched_at_ts"], int)


# --- 3. TTL を where で絞る ------------------------------------------------------


def _seed_expired_and_fresh(client, key):
    meta = vs.parse_cache_key(key)
    base = {"company": meta["company"], "company_hash": meta["company_hash"], "role": meta["role"],
            "doc_type": "es_review", "cache_key": key}
    old = (datetime.now(timezone.utc) - timedelta(seconds=vs.CACHE_TTL_SECONDS + 3600))
    for i in range(3):
        # 期限切れの文書のほうが類似度が高い（上位を占める）
        _put(client, "es_review", f"old{i}", f"期限切れ{i}",
             {**base, "fetched_at": old.isoformat(), "fetched_at_ts": int(old.timestamp())}, emb=NEAR)
    now = datetime.now(timezone.utc)
    _put(client, "es_review", "fresh", "有効",
         {**base, "fetched_at": now.isoformat(), "fetched_at_ts": int(now.timestamp())}, emb=FAR)


def test_expired_docs_ranked_higher_do_not_hide_fresh_docs(client):
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    _seed_expired_and_fresh(client, key)

    assert vs.get_cached_documents(key, NEAR, n_results=2) == ["有効"]


def test_ttl_where_can_be_switched_off_for_rollback(client, monkeypatch):
    monkeypatch.setenv("RAG_TTL_WHERE", "0")
    where = vs._build_where({"company": "A社", "role": "x"})
    assert vs._with_ttl(where) == where


def test_with_ttl_appends_clause_to_and_and_single_filters(client):
    single = vs._with_ttl({"company": "A社"})
    assert single["$and"][0] == {"company": "A社"}
    assert set(single["$and"][1]["fetched_at_ts"]) == {"$gte"}
    multi = vs._with_ttl({"$and": [{"company": "A社"}, {"role": "x"}]})
    assert multi["$and"][:2] == [{"company": "A社"}, {"role": "x"}]
    assert len(multi["$and"]) == 3


def test_docs_without_fetched_at_ts_are_stale_until_backfilled(client):
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    meta = vs.parse_cache_key(key)
    base = {"company": meta["company"], "company_hash": meta["company_hash"], "role": meta["role"],
            "doc_type": "es_review", "cache_key": key}
    _put(client, "es_review", "legacy", "旧形式", {**base, "fetched_at": datetime.now(timezone.utc).isoformat()})
    _put(client, "es_review", "broken", "壊れた日付", {**base, "fetched_at": "not-a-date"})

    assert vs.get_cached_documents(key, NEAR) == []  # U1: fetched_at_ts が無ければ古い扱い

    backfill.backfill(client, apply=True)

    assert vs.get_cached_documents(key, NEAR) == ["旧形式"]


# --- 4. 検索の失敗をログに残す ---------------------------------------------------


class _Failing:
    def query(self, **kwargs):
        raise ValueError("boom")


def test_query_failure_is_logged(caplog):
    with caplog.at_level(logging.WARNING, logger="rag-review"):
        assert vs._query_with_where(_Failing(), NEAR, {"company": "A社"}, 5) == []
    assert any("chromadb query failed" in r.getMessage() and "boom" in r.getMessage() for r in caplog.records)


def test_query_failure_with_fallback_is_logged(caplog):
    with caplog.at_level(logging.WARNING, logger="rag-review"):
        assert vs._query_with_where(_Failing(), NEAR, {"company": "A社"}, 5, fallback_where={"company": "A社"}) == []
    assert any("failed twice" in r.getMessage() for r in caplog.records)


# --- backfill（U3） -------------------------------------------------------------


def _seed_for_backfill(client):
    now = datetime.now(timezone.utc)
    _put(client, "company_context", "ok", "読める", {"company": "A社", "fetched_at": now.isoformat()})
    _put(client, "company_context", "z", "Z 表記", {"company": "A社", "fetched_at": "2026-11-01T00:00:00Z"})
    _put(client, "company_context", "done", "補い済み", {"company": "A社", "fetched_at": "x", "fetched_at_ts": 123})
    _put(client, "company_context", "none", "取得時刻なし", {"company": "A社"})
    _put(client, "company_context", "bad", "壊れた", {"company": "A社", "fetched_at": "not-a-date"})
    future = now + timedelta(days=30)
    _put(client, "company_context", "future", "未来", {"company": "A社", "fetched_at": future.isoformat()})


def _meta(client, doc_id):
    return _col(client, "company_context").get(ids=[doc_id])["metadatas"][0]


def test_backfill_is_dry_run_by_default(client):
    _seed_for_backfill(client)

    result = backfill.backfill(client)

    assert result["company_context"] == {"scanned": 6, "already": 1, "filled": 2, "unreadable": 3}
    assert "fetched_at_ts" not in _meta(client, "ok")


def test_backfill_apply_fills_only_readable_and_keeps_other_metadata(client):
    _seed_for_backfill(client)

    backfill.backfill(client, apply=True)

    assert _meta(client, "z") == {
        "company": "A社", "fetched_at": "2026-11-01T00:00:00Z",
        "fetched_at_ts": int(datetime(2026, 11, 1, tzinfo=timezone.utc).timestamp()),
    }
    assert _meta(client, "done")["fetched_at_ts"] == 123
    for doc_id in ("none", "bad", "future"):
        assert "fetched_at_ts" not in _meta(client, doc_id)
    # 何度流しても同じ
    again = backfill.backfill(client, apply=True)
    assert again["company_context"]["filled"] == 0


def test_backfill_reads_in_batches(client, monkeypatch):
    monkeypatch.setattr(backfill, "BATCH", 2)
    _seed_for_backfill(client)

    result = backfill.backfill(client, apply=True)

    assert result["company_context"]["scanned"] == 6
    assert "fetched_at_ts" in _meta(client, "ok")


def test_backfill_main_requires_apply_to_write(client, capsys):
    _seed_for_backfill(client)

    assert backfill.main([]) == 0
    assert "fetched_at_ts" not in _meta(client, "ok")
    assert backfill.main(["--apply"]) == 0
    assert "fetched_at_ts" in _meta(client, "ok")
