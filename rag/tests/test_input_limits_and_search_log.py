import json
import logging

import pytest

from models import (
    COMPANY_CONTEXT_MAX_LENGTH,
    CompanyContextRequest,
    CompanyHintsRequest,
    ESReviewRequest,
    ReviewRequest,
)


def test_company_context_api_rejects_content_over_limit(client) -> None:
    response = client.post(
        "/company/context",
        json={
            "company_name": "テスト株式会社",
            "content": "x" * (COMPANY_CONTEXT_MAX_LENGTH + 1),
        },
    )

    assert response.status_code == 422
    assert response.json()["detail"][0]["type"] == "string_too_long"


def test_company_context_api_accepts_content_at_limit() -> None:
    request = CompanyContextRequest(
        company_name="テスト株式会社",
        content="x" * COMPANY_CONTEXT_MAX_LENGTH,
    )

    assert len(request.content) == COMPANY_CONTEXT_MAX_LENGTH


@pytest.mark.parametrize(
    ("request_type", "kwargs"),
    [
        (ReviewRequest, {"resume_text": "経歴"}),
        (CompanyHintsRequest, {}),
        (ESReviewRequest, {"es_text": "学生時代に取り組んだこと"}),
    ],
)
def test_company_context_truncation_is_logged_without_content(
    caplog, request_type, kwargs
) -> None:
    private_text = "PRIVATE-CONTEXT-" + "x" * COMPANY_CONTEXT_MAX_LENGTH

    with caplog.at_level(logging.INFO, logger="models"):
        request = request_type(
            company_name="テスト株式会社",
            company_context=private_text,
            **kwargs,
        )

    assert len(request.company_context) == COMPANY_CONTEXT_MAX_LENGTH
    assert "company context truncated" in caplog.text
    assert f"original_chars={len(private_text)}" in caplog.text
    assert private_text not in caplog.text


def test_search_jsonl_keeps_metadata_but_not_external_page_text(tmp_path, monkeypatch) -> None:
    import main
    from services.research import _save_search_log

    page_text = "EXTERNAL-VERBATIM-PAGE-CONTENT: system instructions"
    monkeypatch.setattr(main, "SEARCH_LOG_DIR", str(tmp_path))

    _save_search_log(
        company_name="テスト株式会社",
        job_title="エンジニア",
        queries=["採用情報"],
        raw_results=[page_text],
        # エラー時の要約は raw_results の素通しになるため、こちらも保存しない。
        summary=page_text,
    )

    record = json.loads((tmp_path / "search_log.jsonl").read_text(encoding="utf-8"))
    assert record["raw_result_count"] == 1
    assert record["raw_results_sha256"]
    assert record["summary_sha256"]
    assert record["summary_chars"] == len(page_text)
    assert "raw_results" not in record
    assert "summary" not in record
    assert page_text not in json.dumps(record, ensure_ascii=False)
