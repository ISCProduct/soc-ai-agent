"""rag/models.py の Pydantic バリデーションのテスト。"""
import math

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
    _JP_TOKENS_PER_CHAR,
    _JSON_OVERHEAD_TOKENS,
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

    def test_max_length_output_fits_in_output_cap_for_plain_japanese(self) -> None:
        """上限いっぱいの入力でも、素の日本語なら改善文+STARが天井に収まること(#1564)。

        _estimate_max_tokens は内部で _MAX_OUTPUT_TOKENS に丸めるため、戻り値を
        比較しても「飽和したか」しか分からない。丸める前の必要量で比較する。

        検証できるのは _JP_TOKENS_PER_CHAR(0.85) と実測 0.81 の範囲、つまり
        漢字かな混在の入力までで、半角カナ主体の入力は覆っていない
        （次の test_halfwidth_kana_exceeds_the_cap_known_limitation 参照）。
        """
        chars = int(ES_TEXT_MAX_LENGTH * _IMPROVED_TEXT_RATIO) + _STAR_TEXT_CHARS
        # 丸める前の必要量（_estimate_max_tokens の中身と同じ式）
        needed = math.ceil(chars * _JP_TOKENS_PER_CHAR) + _JSON_OVERHEAD_TOKENS
        assert needed < _MAX_OUTPUT_TOKENS, f"見積もり {needed} が天井 {_MAX_OUTPUT_TOKENS} を超えている"
        # 見積もりが天井に飽和していない＝初回から再試行になる長さではない
        assert _estimate_max_tokens(chars) == needed
        # 実測レート(漢字かな混在 0.81)でも収まる
        assert math.ceil(chars * 0.81) + _JSON_OVERHEAD_TOKENS < _MAX_OUTPUT_TOKENS

    def test_halfwidth_kana_exceeds_the_cap_known_limitation(self) -> None:
        """半角カナ主体の入力は 6,000字だと天井に収まらない（既知の未解決 / #1564）。

        tiktoken(o200k_base) の実測で半角カナは約1.71 tok/char。0.85 の見積もりでは
        半分以下にしかならず、_MAX_OUTPUT_TOKENS まで引き上げても足りずに 422 になる。
        「解決済み」と書き換えられないよう、限界を数値で残しておく。
        係数の素材別見直しは #1564 に残している。
        """
        kana_rate = 1.71
        chars = int(ES_TEXT_MAX_LENGTH * _IMPROVED_TEXT_RATIO) + _STAR_TEXT_CHARS
        assert math.ceil(chars * kana_rate) + _JSON_OVERHEAD_TOKENS > _MAX_OUTPUT_TOKENS
        # 天井に収まる半角カナの限界はおよそ 3,300〜3,400字
        fits = (_MAX_OUTPUT_TOKENS - _JSON_OVERHEAD_TOKENS) / kana_rate
        assert 3300 < (fits - _STAR_TEXT_CHARS) / _IMPROVED_TEXT_RATIO < 3400


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
