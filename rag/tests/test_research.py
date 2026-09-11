"""services/research.py のテスト（Issue #942, #945）。

- #942: 日本語企業名を正規化すると全文字が除去され、公式ドメインの信頼度ブースト
  （0.95）が常に発火しないバグの再発防止。
- #945: run_deep_research のリトライロジック。メインリクエスト成功時はフォールバックを
  呼ばないこと、メイン失敗時はフォールバックを1回だけ試すこと、両方失敗した場合は
  フォールバックの例外が送出されることを検証する。
"""
from unittest.mock import MagicMock, patch

import pytest

import main


def test_domain_trust_score_japanese_company_name_matches_domain():
    """日本語企業名の正規化結果がドメインに含まれる場合は0.95になること。"""
    score = main._domain_trust_score("www.テスト商事.co.jp", "テスト商事")
    assert score == 0.95


def test_domain_trust_score_ascii_company_name_matches_domain():
    """ASCII企業名でのマッチングは引き続き0.95になること（回帰確認）。"""
    score = main._domain_trust_score("toyota.co.jp", "Toyota")
    assert score == 0.95


def test_domain_trust_score_japanese_company_name_not_in_domain():
    """日本語企業名がドメインに含まれない場合は0.95のブーストを受けないこと。"""
    score = main._domain_trust_score("example.com", "テスト商事")
    assert score != 0.95
    assert score == 0.5


def _make_response(text: str, status: str = "completed") -> MagicMock:
    resp = MagicMock()
    resp.output_text = text
    resp.choices = None
    resp.output = None
    resp.status = status
    return resp


def test_run_deep_research_primary_success_skips_fallback(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    mock_client = MagicMock()
    mock_client.responses.create.return_value = _make_response("メイン結果")

    with patch("main.OpenAI", return_value=mock_client):
        result = main.run_deep_research("テスト社", "エンジニア")

    assert result == "メイン結果"
    assert mock_client.responses.create.call_count == 1


def test_run_deep_research_falls_back_once_on_primary_failure(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.setenv("OPENAI_DEEP_RESEARCH_FALLBACK_MODEL", "gpt-4o")
    mock_client = MagicMock()
    mock_client.responses.create.side_effect = [
        Exception("primary failed"),
        _make_response("フォールバック結果"),
    ]

    with patch("main.OpenAI", return_value=mock_client):
        result = main.run_deep_research("テスト社", "エンジニア")

    assert result == "フォールバック結果"
    assert mock_client.responses.create.call_count == 2
    # 2回目はフォールバックモデル・tools無効で呼ばれていること
    second_call_kwargs = mock_client.responses.create.call_args_list[1].kwargs
    assert second_call_kwargs["model"] == "gpt-4o"
    assert "tools" not in second_call_kwargs


def test_run_deep_research_falls_back_once_on_empty_primary_response(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.setenv("OPENAI_DEEP_RESEARCH_FALLBACK_MODEL", "gpt-4o")
    mock_client = MagicMock()
    mock_client.responses.create.side_effect = [
        _make_response(""),
        _make_response("フォールバック結果"),
    ]

    with patch("main.OpenAI", return_value=mock_client):
        result = main.run_deep_research("テスト社", "エンジニア")

    assert result == "フォールバック結果"
    assert mock_client.responses.create.call_count == 2
    second_call_kwargs = mock_client.responses.create.call_args_list[1].kwargs
    assert second_call_kwargs["model"] == "gpt-4o"
    assert "tools" not in second_call_kwargs


def test_run_deep_research_falls_back_once_on_truncated_primary_response(monkeypatch):
    """#992: max_output_tokensで打ち切られた応答(status=incomplete)は、
    出力テキストが非空でも成功扱いにせずフォールバックすること。"""
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.setenv("OPENAI_DEEP_RESEARCH_FALLBACK_MODEL", "gpt-4o")
    mock_client = MagicMock()
    mock_client.responses.create.side_effect = [
        _make_response("途中で切れたレポート...", status="incomplete"),
        _make_response("フォールバック結果", status="completed"),
    ]

    with patch("main.OpenAI", return_value=mock_client):
        result = main.run_deep_research("テスト社", "エンジニア")

    assert result == "フォールバック結果"
    assert mock_client.responses.create.call_count == 2


def test_run_deep_research_raises_fallback_error_when_both_fail(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    mock_client = MagicMock()
    primary_err = Exception("primary failed")
    fallback_err = Exception("fallback failed")
    mock_client.responses.create.side_effect = [primary_err, fallback_err]

    with patch("main.OpenAI", return_value=mock_client):
        with pytest.raises(Exception) as exc_info:
            main.run_deep_research("テスト社", "エンジニア")
        assert exc_info.value is fallback_err

    assert mock_client.responses.create.call_count == 2
