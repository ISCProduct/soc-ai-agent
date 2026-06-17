"""
依存バージョン整合性テスト (Issue #489)

requirements.txt と constraints.txt のバージョン制約が一致していること、
インストール済みパッケージが制約範囲内にあることを検証する。

実行方法:
    cd rag && pytest tests/test_dependency_constraints.py -v
"""
import importlib.metadata
import json
import os
import re

import pytest
from packaging.version import Version

_RAG_DIR = os.path.join(os.path.dirname(__file__), "..")
_REPO_ROOT = os.path.join(_RAG_DIR, "..")
_REQUIREMENTS_TXT = os.path.join(_RAG_DIR, "requirements.txt")
_CONSTRAINTS_TXT = os.path.join(_RAG_DIR, "constraints.txt")
_FRONTEND_PACKAGE_JSON = os.path.join(_REPO_ROOT, "frontend", "package.json")


def _parse_version_spec(filepath: str, package: str) -> str | None:
    """ファイルから package の行を返す（コメント・空行除外）。"""
    with open(filepath, encoding="utf-8") as f:
        for line in f:
            stripped = line.strip()
            if not stripped or stripped.startswith("#"):
                continue
            name = re.split(r"[><=!;\[,\s]", stripped)[0].strip()
            if name.lower() == package.lower():
                return stripped
    return None


def _extract_upper_bound(filepath: str, package: str) -> str | None:
    """ファイルから package の <X.Y.Z 上限を抽出する。"""
    spec = _parse_version_spec(filepath, package)
    if spec is None:
        return None
    match = re.search(r"<([0-9][0-9.]*)", spec)
    return match.group(1) if match else None


def _extract_lower_bound(filepath: str, package: str) -> str | None:
    """ファイルから package の >=X.Y.Z 下限を抽出する。"""
    spec = _parse_version_spec(filepath, package)
    if spec is None:
        return None
    match = re.search(r">=([0-9][0-9.]*)", spec)
    return match.group(1) if match else None


# ── chromadb バージョン整合性 ───────────────────────────────────────────────

class TestChromadbVersionConstraint:
    """Issue #489: requirements.txt と constraints.txt の chromadb 上限を統一した修正を検証。"""

    def test_constraints_txt_upper_bound_is_070(self):
        """constraints.txt の chromadb 上限が <0.7.0 に設定されている。"""
        upper = _extract_upper_bound(_CONSTRAINTS_TXT, "chromadb")
        assert upper is not None, "constraints.txt に chromadb の <上限 が設定されていない"
        assert Version(upper) <= Version("0.7.0"), (
            f"constraints.txt の chromadb 上限 <{upper} が <0.7.0 より緩い。"
            "requirements.txt と合わせて <0.7.0 に固定する必要がある。"
        )

    def test_requirements_txt_upper_bound_matches_constraints(self):
        """requirements.txt と constraints.txt の chromadb 上限が一致している。"""
        req_upper = _extract_upper_bound(_REQUIREMENTS_TXT, "chromadb")
        con_upper = _extract_upper_bound(_CONSTRAINTS_TXT, "chromadb")
        assert req_upper is not None, "requirements.txt に chromadb の上限が設定されていない"
        assert con_upper is not None, "constraints.txt に chromadb の上限が設定されていない"
        assert req_upper == con_upper, (
            f"requirements.txt (<{req_upper}) と constraints.txt (<{con_upper}) の "
            "chromadb 上限が不一致。両ファイルを揃えること。"
        )

    def test_installed_chromadb_within_lower_bound(self):
        """インストール済みの chromadb が下限 >=0.6.3 を満たしている。"""
        version = importlib.metadata.version("chromadb")
        lower = _extract_lower_bound(_REQUIREMENTS_TXT, "chromadb")
        assert lower is not None
        assert Version(version) >= Version(lower), (
            f"インストール済み chromadb {version} が下限 {lower} 未満"
        )

    def test_installed_chromadb_within_upper_bound(self):
        """インストール済みの chromadb が上限 <0.7.0 を満たしている。"""
        version = importlib.metadata.version("chromadb")
        upper = _extract_upper_bound(_CONSTRAINTS_TXT, "chromadb")
        assert upper is not None
        assert Version(version) < Version(upper), (
            f"インストール済み chromadb {version} が上限 <{upper} を超えている"
        )


# ── constraints.txt コメント整合性 ─────────────────────────────────────────

class TestConstraintsCommentConsistency:
    """Issue #489: constraints.txt のコメントと実バージョン制約の乖離を是正した修正を検証。"""

    def _read_constraints(self) -> str:
        with open(_CONSTRAINTS_TXT, encoding="utf-8") as f:
            return f.read()

    def test_tiktoken_comment_not_outdated_07x(self):
        """tiktoken コメントに古い '0.7.x' という記述が残っていない。"""
        content = self._read_constraints()
        # tiktoken セクションのコメント行を抽出
        in_tiktoken_section = False
        for line in content.splitlines():
            if "tiktoken" in line and not line.strip().startswith("#"):
                in_tiktoken_section = False
            if line.strip().startswith("tiktoken"):
                in_tiktoken_section = True
            if in_tiktoken_section and line.strip().startswith("#"):
                assert "0.7.x" not in line, (
                    f"tiktoken コメントに古い '0.7.x' が残っている: {line.strip()!r}。"
                    "実際の制約 (0.13.x) に合わせて更新すること。"
                )

    def test_httpx_comment_not_outdated_027x(self):
        """httpx コメントに古い '0.27.x' という記述が残っていない。"""
        content = self._read_constraints()
        in_httpx_section = False
        for line in content.splitlines():
            if line.strip().startswith("httpx"):
                in_httpx_section = True
            if in_httpx_section and line.strip().startswith("#"):
                assert "0.27.x" not in line, (
                    f"httpx コメントに古い '0.27.x' が残っている: {line.strip()!r}。"
                    "実際の制約 (0.28.x) に合わせて更新すること。"
                )

    def test_langchain_comment_does_not_reference_removed_crewai(self):
        """langchain コメントに除外済みの crewai を前提とした記述が残っていない。"""
        content = self._read_constraints()
        # langchain の制約行より前のコメントを確認
        found_langchain_constraint = False
        for line in content.splitlines():
            stripped = line.strip()
            if stripped.startswith("langchain>=") or stripped.startswith("langchain>="):
                found_langchain_constraint = True
                break
            if stripped.startswith("#") and "crewai 0.64.0 は langchain 0.2.x を要求する" in stripped:
                pytest.fail(
                    "langchain コメントに削除済みの crewai 0.64.0 前提記述が残っている。"
                    "crewai は requirements.txt から除外済みのため更新すること。"
                )

    def test_litellm_comment_does_not_say_crewai_dependency(self):
        """litellm コメントが 'crewai の推移的依存' という旧記述を含まない。"""
        content = self._read_constraints()
        in_litellm_section = False
        for line in content.splitlines():
            if line.strip().startswith("litellm"):
                in_litellm_section = True
            if in_litellm_section and line.strip().startswith("#"):
                assert "crewai の推移的依存" not in line, (
                    f"litellm コメントに古い 'crewai の推移的依存' が残っている: {line.strip()!r}。"
                    "crewai は除外済みのため 'langchain-community の推移的依存' に更新すること。"
                )


# ── Frontend: package.json バージョン整合性 ────────────────────────────────

class TestFrontendReactVersionConsistency:
    """Issue #489: @types/react と react のメジャー版を一致させた修正を検証。"""

    def _load_package_json(self) -> dict:
        with open(_FRONTEND_PACKAGE_JSON, encoding="utf-8") as f:
            return json.load(f)

    def _extract_major(self, version_spec: str) -> int:
        """'^19.2.0' や '>=19.0.0' からメジャーバージョン番号を返す。"""
        match = re.search(r"(\d+)\.", version_spec)
        assert match, f"バージョン文字列からメジャー番号を抽出できない: {version_spec!r}"
        return int(match.group(1))

    def test_types_react_major_matches_react_major(self):
        """@types/react のメジャー版が react のメジャー版と一致している。"""
        pkg = self._load_package_json()
        react_spec = pkg.get("dependencies", {}).get("react", "")
        types_spec = pkg.get("devDependencies", {}).get("@types/react", "")

        assert react_spec, "package.json に react が見つからない"
        assert types_spec, "package.json に @types/react が見つからない"

        react_major = self._extract_major(react_spec)
        types_major = self._extract_major(types_spec)

        assert react_major == types_major, (
            f"react ({react_spec}) と @types/react ({types_spec}) のメジャー版が不一致。"
            f"react={react_major} に対して @types/react={types_major}。"
        )

    def test_types_react_dom_major_matches_react_dom_major(self):
        """@types/react-dom のメジャー版が react-dom のメジャー版と一致している。"""
        pkg = self._load_package_json()
        react_dom_spec = pkg.get("dependencies", {}).get("react-dom", "")
        types_spec = pkg.get("devDependencies", {}).get("@types/react-dom", "")

        assert react_dom_spec, "package.json に react-dom が見つからない"
        assert types_spec, "package.json に @types/react-dom が見つからない"

        react_dom_major = self._extract_major(react_dom_spec)
        types_major = self._extract_major(types_spec)

        assert react_dom_major == types_major, (
            f"react-dom ({react_dom_spec}) と @types/react-dom ({types_spec}) のメジャー版が不一致。"
            f"react-dom={react_dom_major} に対して @types/react-dom={types_major}。"
        )


# ── CLAUDE.md アーキテクチャ記述の整合性 ──────────────────────────────────

class TestClaudeMdConsistency:
    """Issue #489: CLAUDE.md のアーキテクチャ説明が実装と一致していることを検証。"""

    def _read_claude_md(self) -> str:
        path = os.path.join(_REPO_ROOT, "CLAUDE.md")
        with open(path, encoding="utf-8") as f:
            return f.read()

    def test_architecture_does_not_mention_crewai(self):
        """CLAUDE.md のアーキテクチャ説明に除外済みの CrewAI が記載されていない。"""
        content = self._read_claude_md()
        for line in content.splitlines():
            if "構造" in line and "CrewAI" in line:
                pytest.fail(
                    f"CLAUDE.md のアーキテクチャ行に除外済みの CrewAI が含まれている: {line.strip()!r}。"
                    "requirements.txt から crewai を除外した事実と整合させること。"
                )
