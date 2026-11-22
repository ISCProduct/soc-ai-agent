"""ベクトルストアの削除・更新・TTL 判定の整合（SOCAIAGENT-659）。

1. doc_type を指定した削除は、その用途の文書だけを消す（doc_type を持たない旧文書も消さない。U2）
2. 件数が減る更新で、前回の余ったチャンクが残らない。文書の ID は用途ごとに分かれ、
   同じ cache_key の別の用途の文書は上書きも削除もしない
3. TTL を検索の where で絞るので、期限切れの文書が上位を占めても有効な文書が返る。
   fetched_at_ts が無い文書は古い扱い（U1）。backfill で補えば当たる（U3）
4. 検索の失敗をログに残す
"""
import logging
from datetime import datetime, timedelta, timezone

import pytest

import vector_store as vs
from evaluation.search_quality.memory_store import MemoryClient
from scripts import backfill_fetched_at_ts as backfill_cli

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


@pytest.fixture
def ticking_clock(monkeypatch):
    """vector_store の datetime.now を、呼ぶたびに1秒進む時計にする（時計の分解能に依存させない）。"""
    base = [datetime(2026, 11, 20, tzinfo=timezone.utc)]

    class Tick(datetime):
        @classmethod
        def now(cls, tz=None):
            base[0] += timedelta(seconds=1)
            return base[0]

    monkeypatch.setattr(vs, "datetime", Tick)
    return base


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


def test_shrinking_update_removes_leftover_chunks(client, ticking_clock):
    key = vs.build_cache_key("company_research", "A社", "エンジニア", company_original="A社")
    vs.set_cached_documents(key, ["旧1", "旧2", "旧3"], [NEAR] * 3, doc_type="company_research")

    vs.set_cached_documents(key, ["新1", "新2"], [NEAR] * 2, doc_type="company_research")

    assert _docs(client, "company_context") == ["新1", "新2"]


def test_other_doc_type_with_same_cache_key_is_neither_overwritten_nor_deleted(client, ticking_clock):
    # resume_review と company_research は同じ cache_key・同じコレクションに並ぶ。ID を用途ごとに
    # 分けたので、片方を書いても、もう片方のチャンクは上書きも削除もされない（レビューの Major 2）
    key = vs.build_cache_key("company_research", "A社", "エンジニア", company_original="A社")
    assert key == vs.build_cache_key("resume_review", "A社", "エンジニア", company_original="A社")
    assert vs._doc_ids(key, 1, "resume_review") != vs._doc_ids(key, 1, "company_research")
    vs.set_cached_documents(key, ["添削1", "添削2", "添削3"], [NEAR] * 3, doc_type="resume_review")

    vs.set_cached_documents(key, ["研究1"], [NEAR], doc_type="company_research")
    vs.set_cached_documents(key, ["研究2"], [NEAR], doc_type="company_research")

    assert _docs(client, "company_context", {"doc_type": "resume_review"}) == ["添削1", "添削2", "添削3"]
    assert _docs(client, "company_context", {"doc_type": "company_research"}) == ["研究2"]


def test_legacy_ids_without_doc_type_are_cleaned_up_on_next_write(client, ticking_clock):
    # doc_type を含めない旧 ID の文書（ID を分ける前に書かれたもの）は、次の書き込みの掃除で消える
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    meta = vs.parse_cache_key(key)
    legacy_id = vs._doc_ids(key, 1)[0]
    assert legacy_id != vs._doc_ids(key, 1, "es_review")[0]
    _put(client, "es_review", legacy_id, "旧 ID の文書",
         {"company": meta["company"], "company_hash": meta["company_hash"], "role": meta["role"],
          "doc_type": "es_review", "cache_key": key, "fetched_at": "2026-11-01T00:00:00+00:00"})

    vs.set_cached_documents(key, ["新"], [NEAR], doc_type="es_review")

    assert _docs(client, "es_review") == ["新"]


def test_leftover_chunks_without_fetched_at_are_removed(client):
    # fetched_at を持たない余りのチャンクは、いちばん古い扱いで消す
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    meta = vs.parse_cache_key(key)
    _put(client, "es_review", vs._doc_ids(key, 3, "es_review")[2], "取得時刻の無い余り",
         {"company": meta["company"], "company_hash": meta["company_hash"], "role": meta["role"],
          "doc_type": "es_review", "cache_key": key})

    vs.set_cached_documents(key, ["新1", "新2"], [NEAR] * 2, doc_type="es_review")

    assert _docs(client, "es_review") == ["新1", "新2"]


def test_stale_cleanup_failure_is_logged_separately_from_write(client, monkeypatch, caplog):
    # upsert は済んでいるので、余りの掃除の失敗を「書き込みの失敗」と記録しない
    key = vs.build_cache_key("es_review", "A社", company_original="A社")

    def broken(*args, **kwargs):
        raise RuntimeError("cleanup boom")

    monkeypatch.setattr(vs, "_delete_stale_chunks", broken)
    with caplog.at_level(logging.WARNING, logger="rag-review"):
        vs.set_cached_documents(key, ["新"], [NEAR], doc_type="es_review")

    assert _docs(client, "es_review") == ["新"]
    messages = [r.getMessage() for r in caplog.records]
    assert any("stale cleanup failed" in m and "cleanup boom" in m for m in messages)
    assert not any("chromadb set failed" in m for m in messages)


def test_shrinking_update_keeps_chunks_written_later_by_another_request(client):
    # A（2件）と B（3件）が同じキーを同時に書き、B の書き込みが A の掃除より後に入った場合。
    # A は自分より後に書かれた B の3つ目を「余り」として消さない（Codex の指摘）
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    meta = vs.parse_cache_key(key)
    later = datetime.now(timezone.utc) + timedelta(seconds=2)
    _put(client, "es_review", vs._doc_ids(key, 3, "es_review")[2], "B の3つ目",
         {"company": meta["company"], "company_hash": meta["company_hash"], "role": meta["role"],
          "doc_type": "es_review", "cache_key": key, "fetched_at": later.isoformat(),
          "fetched_at_ts": int(later.timestamp())})

    vs.set_cached_documents(key, ["A1", "A2"], [NEAR] * 2, doc_type="es_review")

    assert _docs(client, "es_review") == ["A1", "A2", "B の3つ目"]


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


def test_future_fetched_at_ts_docs_do_not_hide_fresh_docs(client):
    # 時計のずれを超える未来の取得時刻は壊れた文書。後段の _is_fresh で捨てられるので、
    # 検索の段階で外さないと上位を占めて有効な文書が返らない（Codex の指摘）
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    meta = vs.parse_cache_key(key)
    base = {"company": meta["company"], "company_hash": meta["company_hash"], "role": meta["role"],
            "doc_type": "es_review", "cache_key": key}
    future = datetime.now(timezone.utc) + timedelta(days=1)
    for i in range(3):
        _put(client, "es_review", f"future{i}", f"未来{i}",
             {**base, "fetched_at": future.isoformat(), "fetched_at_ts": int(future.timestamp())}, emb=NEAR)
    now = datetime.now(timezone.utc)
    _put(client, "es_review", "fresh", "有効",
         {**base, "fetched_at": now.isoformat(), "fetched_at_ts": int(now.timestamp())}, emb=FAR)

    assert vs.get_cached_documents(key, NEAR, n_results=2) == ["有効"]


def test_ttl_where_can_be_switched_off_for_rollback(client, monkeypatch):
    monkeypatch.setenv("RAG_TTL_WHERE", "0")
    where = vs._build_where({"company": "A社", "role": "x"})
    assert vs._with_ttl(where) == where


def test_with_ttl_appends_clause_to_and_and_single_filters(client):
    # Chroma は1つの条件に演算子を1つしか持てないので、下限と上限は別の条件にする
    single = vs._with_ttl({"company": "A社"})
    assert single["$and"][0] == {"company": "A社"}
    assert [set(c["fetched_at_ts"]) for c in single["$and"][1:]] == [{"$gte"}, {"$lte"}]
    multi = vs._with_ttl({"$and": [{"company": "A社"}, {"role": "x"}]})
    assert multi["$and"][:2] == [{"company": "A社"}, {"role": "x"}]
    assert len(multi["$and"]) == 4
    assert [set(c["fetched_at_ts"]) for c in vs._with_ttl({})["$and"]] == [{"$gte"}, {"$lte"}]


NOW = 1_800_000_000


def test_with_ttl_uses_exact_window(client, monkeypatch):
    # 下限は「今 - TTL」、上限は「今 + 時計のずれの許容（300秒）」
    monkeypatch.setattr(vs, "_now_ts", lambda: NOW)
    assert vs._with_ttl({"company": "A社"}, ttl_seconds=3600) == {"$and": [
        {"company": "A社"},
        {"fetched_at_ts": {"$gte": NOW - 3600}},
        {"fetched_at_ts": {"$lte": NOW + 300}},
    ]}


@pytest.mark.parametrize("offset, hit", [
    (-vs.CACHE_TTL_SECONDS, True),        # TTL ちょうど
    (-vs.CACHE_TTL_SECONDS - 1, False),   # TTL を1秒過ぎた
    (300, True),                          # 時計のずれの許容ちょうど
    (301, False),                         # 許容を1秒超えた未来
])
def test_ttl_boundaries(client, monkeypatch, offset, hit):
    # where と後段の確認が同じ整数の範囲で判定するので、境界でも食い違わない
    monkeypatch.setattr(vs, "_now_ts", lambda: NOW)
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    meta = vs.parse_cache_key(key)
    fetched = datetime.fromtimestamp(NOW + offset, tz=timezone.utc)
    _put(client, "es_review", "doc", "境界の文書",
         {"company": meta["company"], "company_hash": meta["company_hash"], "role": meta["role"],
          "doc_type": "es_review", "cache_key": key, "fetched_at": fetched.isoformat(),
          "fetched_at_ts": NOW + offset})

    assert vs.get_cached_documents(key, NEAR) == (["境界の文書"] if hit else [])


def test_naive_fetched_at_is_treated_as_utc():
    # タイムゾーンの無い日時は、TTL の判定でも backfill でも UTC とみなす
    naive = vs._parse_fetched_at("2026-11-01T00:00:00")
    assert naive == datetime(2026, 11, 1, tzinfo=timezone.utc)
    assert vs.fetched_at_to_ts("2026-11-01T00:00:00") == int(naive.timestamp())


def test_docs_without_fetched_at_ts_are_stale_until_backfilled(client):
    key = vs.build_cache_key("es_review", "A社", company_original="A社")
    meta = vs.parse_cache_key(key)
    base = {"company": meta["company"], "company_hash": meta["company_hash"], "role": meta["role"],
            "doc_type": "es_review", "cache_key": key}
    _put(client, "es_review", "legacy", "旧形式", {**base, "fetched_at": datetime.now(timezone.utc).isoformat()})
    _put(client, "es_review", "broken", "壊れた日付", {**base, "fetched_at": "not-a-date"})

    assert vs.get_cached_documents(key, NEAR) == []  # U1: fetched_at_ts が無ければ古い扱い

    vs.backfill_fetched_at_ts(client, apply=True)

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

    result = vs.backfill_fetched_at_ts(client)

    assert result["company_context"] == {"scanned": 6, "already": 1, "filled": 2, "unreadable": 3}
    assert "fetched_at_ts" not in _meta(client, "ok")


def test_backfill_apply_fills_only_readable_and_keeps_other_metadata(client):
    _seed_for_backfill(client)

    vs.backfill_fetched_at_ts(client, apply=True)

    assert _meta(client, "z") == {
        "company": "A社", "fetched_at": "2026-11-01T00:00:00Z",
        "fetched_at_ts": int(datetime(2026, 11, 1, tzinfo=timezone.utc).timestamp()),
    }
    assert _meta(client, "done")["fetched_at_ts"] == 123
    for doc_id in ("none", "bad", "future"):
        assert "fetched_at_ts" not in _meta(client, doc_id)
    # 何度流しても同じ
    again = vs.backfill_fetched_at_ts(client, apply=True)
    assert again["company_context"]["filled"] == 0


def test_backfill_reads_in_batches(client, monkeypatch):
    monkeypatch.setattr(vs, "BACKFILL_BATCH", 2)
    _seed_for_backfill(client)

    result = vs.backfill_fetched_at_ts(client, apply=True)

    assert result["company_context"]["scanned"] == 6
    assert "fetched_at_ts" in _meta(client, "ok")


def test_backfill_main_requires_apply_to_write(client, capsys):
    _seed_for_backfill(client)

    assert backfill_cli.main([]) == 0
    assert "fetched_at_ts" not in _meta(client, "ok")
    assert backfill_cli.main(["--apply"]) == 0
    assert "fetched_at_ts" in _meta(client, "ok")


def test_backfill_continues_after_one_collection_fails(client, caplog):
    # 1つのコレクションで失敗しても、残りのコレクションは補う。失敗した場所を結果とログに残す
    _seed_for_backfill(client)
    meta = {"company": "A社", "fetched_at": datetime.now(timezone.utc).isoformat()}
    _put(client, "es_review", "es", "ES", dict(meta))
    hints = _col(client, "interview_hints")
    _put(client, "interview_hints", "h", "ヒント", dict(meta))

    def broken_get(**kwargs):
        raise RuntimeError("hints down")

    hints.get = broken_get
    with caplog.at_level(logging.WARNING, logger="rag-review"):
        result = vs.backfill_fetched_at_ts(client, apply=True)

    assert result["interview_hints"]["error"] == "RuntimeError"
    assert vs.backfill_failed_collections(result) == ["interview_hints"]
    assert result["company_context"]["filled"] == 2
    assert "fetched_at_ts" in _col(client, "es_review").get(ids=["es"])["metadatas"][0]
    assert any("backfill failed" in r.getMessage() and "interview_hints" in r.getMessage() for r in caplog.records)


def test_backfill_updates_only_fetched_at_ts(client):
    # update には fetched_at_ts だけを渡す。読んでから update するまでに同じ ID が書き直されても、
    # 新しい本文に古いメタデータ（fetched_at・doc_type・role）を付けない
    _seed_for_backfill(client)
    col = _col(client, "company_context")
    sent = []
    real_update = col.update

    def spy(ids, metadatas=None, **kwargs):
        sent.extend(metadatas or [])
        real_update(ids=ids, metadatas=metadatas, **kwargs)

    col.update = spy
    vs.backfill_fetched_at_ts(client, apply=True)

    assert sent and all(set(m) == {"fetched_at_ts"} for m in sent)
    assert _meta(client, "ok")["company"] == "A社"  # 渡さなかった項目は残る（マージ）


def test_backfill_does_not_skip_docs_when_others_are_deleted_meanwhile(client, monkeypatch):
    # 起動時は並行して余りの掃除などが走る。ID を先に集めるので、途中で文書が消えても読み飛ばさない
    monkeypatch.setattr(vs, "BACKFILL_BATCH", 2)
    now = datetime.now(timezone.utc).isoformat()
    for i in range(5):
        _put(client, "company_context", f"d{i}", f"文書{i}", {"company": "A社", "fetched_at": now})
    col = _col(client, "company_context")
    real_get = col.get
    calls = []

    def get_and_delete_first(**kwargs):
        got = real_get(**kwargs)
        if "metadatas" in (kwargs.get("include") or []):
            calls.append(1)
            if len(calls) == 1:
                col.delete(ids=["d0"])  # 1回目を読んだ直後に、読んだ分の1件が消える
        return got

    col.get = get_and_delete_first
    vs.backfill_fetched_at_ts(client, apply=True)

    for i in range(1, 5):
        assert "fetched_at_ts" in real_get(ids=[f"d{i}"])["metadatas"][0], f"d{i} を読み飛ばした"


def test_search_quality_harness_checks_ttl(monkeypatch):
    # 評価の期限切れ文書は数値の取得時刻を持つので、TTL の下限を外すと期限切れが返って検出される
    from evaluation.search_quality.dataset import default_dataset_path, load_dataset
    from evaluation.search_quality.embedders import hash_embedder
    from evaluation.search_quality.runner import evaluate_company_research

    dataset = load_dataset(default_dataset_path("company_research"))
    assert evaluate_company_research(dataset, hash_embedder(), MemoryClient())["stale_documents"] == 0
    monkeypatch.setattr(vs, "_ttl_window", lambda now, ttl: (0, now + 300))
    assert evaluate_company_research(dataset, hash_embedder(), MemoryClient())["stale_documents"] > 0


# --- 起動時の backfill（Codex の P1・レビューの Mid） ---------------------------


def test_startup_backfill_fills_existing_docs(client, monkeypatch):
    import main

    monkeypatch.setattr(main, "get_chroma_client", lambda: client)
    _seed_for_backfill(client)

    main._run_fetched_at_ts_backfill()

    assert "fetched_at_ts" in _meta(client, "ok")


def test_startup_backfill_failure_does_not_raise(monkeypatch, caplog):
    import main

    def broken():
        raise RuntimeError("chroma down")

    sleeps = []
    monkeypatch.setattr(main, "get_chroma_client", broken)
    monkeypatch.setattr(main.time, "sleep", sleeps.append)
    monkeypatch.setenv("RAG_BACKFILL_ATTEMPTS", "3")
    monkeypatch.setenv("RAG_BACKFILL_RETRY_DELAY_SEC", "5")
    with caplog.at_level(logging.WARNING, logger="rag-review"):
        main._run_fetched_at_ts_backfill()
    messages = [r.getMessage() for r in caplog.records]
    assert sum("backfill failed" in m for m in messages) == 3
    assert any("gave up after 3 attempts" in m for m in messages)
    assert sleeps == [5, 10]  # 待ちは倍々にする


def test_startup_backfill_retries_until_chroma_is_reachable(client, monkeypatch):
    # chroma は独立したサービスなので、起動時にまだ届かないことがある。届くまで試し直す
    import main

    _seed_for_backfill(client)
    attempts = []

    def flaky():
        attempts.append(1)
        if len(attempts) < 3:
            raise RuntimeError("chroma not ready")
        return client

    monkeypatch.setattr(main, "get_chroma_client", flaky)
    monkeypatch.setattr(main.time, "sleep", lambda s: None)
    monkeypatch.setenv("RAG_BACKFILL_ATTEMPTS", "4")
    main._run_fetched_at_ts_backfill()

    assert len(attempts) == 3
    assert "fetched_at_ts" in _meta(client, "ok")


def test_startup_backfill_retries_when_a_collection_failed(client, monkeypatch):
    import main

    _seed_for_backfill(client)
    results = [{"company_context": {"error": "RuntimeError"}}, {"company_context": {"filled": 2}}]
    calls = []

    def fake_backfill(c, apply):
        calls.append(apply)
        return results[len(calls) - 1]

    monkeypatch.setattr(main, "get_chroma_client", lambda: client)
    monkeypatch.setattr(main, "backfill_fetched_at_ts", fake_backfill)
    monkeypatch.setattr(main.time, "sleep", lambda s: None)
    main._run_fetched_at_ts_backfill()

    assert calls == [True, True]


@pytest.mark.parametrize("value, started", [("1", True), ("", True), ("0", False), ("off", False)])
def test_startup_backfill_runs_in_background_unless_disabled(monkeypatch, value, started):
    import main

    calls = []

    class FakeThread:
        def __init__(self, target, name, daemon):
            calls.append((target, daemon))

        def start(self):
            calls.append("start")

    monkeypatch.setattr(main.threading, "Thread", FakeThread)
    if value:
        monkeypatch.setenv("RAG_BACKFILL_ON_STARTUP", value)
    else:
        monkeypatch.delenv("RAG_BACKFILL_ON_STARTUP", raising=False)

    main.start_fetched_at_ts_backfill()

    if started:
        assert calls == [(main._run_fetched_at_ts_backfill, True), "start"]
    else:
        assert calls == []
