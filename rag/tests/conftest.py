"""
共通 pytest フィクスチャ（RAG テスト）
"""
import os
import sys
import types
from unittest.mock import MagicMock
import pytest
from fastapi.testclient import TestClient

# rag/ をパスに追加
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

# テストで必要な環境変数を設定（テスト用ダミーキー）
os.environ.setdefault("OPENAI_API_KEY", "sk-test")

# crewai をスタブ化（ローカルで実際に接続しない）
_crewai_mock = types.ModuleType("crewai")
_crewai_mock.Agent = MagicMock()
_crewai_mock.Task = MagicMock()
_crewai_mock.Crew = MagicMock()
_crewai_mock.Process = MagicMock()
sys.modules.setdefault("crewai", _crewai_mock)

# main を遅延インポートして TestClient を提供
import main

@pytest.fixture
def client():
    """TestClient を返すフィクスチャ"""
    return TestClient(main.app)
