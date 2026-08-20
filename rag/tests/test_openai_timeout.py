"""OpenAIクライアントへのタイムアウト設定の回帰テスト(#996)。

research.py/es_review.py/hints.py/resume.pyの複数箇所でOpenAI呼び出しに
明示的なタイムアウトが未設定で、応答が極端に遅延するとハングしうる状態
だった。OPENAI_TIMEOUT_SEC(RAG_OPENAI_TIMEOUT_SEC環境変数)が渡されることを
代表的な呼び出し元で確認する。
"""
import json
from unittest.mock import MagicMock, patch

import main
from services.es_review import _run_es_review
from services.settings import OPENAI_TIMEOUT_SEC


def test_run_deep_research_passes_timeout(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    mock_client = MagicMock()
    resp = MagicMock()
    resp.output_text = "結果"
    resp.choices = None
    resp.output = None
    resp.status = "completed"
    mock_client.responses.create.return_value = resp

    with patch("main.OpenAI", return_value=mock_client) as mock_openai_cls:
        main.run_deep_research("テスト社", "エンジニア")

    assert mock_openai_cls.call_args.kwargs["timeout"] == OPENAI_TIMEOUT_SEC


def test_run_es_review_passes_timeout(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    mock_client = MagicMock()
    resp = MagicMock()
    resp.choices = [MagicMock(message=MagicMock(content=json.dumps({
        "specificity_score": 5,
        "star_score": 5,
        "length_balance_score": 5,
        "feedback": "ok",
        "improved_text": "ok",
    })))]
    mock_client.chat.completions.create.return_value = resp

    with patch("main.OpenAI", return_value=mock_client) as mock_openai_cls:
        _run_es_review(es_text="hello", question_type="自己PR", company_name="", context_docs=[])

    assert mock_openai_cls.call_args.kwargs["timeout"] == OPENAI_TIMEOUT_SEC
