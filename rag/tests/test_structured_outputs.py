"""LLM の JSON 出力のスキーマ強制（services/output_schemas.py、SOCAIAGENT-563）。"""
import json
from unittest.mock import MagicMock

import pytest

from services import output_schemas
from tests.test_es_review_char_limit import _client, _run


def _assert_strict(schema, path="$"):
    """strict の json_schema の制約: object はすべてのキーを required にし、追加のキーを禁じる。"""
    if schema.get("type") == "object" or "properties" in schema:
        assert schema.get("additionalProperties") is False, path
        assert set(schema["required"]) == set(schema["properties"]), path
        for key, sub in schema["properties"].items():
            _assert_strict(sub, f"{path}.{key}")
    if "items" in schema:
        _assert_strict(schema["items"], f"{path}[]")


@pytest.mark.parametrize("schema", [
    output_schemas.ES_REVIEW_SCHEMA, output_schemas.ES_IMPROVED_SCHEMA, output_schemas.HINTS_PARSE_SCHEMA,
])
def test_schemas_satisfy_strict_rules(schema):
    _assert_strict(schema)


def _formats(client):
    return [call.kwargs["response_format"] for call in client.chat.completions.create.call_args_list]


def test_es_review_uses_strict_schemas(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.delenv("RAG_STRUCTURED_OUTPUTS", raising=False)
    client = _client([100])

    _run(client)

    formats = _formats(client)
    assert [f["type"] for f in formats] == ["json_schema", "json_schema"]
    assert [f["json_schema"]["name"] for f in formats] == ["es_review", "es_improved_text"]
    assert all(f["json_schema"]["strict"] is True for f in formats)


def test_can_switch_back_to_json_mode(monkeypatch):
    """RAG_STRUCTURED_OUTPUTS=0 で JSON モードに戻せる（切り戻し用）。"""
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.setenv("RAG_STRUCTURED_OUTPUTS", "0")
    client = _client([100])

    _run(client)

    assert _formats(client) == [{"type": "json_object"}, {"type": "json_object"}]


def test_falls_back_to_json_mode_when_schema_unsupported(monkeypatch):
    """スキーマに対応しないモデル・互換サーバのエラーなら、JSON モードで呼び直して結果を返す。"""
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.delenv("RAG_STRUCTURED_OUTPUTS", raising=False)
    base = _client([100])
    real_create = base.chat.completions.create.side_effect

    def create(**kwargs):
        if kwargs["response_format"]["type"] == "json_schema":
            raise RuntimeError("Invalid parameter: 'response_format' of type 'json_schema' is not supported with this model.")
        return real_create(**kwargs)

    client = MagicMock()
    client.chat.completions.create.side_effect = create
    result = _run(client)

    assert result.improved_text
    assert [f["type"] for f in _formats(client)] == ["json_schema", "json_object", "json_schema", "json_object"]


def test_other_errors_are_not_retried_in_json_mode(monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.delenv("RAG_STRUCTURED_OUTPUTS", raising=False)
    client = MagicMock()
    client.chat.completions.create.side_effect = RuntimeError("rate limited")
    with pytest.raises(Exception):
        _run(client)
    assert all(f["type"] == "json_schema" for f in _formats(client))


def test_hints_parse_uses_strict_schema(monkeypatch):
    import main
    from services.hints import _parse_hints_from_text

    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    monkeypatch.delenv("RAG_STRUCTURED_OUTPUTS", raising=False)
    client = MagicMock()
    client.chat.completions.create.return_value.choices = [
        MagicMock(message=MagicMock(content=json.dumps({"style_tags": ["a"], "top_questions": ["q"]})))
    ]
    monkeypatch.setattr(main, "OpenAI", lambda **_kw: client)

    result = _parse_hints_from_text("テスト株式会社", "エンジニア", "リサーチ結果")

    fmt = client.chat.completions.create.call_args.kwargs["response_format"]
    assert fmt["type"] == "json_schema" and fmt["json_schema"]["name"] == "hints_parse"
    assert result.style_tags == ["a"]
