"""
内部サービス認証ミドルウェアのテスト (#615)
"""
import pytest
from fastapi.testclient import TestClient

import main


@pytest.fixture
def raw_client():
    """認証ヘッダーなしの TestClient"""
    return TestClient(main.app)


class TestInternalAuth:
    def test_トークンなしは401(self, raw_client):
        res = raw_client.get("/vector/status")
        assert res.status_code == 401

    def test_不正なトークンは401(self, raw_client):
        res = raw_client.get(
            "/vector/status", headers={"X-Internal-Token": "wrong-token"}
        )
        assert res.status_code == 401

    def test_ヘルスチェックは認証不要(self, raw_client):
        for path in ("/health", "/healthz"):
            res = raw_client.get(path)
            assert res.status_code == 200, path

    def test_トークン未設定時はフェイルクローズで503(self, raw_client, monkeypatch):
        monkeypatch.delenv("RAG_INTERNAL_TOKEN", raising=False)
        res = raw_client.get(
            "/vector/status", headers={"X-Internal-Token": "test-internal-token"}
        )
        assert res.status_code == 503

    def test_トークン未設定でもヘルスチェックは通る(self, raw_client, monkeypatch):
        monkeypatch.delenv("RAG_INTERNAL_TOKEN", raising=False)
        res = raw_client.get("/healthz")
        assert res.status_code == 200

    def test_正しいトークンは通過する(self, client):
        # conftest の client フィクスチャは正しいトークンを付与している
        res = client.get("/healthz")
        assert res.status_code == 200
