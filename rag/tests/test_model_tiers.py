"""工程ごとのモデルの二層化（#1526）。"""
import pytest

from services import model_tiers

_ENVS = [
    "AI_MODEL_PREMIUM",
    "AI_MODEL_LIGHT",
    "OPENAI_CHAT_MODEL",
    "OPENAI_ES_REVIEW_MODEL",
    "OPENAI_REVIEW_MODEL",
    "OPENAI_HINTS_PARSE_MODEL",
    "OPENAI_ES_IMPROVE_MODEL",
]


@pytest.fixture(autouse=True)
def _clear_env(monkeypatch):
    for name in _ENVS:
        monkeypatch.delenv(name, raising=False)


def test_未設定なら今までの既定():
    # 導入だけでコストが変わらないこと
    assert model_tiers.es_review_model() == "gpt-4o"
    assert model_tiers.resume_review_model() == "gpt-4o-mini"
    assert model_tiers.summary_model() == "gpt-4o-mini"
    assert model_tiers.hints_parse_model() == "gpt-4o-mini"


def test_既存のOPENAI_CHAT_MODELは今まで通り効く(monkeypatch):
    monkeypatch.setenv("OPENAI_CHAT_MODEL", "gpt-4.1-mini")
    assert model_tiers.es_review_model() == "gpt-4.1-mini"
    assert model_tiers.summary_model() == "gpt-4.1-mini"


def test_層の設定で切り替わる(monkeypatch):
    monkeypatch.setenv("OPENAI_CHAT_MODEL", "gpt-4o-mini")
    monkeypatch.setenv("AI_MODEL_PREMIUM", "gpt-4o")
    monkeypatch.setenv("AI_MODEL_LIGHT", "gpt-4.1-nano")
    # 学生に見える判断は上位、内部の定型処理は軽量
    assert model_tiers.es_review_model() == "gpt-4o"
    assert model_tiers.resume_review_model() == "gpt-4o"
    assert model_tiers.summary_model() == "gpt-4.1-nano"
    assert model_tiers.hints_parse_model() == "gpt-4.1-nano"


def test_工程ごとの設定が層より優先(monkeypatch):
    monkeypatch.setenv("AI_MODEL_PREMIUM", "gpt-4o")
    monkeypatch.setenv("AI_MODEL_LIGHT", "gpt-4.1-nano")
    monkeypatch.setenv("OPENAI_ES_REVIEW_MODEL", "gpt-4.1")
    monkeypatch.setenv("OPENAI_REVIEW_MODEL", "gpt-4o-mini")
    monkeypatch.setenv("OPENAI_HINTS_PARSE_MODEL", "gpt-4o-mini")
    assert model_tiers.es_review_model() == "gpt-4.1"
    assert model_tiers.resume_review_model() == "gpt-4o-mini"
    assert model_tiers.hints_parse_model() == "gpt-4o-mini"


def test_healthはES添削で使うモデルを返す(monkeypatch, client):
    # aibench（ES添削）は /health の chat_model を結果に残す。実際のモデルとずれると比較に使えない
    monkeypatch.setenv("AI_MODEL_PREMIUM", "gpt-4o")
    monkeypatch.setenv("OPENAI_CHAT_MODEL", "gpt-4o-mini")
    body = client.get("/health").json()
    assert body.get("chat_model") == model_tiers.es_review_model() == "gpt-4o"



def test_ES添削の改善文は軽量の層(monkeypatch):
    """改善文の生成は評価より軽い処理なので軽量の層にする（SOCAIAGENT-559）。評価は上位のまま。"""
    assert model_tiers.es_improve_model() == "gpt-4o-mini"
    assert model_tiers.es_review_model() == "gpt-4o"
    # OPENAI_CHAT_MODEL（評価の既定を変える既存の変数）には引きずられない
    monkeypatch.setenv("OPENAI_CHAT_MODEL", "gpt-4.1")
    assert model_tiers.es_improve_model() == "gpt-4o-mini"
    monkeypatch.setenv("AI_MODEL_LIGHT", "gpt-4.1-nano")
    assert model_tiers.es_improve_model() == "gpt-4.1-nano"
    # 工程ごとの設定が最優先（元の上位モデルに戻すとき）
    monkeypatch.setenv("OPENAI_ES_IMPROVE_MODEL", "gpt-4o")
    assert model_tiers.es_improve_model() == "gpt-4o"
