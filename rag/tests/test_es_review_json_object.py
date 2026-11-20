"""ES添削のJSON出力検証(SOCAIAGENT-682)。dict以外・不正JSONは明確なエラーにする。"""
from unittest.mock import patch

import pytest

from services import es_review
from services.es_review import _call_json_with_retry


def _call(content: str):
    with patch.object(es_review, "_chat_json", return_value=(content, "stop")):
        return _call_json_with_retry(None, "m", "s", "u", 100, label="review")


def test_正常なdictはそのまま返る():
    assert _call('{"feedback": "ok"}') == {"feedback": "ok"}


def test_空文字は従来どおり空dict():
    assert _call("") == {}


@pytest.mark.parametrize("content", ["not json", "[1, 2]", "null", '"text"', "3"])
def test_JSONでないまたはdictでない出力はValueError(content):
    with pytest.raises(ValueError, match="JSON"):
        _call(content)
