"""/health と /healthz の OpenAI の状態（SOCAIAGENT-562）。"""
from unittest.mock import MagicMock

import pytest

from routers import health as health_router


@pytest.fixture(autouse=True)
def _reset_probe(monkeypatch):
    health_router._openai_probe.update({"at": 0.0, "result": None})
    monkeypatch.setattr(health_router, "ping_chroma", lambda: (True, "ok"))
    yield
    health_router._openai_probe.update({"at": 0.0, "result": None})


def test_health_is_degraded_without_openai_key(client, monkeypatch):
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    body = client.get("/health").json()
    assert body["status"] == "degraded"
    assert body["openai"] == {"configured": False}


def test_health_ok_with_key_and_never_returns_key(client, monkeypatch):
    monkeypatch.setenv("OPENAI_API_KEY", "sk-secret-value")
    res = client.get("/health")
    assert res.json()["status"] == "ok"
    assert res.json()["openai"] == {"configured": True}
    assert "sk-secret-value" not in res.text


def test_deep_health_probes_openai_and_caches(client, monkeypatch):
    import main

    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    openai_client = MagicMock()
    factory = MagicMock(return_value=openai_client)
    monkeypatch.setattr(main, "OpenAI", factory)

    for _ in range(3):
        body = client.get("/health?deep=1").json()
        assert body["status"] == "ok"
        assert body["openai"] == {"configured": True, "reachable": True}
    # 結果を覚えるので、何度呼んでも OpenAI へは1回だけ
    assert openai_client.models.list.call_count == 1


def test_deep_health_degraded_when_openai_unreachable(client, monkeypatch):
    import main

    monkeypatch.setenv("OPENAI_API_KEY", "sk-test")
    openai_client = MagicMock()
    openai_client.models.list.side_effect = RuntimeError("Incorrect API key provided: sk-tes***")
    monkeypatch.setattr(main, "OpenAI", MagicMock(return_value=openai_client))

    res = client.get("/health?deep=1")
    body = res.json()
    assert body["status"] == "degraded"
    assert body["openai"] == {"configured": True, "reachable": False, "detail": "RuntimeError"}
    # 例外の本文（鍵の断片を含みうる）は返さない
    assert "sk-tes" not in res.text


def test_healthz_stays_200_without_openai_key(client, monkeypatch):
    """ECS / ALB が見る /healthz は OpenAI の状態では 503 にしない（タスクの入れ替えで直らないため）。"""
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)
    res = client.get("/healthz")
    assert res.status_code == 200
    assert res.json()["openai"] == {"configured": False}


def test_healthz_503_when_vector_store_down(client, monkeypatch):
    monkeypatch.setattr(health_router, "ping_chroma", lambda: (False, "down"))
    assert client.get("/healthz").status_code == 503
