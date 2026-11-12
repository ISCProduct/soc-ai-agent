"""固定の評価セットの読み込みと検証（#1773）。

評価セットは匿名の合成データだけで作る。実在の学生・企業のデータは入れない。
個人情報らしい文字列（メールアドレス・電話番号・郵便番号）が入っていたら読み込みで落とす。
"""
from __future__ import annotations

import json
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List

DATA_DIR = Path(__file__).parent / "data"

_PII_PATTERNS = {
    "メールアドレス": re.compile(r"[\w.+-]+@[\w-]+\.[\w.-]+"),
    "電話番号": re.compile(r"0\d{1,4}-\d{1,4}-\d{3,4}|\b0\d{9,10}\b"),
    "郵便番号": re.compile(r"〒?\d{3}-\d{4}\b"),
}


class DatasetError(ValueError):
    """評価セットの形式が正しくない。"""


@dataclass(frozen=True)
class Document:
    id: str
    text: str
    company: str = ""
    role: str = ""
    # company_original はサニタイズ前の企業名。本番と同じくキャッシュキーの企業ハッシュに使う（#938）
    company_original: str = ""
    # stale は取得時刻を鮮度の期限より前にして保存する文書。検索結果に出てはいけない
    stale: bool = False

    @property
    def company_identity(self) -> tuple:
        return (self.company, self.company_original)


@dataclass(frozen=True)
class Query:
    id: str
    query: str
    relevant: List[str]
    company: str = ""
    role: str = ""
    company_original: str = ""

    @property
    def company_identity(self) -> tuple:
        return (self.company, self.company_original)


@dataclass
class Dataset:
    name: str
    documents: List[Document]
    queries: List[Query]
    by_id: Dict[str, Document] = field(default_factory=dict)


def _check_pii(where: str, text: str) -> None:
    for label, pattern in _PII_PATTERNS.items():
        if pattern.search(text):
            raise DatasetError(f"{where}: {label}らしき文字列が含まれている。評価セットは匿名の合成データにすること")


def _check_pii_everywhere(where: str, value) -> None:
    """本文だけでなく、ID・企業名・説明など全ての文字列を検査する。"""
    if isinstance(value, str):
        _check_pii(where, value)
    elif isinstance(value, dict):
        for key, item in value.items():
            _check_pii_everywhere(f"{where}.{key}", key)
            _check_pii_everywhere(f"{where}.{key}", item)
    elif isinstance(value, list):
        for index, item in enumerate(value):
            _check_pii_everywhere(f"{where}[{index}]", item)


def load_dataset(path: Path) -> Dataset:
    """評価セットを読み込み、ID の重複・正解の参照先・本文の重複・個人情報を検証する。"""
    raw = json.loads(Path(path).read_text(encoding="utf-8"))
    name = raw.get("name") or Path(path).stem
    _check_pii_everywhere(name, raw)
    documents: List[Document] = []
    for item in raw.get("documents", []):
        doc = Document(
            id=str(item["id"]),
            text=str(item["text"]).strip(),
            company=str(item.get("company", "")),
            role=str(item.get("role", "")),
            company_original=str(item.get("company_original", "")),
            stale=bool(item.get("stale", False)),
        )
        if not doc.id or not doc.text:
            raise DatasetError(f"{name}: id と text は必須")
        _check_pii(f"{name}/{doc.id}", doc.text)
        documents.append(doc)
    queries: List[Query] = []
    for item in raw.get("queries", []):
        query = Query(
            id=str(item["id"]),
            query=str(item["query"]).strip(),
            relevant=[str(r) for r in item.get("relevant", [])],
            company=str(item.get("company", "")),
            role=str(item.get("role", "")),
            company_original=str(item.get("company_original", "")),
        )
        if not query.query or not query.relevant:
            raise DatasetError(f"{name}/{query.id}: query と relevant は必須")
        _check_pii(f"{name}/{query.id}", query.query)
        queries.append(query)

    by_id: Dict[str, Document] = {}
    texts = set()
    for doc in documents:
        if doc.id in by_id:
            raise DatasetError(f"{name}: 文書 ID {doc.id} が重複している")
        # 企業リサーチの結果は本文で返るので、本文から ID を引けるよう重複を禁止する
        if doc.text in texts:
            raise DatasetError(f"{name}/{doc.id}: 本文が他の文書と重複している")
        by_id[doc.id] = doc
        texts.add(doc.text)
    query_ids = set()
    for query in queries:
        if query.id in query_ids:
            raise DatasetError(f"{name}: クエリ ID {query.id} が重複している")
        query_ids.add(query.id)
        missing = [r for r in query.relevant if r not in by_id]
        if missing:
            raise DatasetError(f"{name}/{query.id}: 正解 {missing} が文書に無い")
        stale = [r for r in query.relevant if by_id[r].stale]
        if stale:
            raise DatasetError(f"{name}/{query.id}: 期限切れの文書 {stale} を正解にしている（返ってはいけない文書）")
    if not documents or not queries:
        raise DatasetError(f"{name}: 文書とクエリは1件以上必要")
    return Dataset(name=name, documents=documents, queries=queries, by_id=by_id)


def default_dataset_path(name: str) -> Path:
    return DATA_DIR / f"{name}.json"
