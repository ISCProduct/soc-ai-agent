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
from packaging.requirements import InvalidRequirement, Requirement
from packaging.utils import canonicalize_name
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


def _declared_requirements(filepath: str) -> tuple[list[Requirement], list[tuple[str, str]]]:
    """宣言ファイルを1行ずつ Requirement として解釈する。

    `==` / `>=` / `<` / 複合指定を一様に扱えるため、パッケージ名の列挙が不要になる。
    パースできない行は例外にせず errors として返す。モジュールインポート時に例外を投げると
    collection error になり、`--maxfail=1` の CI では依存整合と無関係なテストまで
    止まってしまうため（原因も専用テストの失敗メッセージで示す）。
    """
    reqs: list[Requirement] = []
    errors: list[tuple[str, str]] = []
    with open(filepath, encoding="utf-8") as f:
        for line in f:
            # 行頭または空白直後の # だけをコメントとして落とす。
            # 直接参照URL (`pkg @ git+https://...#egg=pkg`) のフラグメントを壊さないため。
            stripped = re.sub(r"(?:^|\s)#.*$", "", line).strip()
            if not stripped or stripped.startswith("-"):
                # 空行・コメント行と、pip オプション行（-r / --index-url 等）は対象外
                continue
            try:
                reqs.append(Requirement(stripped))
            except InvalidRequirement as exc:
                errors.append((stripped, str(exc)))
    return reqs, errors


_DECLARED, _DECLARED_ERRORS = _declared_requirements(_REQUIREMENTS_TXT)
_CONSTRAINTS_DECLARED, _CONSTRAINTS_ERRORS = _declared_requirements(_CONSTRAINTS_TXT)
# PEP 503 の正規化名で引く（requirements 側が langchain_core、constraints 側が
# langchain-core のような表記ゆれでも照合が外れないようにする）
_CONSTRAINED = {canonicalize_name(r.name): r for r in _CONSTRAINTS_DECLARED}


def _skip_if_marker_not_applicable(req: Requirement) -> None:
    """環境マーカーが成立しない宣言（例: python_version < "3.11"）は検証対象外。"""
    if req.marker is not None and not req.marker.evaluate():
        pytest.skip(f"{req.name} は現環境ではマーカー不成立: {req.marker}")


# pip は既定で prerelease をインストールしないため、検証も prereleases=False で揃える。
# packaging の SpecifierSet.contains() の既定はバージョンにより変わるため明示する（packaging は推移的依存）。
def _installed_version(req: Requirement) -> str:
    try:
        return importlib.metadata.version(req.name)
    except importlib.metadata.PackageNotFoundError:
        pytest.fail(
            f"{req.name}: requirements.txt に宣言されているがインストールされていない。"
            "`pip install -r requirements.txt -c constraints.txt` で環境を作り直すこと"
        )
        raise AssertionError("unreachable")  # pytest.fail は必ず送出する


class TestDeclaredPackagesMatchInstalled:
    """Issue #1159: requirements.txt の全宣言について、実インストール版が宣言を満たすことを検証する。

    #1067 で入れた乖離検知は langchain 系4件と chromadb だけが対象で、
    `==` ピン（fastapi/pydantic/pytest/tiktoken/uvicorn）は検証されていなかった。
    ここでは宣言を列挙せず requirements.txt を読んで判定するため、
    依存を追加しても自動的に検証対象になる。
    """

    def test_declaration_files_are_parsable(self):
        """requirements.txt / constraints.txt の全行が Requirement として解釈できる。"""
        assert not _DECLARED_ERRORS, f"requirements.txt にパースできない行がある: {_DECLARED_ERRORS}"
        assert not _CONSTRAINTS_ERRORS, f"constraints.txt にパースできない行がある: {_CONSTRAINTS_ERRORS}"

    def test_requirements_txt_is_not_empty(self):
        """宣言の読み取り自体が壊れていないことを確かめる（全件検証が空振りしないため）。"""
        assert _DECLARED, "requirements.txt の宣言が1件も読めていない"

    @pytest.mark.parametrize("req", _DECLARED, ids=[r.name for r in _DECLARED])
    def test_requirements_declaration_is_bounded(self, req: Requirement):
        """直接依存は上下から縛られている（`==` 系1件、または下限と上限の両方）。

        削除した旧テスト（chromadb / langchain 系のインストール済み検証）が
        `assert lower/upper is not None` で守っていたのは「上限・下限が宣言されていること」。
        非空 specifier の確認だけでは、`langchain-core>=1.0`（上限なし）のように
        片側を落としても素通りするため、langchain 1.x 固定(#894)や
        chromadb <0.7.0 固定(#489)の意図が黙って失われる。
        """
        if req.url:
            # 直接参照（`pkg @ git+https://...`）は specifier ではなく URL でピン留めする形式
            return
        ops = {spec.operator for spec in req.specifier}
        bounded = ops & {"==", "===", "~="} or ({">=", ">"} & ops and {"<", "<="} & ops)
        assert bounded, (
            f"{req.name}: 宣言 {req.specifier or '(指定なし)'} が上限・下限の両方を縛っていない"
        )

    @pytest.mark.parametrize("req", _DECLARED, ids=[r.name for r in _DECLARED])
    def test_installed_satisfies_requirements(self, req: Requirement):
        """インストール済み版が requirements.txt の宣言を満たしている。"""
        _skip_if_marker_not_applicable(req)
        installed = _installed_version(req)
        assert req.specifier.contains(installed, prereleases=False), (
            f"{req.name}: インストール済み {installed} が宣言 {req.specifier} を満たしていない。"
            "`pip install -r requirements.txt -c constraints.txt` で環境を作り直すこと (#1159)"
        )

    @pytest.mark.parametrize(
        "req", _CONSTRAINTS_DECLARED, ids=[r.name for r in _CONSTRAINTS_DECLARED]
    )
    def test_installed_satisfies_constraints(self, req: Requirement):
        """constraints.txt の全宣言について、インストール済み版が制約を満たしている。

        requirements.txt にある直接依存も、constraints.txt にしか無い宣言
        （numpy / litellm / httpx）も同じ扱いで検証する。後者は推移的依存の範囲を
        縛るための制約なので、守られていなければ #1067 と同種の乖離になる。
        未インストール（その推移的依存が入らない構成）は skip。

        constraints 側には上下限の両方を要求しない（`numpy<3.0` のように
        上限だけを縛る設計の宣言があるため）。
        """
        _skip_if_marker_not_applicable(req)
        assert str(req.specifier), f"{req.name}: constraints.txt にバージョン指定が無い"
        try:
            installed = importlib.metadata.version(req.name)
        except importlib.metadata.PackageNotFoundError:
            pytest.skip(f"{req.name} は未インストール（推移的依存が入っていない）")
        assert req.specifier.contains(installed, prereleases=False), (
            f"{req.name}: インストール済み {installed} が constraints.txt の "
            f"{req.specifier} を満たしていない (#1159)"
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
