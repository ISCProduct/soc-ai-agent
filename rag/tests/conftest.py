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

# crewai をスタブ化（requirements.txt から除外済みのため、ローカルで未インストール）
_crewai_mock = types.ModuleType("crewai")
_crewai_mock.Agent = MagicMock()
_crewai_mock.Task = MagicMock()
_crewai_mock.Crew = MagicMock()
_crewai_mock.Process = MagicMock()
sys.modules.setdefault("crewai", _crewai_mock)

# chromadb をスタブ化する。
# chromadb 0.6.x の Settings クラスが pydantic.v1 互換層（Python 3.14 環境）で
# type inference エラーを起こすため、import 前にモックを注入して回避する。
# 実際の DB 接続はテストでは不要で、各テストが get_chroma_client 等を個別にモックする。
_chroma_client_mock = MagicMock()
_chromadb_mock = types.ModuleType("chromadb")
_chromadb_mock.PersistentClient = MagicMock(return_value=_chroma_client_mock)
sys.modules.setdefault("chromadb", _chromadb_mock)

# main を遅延インポートして TestClient を提供
import main


@pytest.fixture
def client():
    """TestClient を返すフィクスチャ"""
    return TestClient(main.app)
