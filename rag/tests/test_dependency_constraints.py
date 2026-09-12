"""
依存バージョン整合性テスト (Issue #489 / #1067 / #1159)

requirements.txt と constraints.txt のバージョン制約が一致していること、
インストール済みパッケージが制約範囲内にあることを検証する。

インストール済み版の検証は requirements.txt の全宣言を対象にする（#1159）。
パッケージ名を列挙する方式だと、列挙漏れのパッケージで #1067 と同じ乖離が
起きても CI が素通りするため。

実行方法:
    cd rag && pytest tests/test_dependency_constraints.py -v
"""
import importlib.metadata
import json
import os
import re

import pytest
from packaging.requirements import Requirement
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
                    "crewai は除外済みのため更新すること。"
                )


# ── LangChain が requirements に載っていること ───────────────────────────────

class TestLangchainInRequirements:
    def test_langchain_packages_listed_in_requirements(self):
        required = [
            "langchain",
            "langchain-core",
            "langchain-openai",
            "langchain-text-splitters",
        ]
        for pkg in required:
            spec = _parse_version_spec(_REQUIREMENTS_TXT, pkg)
            assert spec is not None, f"requirements.txt に {pkg} が無い。LangChain 導入を反映すること。"
            con = _parse_version_spec(_CONSTRAINTS_TXT, pkg)
            assert con is not None, f"constraints.txt に {pkg} が無い"

    def test_langchain_major_version_is_1x(self):
        """requirements と constraints の langchain 系下限が 1.x である。"""
        for pkg in ("langchain", "langchain-core", "langchain-openai", "langchain-text-splitters"):
            lower = _extract_lower_bound(_REQUIREMENTS_TXT, pkg)
            assert lower is not None, f"requirements.txt に {pkg} の下限が無い"
            assert Version(lower) >= Version("1.0"), (
                f"{pkg} の下限 {lower} が 1.x 未満。LangChain 1.x 移行 (#894) を反映すること。"
            )
            req_upper = _extract_upper_bound(_REQUIREMENTS_TXT, pkg)
            con_upper = _extract_upper_bound(_CONSTRAINTS_TXT, pkg)
            assert req_upper == con_upper, (
                f"{pkg}: requirements (<{req_upper}) と constraints (<{con_upper}) の上限不一致"
            )

    def test_langchain_community_not_listed(self):
        """未使用の langchain-community は requirements から削除済み。"""
        assert _parse_version_spec(_REQUIREMENTS_TXT, "langchain-community") is None
        assert _parse_version_spec(_CONSTRAINTS_TXT, "langchain-community") is None


def _declared_requirements(filepath: str) -> list[Requirement]:
    """宣言ファイルを1行ずつ Requirement として解釈する。

    `==` / `>=` / `<` / 複合指定を一様に扱えるため、パッケージ名の列挙が不要になる。
    """
    reqs: list[Requirement] = []
    with open(filepath, encoding="utf-8") as f:
        for line in f:
            stripped = line.split("#")[0].strip()
            if not stripped or stripped.startswith("-"):
                # 空行・コメント行と、pip オプション行（-r / --index-url 等）は対象外
                continue
            reqs.append(Requirement(stripped))
    return reqs


_DECLARED = _declared_requirements(_REQUIREMENTS_TXT)
_CONSTRAINED = {r.name.lower(): r for r in _declared_requirements(_CONSTRAINTS_TXT)}


class TestDeclaredPackagesMatchInstalled:
    """Issue #1159: requirements.txt の全宣言について、実インストール版が宣言を満たすことを検証する。

    #1067 で入れた乖離検知は langchain 系4件と chromadb だけが対象で、
    `==` ピン（fastapi/pydantic/pytest/tiktoken/uvicorn）は検証されていなかった。
    ここでは宣言を列挙せず requirements.txt を読んで判定するため、
    依存を追加しても自動的に検証対象になる。
    """

    def test_requirements_txt_is_not_empty(self):
        """宣言の読み取り自体が壊れていないことを確かめる（全件検証が空振りしないため）。"""
        assert len(_DECLARED) >= 5, f"requirements.txt の宣言が読めていない: {_DECLARED}"

    @pytest.mark.parametrize("req", _DECLARED, ids=[r.name for r in _DECLARED])
    def test_installed_satisfies_requirements(self, req: Requirement):
        """インストール済み版が requirements.txt の宣言を満たしている。"""
        try:
            installed = importlib.metadata.version(req.name)
        except importlib.metadata.PackageNotFoundError:
            pytest.fail(
                f"{req.name}: requirements.txt に宣言されているがインストールされていない。"
                "`pip install -r requirements.txt -c constraints.txt` で環境を作り直すこと"
            )
        assert req.specifier.contains(installed, prereleases=True), (
            f"{req.name}: インストール済み {installed} が宣言 {req.specifier} を満たしていない。"
            "`pip install -r requirements.txt -c constraints.txt` で環境を作り直すこと (#1159)"
        )

    @pytest.mark.parametrize("req", _DECLARED, ids=[r.name for r in _DECLARED])
    def test_installed_satisfies_constraints(self, req: Requirement):
        """constraints.txt にも宣言がある場合、インストール済み版がそちらも満たしている。"""
        con = _CONSTRAINED.get(req.name.lower())
        if con is None:
            pytest.skip(f"{req.name} は constraints.txt に宣言が無い")
        installed = importlib.metadata.version(req.name)
        assert con.specifier.contains(installed, prereleases=True), (
            f"{req.name}: インストール済み {installed} が constraints.txt の "
            f"{con.specifier} を満たしていない (#1159)"
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
