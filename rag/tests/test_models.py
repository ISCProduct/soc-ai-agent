"""rag/models.py の Pydantic バリデーションのテスト。"""
import pytest
from pydantic import ValidationError

from models import ESReviewRequest


class TestESReviewRequestEsTextMaxLength:
    def test_es_text_within_limit_is_accepted(self) -> None:
        req = ESReviewRequest(es_text="a" * 10000)
        assert len(req.es_text) == 10000

    def test_es_text_over_limit_raises_validation_error(self) -> None:
        with pytest.raises(ValidationError):
            ESReviewRequest(es_text="a" * 10001)
