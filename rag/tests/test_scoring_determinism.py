"""採点の呼び出しだけ温度を下げて seed を固定する（SOCAIAGENT-699）。

改善文の生成は表現の幅が要るので、従来の温度のまま seed も渡さない。
"""
import asyncio
import json
from unittest.mock import MagicMock, patch

from models import ReviewRequest
from routers.resume import review_resume_stream
from services.es_review import _run_es_review
from services.resume_report import generate_resume_report
from services.settings import SCORING_SEED, SCORING_TEMPERATURE

_REVIEW = {
    "specificity_score": 7,
    "star_score": 6,
    "company_fit_score": None,
    "length_balance_score": 5,
    "feedback": "具体性を高めましょう",
    "feedback_items": [],
    "company_strategy": "",
}
_IMPROVED = {"improved_text": "改善後の文章。具体的な行動を書き足した。"}


def _es_client() -> MagicMock:
    def _create(**kwargs):
        user = next(m["content"] for m in kwargs["messages"] if m["role"] == "user")
        payload = _IMPROVED if "【添削フィードバック】" in user else _REVIEW
        return MagicMock(
            choices=[MagicMock(message=MagicMock(content=json.dumps(payload, ensure_ascii=False)), finish_reason="stop")]
        )

    client = MagicMock()
    client.chat.completions.create.side_effect = _create
    return client


def test_es_review_scoring_call_is_low_temperature_with_seed_and_rewrite_is_not():
    client = _es_client()
    with patch("main.OpenAI", return_value=client):
        _run_es_review(es_text="学生時代に力を入れたことは、", question_type="自己PR", company_name="", context_docs=[])

    calls = [c.kwargs for c in client.chat.completions.create.call_args_list]
    assert len(calls) == 2
    review, improved = calls
    assert review["temperature"] == SCORING_TEMPERATURE
    assert review["seed"] == SCORING_SEED
    assert improved["temperature"] == 0.3
    assert "seed" not in improved


def test_resume_report_is_low_temperature_with_seed(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    client = MagicMock()
    client.chat.completions.create.return_value = MagicMock(
        choices=[MagicMock(message=MagicMock(content="レポート"), finish_reason="stop")]
    )
    with patch("services.resume_report.OpenAI", return_value=client):
        generate_resume_report(ReviewRequest(resume_text="履歴書", company_name="テスト株式会社"), [], "none", 10000)

    kwargs = client.chat.completions.create.call_args.kwargs
    assert kwargs["temperature"] == SCORING_TEMPERATURE
    assert kwargs["seed"] == SCORING_SEED


def test_resume_stream_is_low_temperature_with_seed(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    client = MagicMock()
    client.chat.completions.create.return_value = iter([])
    with patch("main._gather_context", return_value=([], "none")), \
         patch("main.RESUME_REVIEW_INPUT_CHAR_LIMIT", 10000), \
         patch("routers.resume.OpenAI", return_value=client):
        response = review_resume_stream(ReviewRequest(resume_text="履歴書", company_name="テスト株式会社"))

        async def _drain():
            async for _ in response.body_iterator:
                pass

        asyncio.run(_drain())

    kwargs = client.chat.completions.create.call_args.kwargs
    assert kwargs["temperature"] == SCORING_TEMPERATURE
    assert kwargs["seed"] == SCORING_SEED
