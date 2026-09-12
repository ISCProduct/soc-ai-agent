"""リクエストID伝播のテスト(#1188)。

Backend は X-Request-ID を送る。RAG は X-Trace-ID を正としつつ
X-Request-ID も受け入れ、同じIDでログを突き合わせられること。
"""
import pytest
from fastapi.testclient import TestClient

import main


@pytest.fixture
def client():
    return TestClient(main.app)


@pytest.mark.parametrize(
    ("value", "expected"),
    [
        ("abc123", "abc123"),
        ("req-123_ABC", "req-123_ABC"),
        ("a" * 64, "a" * 64),
        ("a" * 65, None),
        ("", None),
        (None, None),
        ("req 123", None),
        ("req\n123", None),
        ("../../etc/passwd", None),
        ("<script>", None),
    ],
)
def test_safe_trace_id(value, expected):
    assert main._safe_trace_id(value) == expected


def test_trace_id_pattern_is_anchored():
    """前方一致だけで通ると不正な後続文字を見逃すため、両端アンカーを固定する。"""
    assert main._TRACE_ID_PATTERN.pattern.startswith("^")
    assert main._TRACE_ID_PATTERN.pattern.endswith("$")


class TestTraceIDMiddleware:
    def test_X_Trace_IDを優先して両ヘッダーへ返す(self, client):
        res = client.get("/health", headers={"X-Trace-ID": "trace-1", "X-Request-ID": "req-1"})
        assert res.headers["X-Trace-ID"] == "trace-1"
        assert res.headers["X-Request-ID"] == "trace-1"

    def test_X_Request_IDだけでも採用する(self, client):
        res = client.get("/health", headers={"X-Request-ID": "req-only"})
        assert res.headers["X-Trace-ID"] == "req-only"
        assert res.headers["X-Request-ID"] == "req-only"

    def test_不正な値は採番し直す(self, client):
        res = client.get("/health", headers={"X-Request-ID": "bad id"})
        generated = res.headers["X-Trace-ID"]
        assert generated != "bad id"
        # 採番後の値はそれ自体が安全な形式であること（uuid4 は英数字とハイフンのみ）
        assert main._safe_trace_id(generated) == generated

    def test_ヘッダーなしでも採番する(self, client):
        res = client.get("/health")
        assert res.headers["X-Trace-ID"]
        assert res.headers["X-Request-ID"] == res.headers["X-Trace-ID"]
