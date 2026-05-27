"""
RAG サービス単体・統合テスト

実行方法:
    cd rag && pytest tests/ -v
"""
import re
import sys
import os
import types
from unittest.mock import MagicMock, patch

import pytest

# rag/ ディレクトリをパスに追加
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

# ── 重いモジュールをインポート前にモック ──────────────────────────────────────
# crewai はローカル環境に不要なため sys.modules でスタブ化する

_crewai_mock = types.ModuleType("crewai")
_crewai_mock.Agent = MagicMock()
_crewai_mock.Task = MagicMock()
_crewai_mock.Crew = MagicMock()
_crewai_mock.Process = MagicMock()
sys.modules.setdefault("crewai", _crewai_mock)

# ── main をインポート ──────────────────────────────────────────────────────────
import main  # noqa: E402  (after sys.modules stubs)


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


# ── embed_texts テスト（OpenAI モック） ─────────────────────────────────────

class TestEmbedTexts:
    def _make_mock_client(self, embeddings: list):
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=emb) for emb in embeddings]
        mock_client.embeddings.create.return_value = mock_response
        return mock_client

    def test_returns_embeddings(self):
        mock_client = self._make_mock_client([[0.1, 0.2, 0.3]])
        with patch.dict("os.environ", {"OPENAI_API_KEY": "sk-test"}):
            with patch("main.OpenAI", return_value=mock_client):
                result = main.embed_texts(["hello world"])
        assert result == [[0.1, 0.2, 0.3]]
        mock_client.embeddings.create.assert_called_once()

    def test_multiple_texts(self):
        mock_client = self._make_mock_client([[0.1], [0.2]])
        with patch.dict("os.environ", {"OPENAI_API_KEY": "sk-test"}):
            with patch("main.OpenAI", return_value=mock_client):
                result = main.embed_texts(["text1", "text2"])
        assert len(result) == 2

    def test_retries_on_failure_then_succeeds(self):
        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=[0.5])]
        mock_client.embeddings.create.side_effect = [Exception("timeout"), mock_response]

        with patch.dict("os.environ", {"OPENAI_API_KEY": "sk-test"}):
            with patch("main.OpenAI", return_value=mock_client):
                with patch("main.EMBED_MAX_RETRIES", 2):
                    with patch("time.sleep"):
                        result = main.embed_texts(["test"])

        assert result == [[0.5]]
        assert mock_client.embeddings.create.call_count == 2

    def test_raises_after_max_retries(self):
        mock_client = MagicMock()
        mock_client.embeddings.create.side_effect = Exception("persistent error")

        with patch.dict("os.environ", {"OPENAI_API_KEY": "sk-test"}):
            with patch("main.OpenAI", return_value=mock_client):
                with patch("main.EMBED_MAX_RETRIES", 2):
                    with patch("time.sleep"):
                        with pytest.raises(Exception, match="persistent error"):
                            main.embed_texts(["test"])

        assert mock_client.embeddings.create.call_count == 2

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

class TestReviewEndpoint:
    def test_health(self):
        from fastapi.testclient import TestClient
        client = TestClient(main.app)
        resp = client.get("/health")
        assert resp.status_code == 200
        assert resp.json() == {"status": "ok"}

    def test_review_success_web_search_path(self):
        from fastapi.testclient import TestClient

        with patch("main.get_cached_context", return_value=[]), \
             patch("main.USE_DEEP_RESEARCH", False), \
             patch("main.ALLOW_WEB_SEARCH_FALLBACK", True), \
             patch("main._run_async", return_value="企業の採用情報: チームワークを重視"), \
             patch("main.set_cached_context"), \
             patch("main.run_crewai", return_value="【企業別レビュー報告書】\nレポート内容"):

            client = TestClient(main.app)
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
        from fastapi.testclient import TestClient

        cached_docs = ["企業の採用価値観: チームワーク重視"]

        with patch("main.get_cached_context", return_value=cached_docs), \
             patch("main.run_crewai", return_value="キャッシュヒット時のレポート") as mock_crewai:

            client = TestClient(main.app)
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
        from fastapi.testclient import TestClient

        with patch("main.get_cached_context", return_value=[]), \
             patch("main.USE_DEEP_RESEARCH", False), \
             patch("main.ALLOW_WEB_SEARCH_FALLBACK", False), \
             patch("main.run_crewai", return_value="外部コンテキストなしのレポート") as mock_crewai:

            client = TestClient(main.app)
            resp = client.post("/resume/review", json={
                "resume_text": "経歴テスト",
                "company_name": "テスト企業",
            })

        assert resp.status_code == 200
        call_kwargs = mock_crewai.call_args
        assert call_kwargs.kwargs.get("context_source") == "none"

    def test_review_missing_resume_text(self):
        from fastapi.testclient import TestClient
        client = TestClient(main.app)
        resp = client.post("/resume/review", json={
            "resume_text": "",
            "company_name": "テスト株式会社",
        })
        assert resp.status_code == 422

    def test_review_missing_company_name(self):
        from fastapi.testclient import TestClient
        client = TestClient(main.app)
        resp = client.post("/resume/review", json={
            "resume_text": "経歴テスト",
            "company_name": "",
        })
        assert resp.status_code == 422


class TestFailureCases:
    def test_chromadb_connection_failure_fallback(self):
        """ChromaDB 接続で例外が出てもエンドポイントが 200 を返すことを確認する。"""
        from fastapi.testclient import TestClient

        with patch("main.get_cached_context", side_effect=Exception("ChromaDB connection failed")), \
             patch("main.run_crewai", return_value="外部依存失敗時のレポート") as mock_crewai:

            client = TestClient(main.app)
            resp = client.post("/resume/review", json={
                "resume_text": "経歴テスト",
                "company_name": "接続失敗企業",
            })

        assert resp.status_code == 200
        body = resp.json()
        assert "report" in body
        assert len(body["report"]) > 0
