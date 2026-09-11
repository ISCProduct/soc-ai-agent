"""
RAG サービス単体・統合テスト

実行方法:
    cd rag && pytest tests/ -v

モジュールスタブ（crewai・chromadb）は conftest.py で一元管理している。
"""
import os
import re
from unittest.mock import MagicMock, patch

import pytest

import main


# ── 純粋関数テスト（モック不要） ────────────────────────────────────────────

class TestCosineSimilarity:
    def test_identical_vectors(self):
        assert main.cosine_similarity([1.0, 0.0], [1.0, 0.0]) == pytest.approx(1.0)

    def test_orthogonal_vectors(self):
        assert main.cosine_similarity([1.0, 0.0], [0.0, 1.0]) == pytest.approx(0.0)

    def test_opposite_vectors(self):
        assert main.cosine_similarity([1.0, 0.0], [-1.0, 0.0]) == pytest.approx(-1.0)

    def test_zero_vector_returns_zero(self):
        assert main.cosine_similarity([0.0, 0.0], [1.0, 0.0]) == 0.0

    def test_both_zero_returns_zero(self):
        assert main.cosine_similarity([0.0, 0.0], [0.0, 0.0]) == 0.0


class TestSanitizeCollectionName:
    def test_ascii_unchanged(self):
        name = main._sanitize_collection_name("company_abc-123")
        assert name == "company_abc-123"

    def test_special_chars_replaced(self):
        name = main._sanitize_collection_name("株式会社ABC::engineer")
        assert re.match(r"^[a-zA-Z0-9][a-zA-Z0-9_-]*[a-zA-Z0-9]$", name)

    def test_max_length_63(self):
        name = main._sanitize_collection_name("a" * 100)
        assert len(name) <= 63

    def test_min_length_3(self):
        name = main._sanitize_collection_name("ab")
        assert len(name) >= 3

    def test_double_colon_separator(self):
        name = main._sanitize_collection_name("CompanyXYZ::engineer")
        assert 3 <= len(name) <= 63


# ── embed_texts テスト（LangChain OpenAIEmbeddings モック） ─────────────────

class TestEmbedTexts:
    def _patch_embeddings(self, embed_documents_side_effect=None, return_value=None):
        mock_emb = MagicMock()
        if embed_documents_side_effect is not None:
            mock_emb.embed_documents.side_effect = embed_documents_side_effect
        else:
            mock_emb.embed_documents.return_value = return_value
        return patch("services.embed.get_embeddings", return_value=mock_emb), mock_emb

    def test_returns_embeddings(self):
        p, mock_emb = self._patch_embeddings(return_value=[[0.1, 0.2, 0.3]])
        with patch.dict("os.environ", {"OPENAI_API_KEY": "sk-test"}):
            with p:
                result = main.embed_texts(["hello world"])
        assert result == [[0.1, 0.2, 0.3]]
        mock_emb.embed_documents.assert_called_once()

    def test_multiple_texts(self):
        p, mock_emb = self._patch_embeddings(return_value=[[0.1], [0.2]])
        with patch.dict("os.environ", {"OPENAI_API_KEY": "sk-test"}):
            with p:
                result = main.embed_texts(["text1", "text2"])
        assert len(result) == 2
        mock_emb.embed_documents.assert_called_once()

    def test_retries_on_failure_then_succeeds(self):
        p, mock_emb = self._patch_embeddings(
            embed_documents_side_effect=[Exception("timeout"), [[0.5]]],
        )
        with patch.dict("os.environ", {"OPENAI_API_KEY": "sk-test"}):
            with p:
                with patch("main.EMBED_MAX_RETRIES", 2):
                    with patch("time.sleep"):
                        result = main.embed_texts(["test"])

        assert result == [[0.5]]
        assert mock_emb.embed_documents.call_count == 2

    def test_retry_wait_is_capped(self):
        """#994: 指数バックオフの待機時間が上限(5秒)で頭打ちになること
        (スレッドプール専有時間を抑えるため)。上限なしなら2**5=32秒になるはずの
        6回目の待機が5秒に切り詰められることを確認する。"""
        p, _mock_emb = self._patch_embeddings(
            embed_documents_side_effect=[Exception(f"e{i}") for i in range(6)] + [[[0.5]]],
        )
        with patch.dict("os.environ", {"OPENAI_API_KEY": "sk-test"}):
            with p:
                with patch("main.EMBED_MAX_RETRIES", 7):
                    with patch("time.sleep") as mock_sleep:
                        main.embed_texts(["test"])

        waits = [call.args[0] for call in mock_sleep.call_args_list]
        assert waits == [1, 2, 4, 5, 5, 5]  # 2**0..2**5(1,2,4,8,16,32)のうち8以降は5に頭打ち
        assert all(w <= 5 for w in waits)

    def test_raises_after_max_retries(self):
        p, mock_emb = self._patch_embeddings(
            embed_documents_side_effect=Exception("persistent error"),
        )
        with patch.dict("os.environ", {"OPENAI_API_KEY": "sk-test"}):
            with p:
                with patch("main.EMBED_MAX_RETRIES", 2):
                    with patch("time.sleep"):
                        with pytest.raises(Exception, match="persistent error"):
                            main.embed_texts(["test"])

        assert mock_emb.embed_documents.call_count == 2

    def test_raises_without_api_key(self):
        from fastapi import HTTPException
        env = {k: v for k, v in os.environ.items() if k != "OPENAI_API_KEY"}
        with patch.dict("os.environ", env, clear=True):
            with pytest.raises(HTTPException) as exc_info:
                main.embed_texts(["test"])
        assert exc_info.value.status_code == 500


# ── retrieve_docs テスト ─────────────────────────────────────────────────────

class TestRetrieveDocs:
    def test_returns_most_similar_first(self):
        # doc0 に似たクエリ → doc0 が先頭
        with patch("main.embed_texts") as mock_embed:
            mock_embed.return_value = [
                [1.0, 0.0],  # doc0
                [0.0, 1.0],  # doc1
                [1.0, 0.0],  # query (doc0 に類似)
            ]
            result = main.retrieve_docs(["doc0", "doc1"], "query")
        assert result[0] == "doc0"

    def test_empty_docs_returns_empty(self):
        assert main.retrieve_docs([], "query") == []

    def test_returns_at_most_5(self):
        docs = [f"doc{i}" for i in range(10)]
        embeddings = [[float(i), 0.0] for i in range(10)] + [[1.0, 0.0]]
        with patch("main.embed_texts", return_value=embeddings):
            result = main.retrieve_docs(docs, "query")
        assert len(result) <= 5


# ── 統合テスト（FastAPI TestClient） ────────────────────────────────────────

def _auth_client():
    """内部認証トークン付きの TestClient を返す (#615)"""
    from fastapi.testclient import TestClient
    return TestClient(main.app, headers={"X-Internal-Token": os.environ["RAG_INTERNAL_TOKEN"]})

class TestReviewEndpoint:
    def test_health(self):
        client = _auth_client()
        resp = client.get("/health")
        assert resp.status_code == 200
        body = resp.json()
        assert body["status"] == "ok"
        assert "vector_store" in body
        assert body["vector_store"]["ok"] is True
        assert "detail" in body["vector_store"]

    def test_review_success_web_search_path(self):

        with patch("main.get_cached_context", return_value=[]), \
             patch("main.USE_DEEP_RESEARCH", False), \
             patch("main.ALLOW_WEB_SEARCH_FALLBACK", True), \
             patch("main._run_async", return_value="企業の採用情報: チームワークを重視"), \
             patch("main.set_cached_context"), \
             patch("main.run_crewai", return_value="【企業別レビュー報告書】\nレポート内容"):

            client = _auth_client()
            resp = client.post("/resume/review", json={
                "resume_text": "テスト経歴書の内容です。",
                "company_name": "テスト株式会社",
                "job_title": "ソフトウェアエンジニア",
            })

        assert resp.status_code == 200
        body = resp.json()
        assert "report" in body
        assert len(body["report"]) > 0

    def test_review_uses_cache_on_second_call(self):

        cached_docs = ["企業の採用価値観: チームワーク重視"]

        with patch("main.get_cached_context", return_value=cached_docs), \
             patch("main.run_crewai", return_value="キャッシュヒット時のレポート") as mock_crewai:

            client = _auth_client()
            resp = client.post("/resume/review", json={
                "resume_text": "経歴テスト",
                "company_name": "キャッシュ企業",
                "job_title": "PM",
            })

        assert resp.status_code == 200
        call_kwargs = mock_crewai.call_args
        assert call_kwargs.kwargs.get("context_source") == "cache"
        assert call_kwargs.kwargs.get("context_docs") == cached_docs

    def test_review_no_context_fallback(self):
        """Deep Research・Web Search 両方無効時もコンテキストなしでレポートを返す。"""

        with patch("main.get_cached_context", return_value=[]), \
             patch("main.USE_DEEP_RESEARCH", False), \
             patch("main.ALLOW_WEB_SEARCH_FALLBACK", False), \
             patch("main.run_crewai", return_value="外部コンテキストなしのレポート") as mock_crewai:

            client = _auth_client()
            resp = client.post("/resume/review", json={
                "resume_text": "経歴テスト",
                "company_name": "テスト企業",
            })

        assert resp.status_code == 200
        call_kwargs = mock_crewai.call_args
        assert call_kwargs.kwargs.get("context_source") == "none"

    def test_review_prefers_company_context_over_web_search(self):
        """Backend brief（company_context）があるとき Search しない。"""

        with patch("main.get_cached_context", return_value=[]) as mock_cache, \
             patch("main.USE_DEEP_RESEARCH", True), \
             patch("main.ALLOW_WEB_SEARCH_FALLBACK", True), \
             patch("main._run_async") as mock_async, \
             patch("main.set_cached_context"), \
             patch("main.run_crewai", return_value="brief利用レポート") as mock_crewai:

            client = _auth_client()
            resp = client.post("/resume/review", json={
                "resume_text": "経歴テスト",
                "company_name": "テスト企業",
                "company_context": "企業名: テスト企業\n事業: SaaS",
            })

        assert resp.status_code == 200
        mock_async.assert_not_called()
        mock_cache.assert_not_called()
        call_kwargs = mock_crewai.call_args
        assert call_kwargs.kwargs.get("context_source") == "company_brief"
        assert call_kwargs.kwargs.get("context_docs") == ["企業名: テスト企業\n事業: SaaS"]

    def test_review_missing_resume_text(self):
        client = _auth_client()
        resp = client.post("/resume/review", json={
            "resume_text": "",
            "company_name": "テスト株式会社",
        })
        assert resp.status_code == 422

    def test_hints_skips_web_search_when_fallback_disabled(self):

        with patch("main.get_cached_context", return_value=[]), \
             patch("main.ALLOW_WEB_SEARCH_FALLBACK", False), \
             patch("main._run_hints_web_search") as mock_search, \
             patch("main._parse_hints_from_text", return_value=main.CompanyHintsResponse(
                 style_tags=[], top_questions=[], cached=False
             )) as mock_parse:

            client = _auth_client()
            resp = client.post("/company/hints", json={
                "company_name": "テスト企業",
                "position": "エンジニア",
            })

        assert resp.status_code == 200
        mock_search.assert_not_called()
        mock_parse.assert_called_once()

    def test_review_missing_company_name(self):
        client = _auth_client()
        resp = client.post("/resume/review", json={
            "resume_text": "経歴テスト",
            "company_name": "",
        })
        assert resp.status_code == 422


# ── 履歴書レビュー/面接ヒントの検索結果クロスキャッシュ ──────────────────────
# Web Search はツール呼び出し自体に加え検索結果が固定8,000トークン/callとして
# 課金される（OpenAI web_search tool）。同一企業に対し resume_review と
# interview_hints が別々に検索していたコストを、検索結果の共有で削減する。
# 実運用へ一気に展開せず、まず少数社（1〜5社）で挙動を確認する。
PILOT_COMPANIES = [
    "パイロット企業A",
    "パイロット企業B",
    "パイロット企業C",
    "パイロット企業D",
    "パイロット企業E",
]


class TestCrossFeatureWebSearchCacheSharing:
    """resume_review と interview_hints が Web 検索結果を共有し、
    どちらか一方が先に調査済みならもう一方は Web 検索を行わないことを検証する。"""

    @pytest.mark.parametrize("company_name", PILOT_COMPANIES)
    def test_resume_web_search_result_reused_by_hints(self, company_name):
        """resume/review で Web 検索した結果を、後続の company/hints がキャッシュヒットとして再利用する。"""
        from vector_store import build_cache_key

        role = "エンジニア"
        resume_key = build_cache_key("resume_review", company_name, role, company_original=company_name)
        hints_key = build_cache_key("interview_hints", company_name, role, company_original=company_name)
        store: dict[str, list[str]] = {}

        def fake_get(key, query="採用 価値観 求める人物像"):
            return store.get(key, [])

        def fake_set(key, docs, source="unknown", doc_type=None):
            store[key] = docs

        # 1回目: resume/review がキャッシュミスして Web 検索を実行する
        with patch("main.get_cached_context", side_effect=fake_get), \
             patch("main.set_cached_context", side_effect=fake_set), \
             patch("main.USE_DEEP_RESEARCH", False), \
             patch("main.ALLOW_WEB_SEARCH_FALLBACK", True), \
             patch("main._run_async", return_value=f"{company_name}の採用情報: チームワークを重視"), \
             patch("main.run_crewai", return_value="レポート"):

            client = _auth_client()
            resp = client.post("/resume/review", json={
                "resume_text": "テスト経歴書の内容です。",
                "company_name": company_name,
                "job_title": role,
            })
        assert resp.status_code == 200

        # Web検索結果が resume_review 側・interview_hints 側の両方のキーに保存されている
        assert resume_key in store
        assert hints_key in store
        assert store[hints_key] == store[resume_key]

        # 2回目: company/hints を呼んでも Web 検索は行われず、共有キャッシュから構造化するだけ
        with patch("main.get_cached_context", side_effect=fake_get), \
             patch("main.set_cached_context", side_effect=fake_set), \
             patch("main.ALLOW_WEB_SEARCH_FALLBACK", True), \
             patch("main._run_hints_web_search") as mock_search:

            client = _auth_client()
            resp = client.post("/company/hints", json={
                "company_name": company_name,
                "position": role,
            })
        assert resp.status_code == 200
        mock_search.assert_not_called()

    @pytest.mark.parametrize("company_name", PILOT_COMPANIES)
    def test_hints_web_search_result_reused_by_resume(self, company_name):
        """company/hints で Web 検索した結果を、後続の resume/review がキャッシュヒットとして再利用する。"""
        from vector_store import build_cache_key

        role = "エンジニア"
        resume_key = build_cache_key("resume_review", company_name, role, company_original=company_name)
        hints_key = build_cache_key("interview_hints", company_name, role, company_original=company_name)
        store: dict[str, list[str]] = {}

        def fake_get(key, query="採用 価値観 求める人物像"):
            return store.get(key, [])

        def fake_set(key, docs, source="unknown", doc_type=None):
            store[key] = docs

        # 1回目: company/hints がキャッシュミスして Web 検索を実行する
        with patch("main.get_cached_context", side_effect=fake_get), \
             patch("main.set_cached_context", side_effect=fake_set), \
             patch("main.ALLOW_WEB_SEARCH_FALLBACK", True), \
             patch("main._run_hints_web_search", return_value=f"{company_name}の面接情報: 深掘り質問が多い"):

            client = _auth_client()
            resp = client.post("/company/hints", json={
                "company_name": company_name,
                "position": role,
            })
        assert resp.status_code == 200

        assert hints_key in store
        assert resume_key in store
        assert store[resume_key] == store[hints_key]

        # 2回目: resume/review を呼んでも Web 検索は行われず、共有キャッシュを再利用するだけ
        with patch("main.get_cached_context", side_effect=fake_get), \
             patch("main.set_cached_context", side_effect=fake_set), \
             patch("main.USE_DEEP_RESEARCH", False), \
             patch("main.ALLOW_WEB_SEARCH_FALLBACK", True), \
             patch("main._run_async") as mock_async, \
             patch("main.run_crewai", return_value="レポート") as mock_crewai:

            client = _auth_client()
            resp = client.post("/resume/review", json={
                "resume_text": "テスト経歴書の内容です。",
                "company_name": company_name,
                "job_title": role,
            })
        assert resp.status_code == 200
        mock_async.assert_not_called()
        call_kwargs = mock_crewai.call_args
        assert call_kwargs.kwargs.get("context_source") == "cache"


class TestFailureCases:
    def test_chromadb_connection_failure_fallback(self):
        """ChromaDB 接続で例外が出てもエンドポイントが 200 を返すことを確認する。"""

        with patch("main.get_cached_context", side_effect=Exception("ChromaDB connection failed")), \
             patch("main.run_crewai", return_value="外部依存失敗時のレポート") as mock_crewai:

            client = _auth_client()
            resp = client.post("/resume/review", json={
                "resume_text": "経歴テスト",
                "company_name": "接続失敗企業",
            })

        assert resp.status_code == 200
        body = resp.json()
        assert "report" in body
        assert len(body["report"]) > 0
