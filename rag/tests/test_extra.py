"""
追加テスト: OpenAI 429 リトライ、キャッシュ例外ハンドリング、CrewAI モック
"""
from unittest.mock import MagicMock, patch
import types

import pytest

# rag をパスに追加は conftest.py が担当しているためここでは不要
import main


class TestEmbedTextsRetry429:
    def test_retries_on_429_then_succeeds(self):
        class RateLimitError(Exception):
            pass

        mock_client = MagicMock()
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=[0.9])]

        # 1回目は 429 相当の例外、2回目で成功
        mock_client.embeddings.create.side_effect = [RateLimitError("429 Too Many Requests"), mock_response]

        with patch.dict("os.environ", {"OPENAI_API_KEY": "sk-test"}):
            with patch("main.OpenAI", return_value=mock_client):
                with patch("main.EMBED_MAX_RETRIES", 2):
                    with patch("time.sleep"):
                        result = main.embed_texts(["hello"])

        assert result == [[0.9]]
        assert mock_client.embeddings.create.call_count == 2


class TestCacheBehavior:
    def test_set_cached_context_handles_chromadb_exception(self):
        # get_chroma_client が例外を投げても set_cached_context は例外を伝播させない
        with patch("main.get_chroma_client", side_effect=Exception("chroma failure")):
            # 空でないドキュメントを渡しても例外は吸収される
            main.set_cached_context("key", ["doc1"])  # should not raise

    def test_set_cached_context_ignores_empty_docs(self):
        # ドキュメントが空のときは早期リターンして何もしない
        # ここでは単に例外が出ないことを確認する
        main.set_cached_context("key", [])


class TestRunCrewAI:
    def test_run_crewai_returns_string_from_mocked_crew(self):
        # main.Crew をモックして kickoff が所望の文字列を返すようにする
        class DummyCrew:
            def __init__(self, *args, **kwargs):
                pass

            def kickoff(self):
                return "【企業別レビュー報告書】\nモックレポート"

        with patch("main.Crew", DummyCrew):
            report = main.run_crewai(
                resume_text="経歴",
                company_name="テスト社",
                job_title="エンジニア",
                context_docs=["doc1"],
                context_source="cache",
            )

        assert isinstance(report, str)
        assert "モックレポート" in report
