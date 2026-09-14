"""検索コストの調整ノブのテスト（#1124）。

OpenAI の web_search は検索結果が固定トークンとして課金されるため、
1コールの重さ（search_context_size）とコール回数（クエリ数）が
そのままコストに効く。既定値と env での上書きを固定する。
"""
import os

import pytest

from services.research import max_search_queries, web_search_context_size


@pytest.fixture(autouse=True)
def _clear_env(monkeypatch):
    monkeypatch.delenv("OPENAI_WEB_SEARCH_CONTEXT_SIZE", raising=False)
    monkeypatch.delenv("OPENAI_WEB_SEARCH_MAX_QUERIES", raising=False)


class TestWebSearchContextSize:
    def test_既定はmedium(self):
        # 以前は "high" 固定だった。high は最も高コストな設定
        assert web_search_context_size() == "medium"

    @pytest.mark.parametrize("value", ["low", "medium", "high"])
    def test_有効な値はそのまま使う(self, monkeypatch, value):
        monkeypatch.setenv("OPENAI_WEB_SEARCH_CONTEXT_SIZE", value)
        assert web_search_context_size() == value

    def test_大文字空白は正規化する(self, monkeypatch):
        monkeypatch.setenv("OPENAI_WEB_SEARCH_CONTEXT_SIZE", "  HIGH  ")
        assert web_search_context_size() == "high"

    @pytest.mark.parametrize("value", ["", "huge", "0", "none"])
    def test_不正な値は既定に倒す(self, monkeypatch, value):
        # 設定ミスで起動やリクエストを止めない
        monkeypatch.setenv("OPENAI_WEB_SEARCH_CONTEXT_SIZE", value)
        assert web_search_context_size() == "medium"


class TestMaxSearchQueries:
    def test_既定は4(self):
        # 以前は 5 固定。1クエリ = 1コール = 固定トークン課金
        assert max_search_queries() == 4

    @pytest.mark.parametrize(("value", "expected"), [("1", 1), ("3", 3), ("10", 10)])
    def test_範囲内の値は使う(self, monkeypatch, value, expected):
        monkeypatch.setenv("OPENAI_WEB_SEARCH_MAX_QUERIES", value)
        assert max_search_queries() == expected

    @pytest.mark.parametrize("value", ["0", "11", "-1", "abc", "", "2.5"])
    def test_範囲外や不正な値は既定に倒す(self, monkeypatch, value):
        monkeypatch.setenv("OPENAI_WEB_SEARCH_MAX_QUERIES", value)
        assert max_search_queries() == 4


class TestDefaultModels:
    """既定モデルが gpt-4o-mini であることを固定する。

    gpt-4o は入力単価が 16.7 倍。要約・パース用途では mini で足りる。
    """

    @pytest.mark.parametrize(
        ("env_key", "module_path"),
        [
            ("OPENAI_CHAT_MODEL", "services.research"),
            ("OPENAI_CHAT_MODEL", "services.hints"),
            ("OPENAI_HINTS_PARSE_MODEL", "services.hints"),
        ],
    )
    def test_既定はminiでenvで上書きできる(self, monkeypatch, env_key, module_path):
        monkeypatch.delenv(env_key, raising=False)
        assert os.getenv(env_key, "gpt-4o-mini") == "gpt-4o-mini"
        monkeypatch.setenv(env_key, "gpt-4o")
        assert os.getenv(env_key, "gpt-4o-mini") == "gpt-4o"

    def test_ソース上の既定値にgpt_4oが残っていない(self):
        """既定値の書き換え漏れを検出する。"""
        import pathlib

        for name in ("research.py", "hints.py"):
            src = pathlib.Path(__file__).parent.parent / "services" / name
            text = src.read_text(encoding="utf-8")
            assert 'OPENAI_CHAT_MODEL", "gpt-4o")' not in text, name
            assert 'OPENAI_HINTS_PARSE_MODEL", "gpt-4o")' not in text, name
