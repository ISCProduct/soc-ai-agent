"""rag/models.py の Pydantic バリデーションのテスト。"""
import pytest
from pydantic import ValidationError

from models import (
    ES_CHAR_LIMIT_MAX,
    ES_CHAR_LIMIT_MIN,
    ES_TEXT_MAX_LENGTH,
    ESReviewRequest,
)
from services.es_review import (
    _IMPROVED_TEXT_RATIO,
    _MAX_OUTPUT_TOKENS,
    _STAR_TEXT_CHARS,
    _estimate_max_tokens,
)


class TestESReviewRequestEsTextMaxLength:
    def test_es_text_max_length_is_pinned(self) -> None:
        """入力上限は 6,000字(#1564)。緩めると「必ず422になる帯」が復活する。"""
        assert ES_TEXT_MAX_LENGTH == 6000

    def test_es_text_within_limit_is_accepted(self) -> None:
        req = ESReviewRequest(es_text="a" * ES_TEXT_MAX_LENGTH)
        assert len(req.es_text) == ES_TEXT_MAX_LENGTH

    def test_es_text_over_limit_raises_validation_error(self) -> None:
        with pytest.raises(ValidationError):
            ESReviewRequest(es_text="a" * (ES_TEXT_MAX_LENGTH + 1))

    def test_max_length_output_fits_in_output_cap(self) -> None:
        """上限いっぱいの入力でも改善文+STARが出力天井に収まること(#1564)。

        ここが破れると「入力は通るが必ず 422」の帯が戻る。入力上限を上げるか
        STARの見積もりを増やすなら、_MAX_OUTPUT_TOKENS 側も一緒に見直すこと。
        """
        needed = _estimate_max_tokens(
            int(ES_TEXT_MAX_LENGTH * _IMPROVED_TEXT_RATIO) + _STAR_TEXT_CHARS
        )
        assert needed < _MAX_OUTPUT_TOKENS, f"見積もり {needed} が天井 {_MAX_OUTPUT_TOKENS} に達している"


class TestESReviewRequestCharLimit:
    """設問の文字数上限(#1523)。"""

    def test_char_limit_defaults_to_none(self) -> None:
        req = ESReviewRequest(es_text="あ")
        assert req.char_limit is None
        assert req.char_limit_mode == "within"

    def test_char_limit_range_is_pinned(self) -> None:
        assert (ES_CHAR_LIMIT_MIN, ES_CHAR_LIMIT_MAX) == (100, 2000)

    @pytest.mark.parametrize("value", [ES_CHAR_LIMIT_MIN, 400, 800, ES_CHAR_LIMIT_MAX])
    def test_char_limit_within_range_is_accepted(self, value: int) -> None:
        assert ESReviewRequest(es_text="あ", char_limit=value).char_limit == value

    @pytest.mark.parametrize("value", [0, 99, ES_CHAR_LIMIT_MAX + 1])
    def test_char_limit_out_of_range_raises(self, value: int) -> None:
        with pytest.raises(ValidationError):
            ESReviewRequest(es_text="あ", char_limit=value)

    def test_unknown_char_limit_mode_raises(self) -> None:
        with pytest.raises(ValidationError):
            ESReviewRequest(es_text="あ", char_limit=400, char_limit_mode="exact")

    def test_around_mode_is_accepted(self) -> None:
        req = ESReviewRequest(es_text="あ", char_limit=400, char_limit_mode="around")
        assert req.char_limit_mode == "around"
