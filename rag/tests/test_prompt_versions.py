"""RAG のプロンプトの版（services/prompt_versions.py、SOCAIAGENT-560）。"""
import logging
import re
from unittest.mock import MagicMock

import pytest

from services import prompt_versions


def test_versions_are_named_stage_vN():
    for stage, version in prompt_versions.PROMPT_VERSIONS.items():
        assert re.fullmatch(rf"{stage}_v\d+", version), (stage, version)


def test_unknown_stage_raises():
    with pytest.raises(KeyError):
        prompt_versions.prompt_version("no_such_stage")


def test_es_review_version_comes_from_registry():
    from services import es_review

    assert es_review._PROMPT_VERSION == prompt_versions.PROMPT_VERSIONS["es_review"]


def test_hints_parse_logs_stage_version_and_model(monkeypatch, caplog):
    """LLM を呼ぶたびに工程・版・モデルがログに残る（変更の前後比較・切り戻しを追えるように）。"""
    import json

    import main
    from services.hints import _parse_hints_from_text

    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    client = MagicMock()
    client.chat.completions.create.return_value.choices = [
        MagicMock(message=MagicMock(content=json.dumps({"style_tags": [], "top_questions": []})))
    ]
    monkeypatch.setattr(main, "OpenAI", lambda **_kw: client)
    with caplog.at_level(logging.INFO, logger="rag-review"):
        _parse_hints_from_text("テスト株式会社", "エンジニア", "リサーチ結果")
    assert "stage=hints_parse prompt_version=hints_parse_v1 model=" in caplog.text


def test_resume_report_logs_version(monkeypatch, caplog):
    from unittest.mock import patch

    from models import ReviewRequest
    from services.resume_report import generate_resume_report

    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    client = MagicMock()
    client.chat.completions.create.return_value.choices = [MagicMock(message=MagicMock(content="レポート"))]
    with patch("services.resume_report.OpenAI", return_value=client), \
            caplog.at_level(logging.INFO, logger="rag-review"):
        generate_resume_report(ReviewRequest(resume_text="経歴", company_name="A社"), [], "none", 1000)
    assert "stage=resume_review prompt_version=resume_review_v1" in caplog.text


def test_es_review_logs_version_for_each_call(monkeypatch, caplog):
    """ES 添削も、評価と改善文の呼び出しごとに版がログに出る（切れたときだけではない）。"""
    from tests.test_es_review_char_limit import _client, _run

    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    with caplog.at_level(logging.INFO, logger="rag-review"):
        _run(_client([100]))
    assert caplog.text.count("stage=es_review prompt_version=es_review_v6") == 2
