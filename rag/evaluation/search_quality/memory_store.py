"""評価用のメモリ上の Chroma 互換クライアント（#1773）。

CI では chromadb をスタブに置き換えているので、検索経路（vector_store / student_profile_store）が
使う API だけをメモリ上で再現する。距離は Chroma の既定（l2: ユークリッド距離の2乗）に合わせる。
本物の Chroma で測るときは scripts/eval_search_quality.py --store chroma を使う。
"""
from __future__ import annotations

from typing import Any, Dict, List, Optional


def _matches(metadata: Dict[str, Any], where: Optional[Dict[str, Any]]) -> bool:
    if not where:
        return True
    if "$and" in where:
        return all(_matches(metadata, clause) for clause in where["$and"])
    return all(_match_value(metadata.get(key), value) for key, value in where.items())


def _is_number(value: Any) -> bool:
    return isinstance(value, (int, float)) and not isinstance(value, bool)


def _comparable(a: Any, b: Any) -> bool:
    """本物の Chroma と同じく、型の違う値（数値と文字列など）は比較せず、条件に当たらない扱いにする。"""
    return (_is_number(a) and _is_number(b)) or (isinstance(a, str) and isinstance(b, str))


_OPS = {
    "$eq": lambda a, b: a == b,
    "$ne": lambda a, b: a != b,
    "$gt": lambda a, b: _comparable(a, b) and a > b,
    "$gte": lambda a, b: _comparable(a, b) and a >= b,
    "$lt": lambda a, b: _comparable(a, b) and a < b,
    "$lte": lambda a, b: _comparable(a, b) and a <= b,
    "$in": lambda a, b: a in b,
    "$nin": lambda a, b: a not in b,
}


def _match_value(actual: Any, cond: Any) -> bool:
    # Chroma と同じく {"$gte": n} などの比較演算子を受ける（SOCAIAGENT-659 の TTL 条件）。
    # 値を持たない文書・型の違う値は比較演算子に当たらない
    if isinstance(cond, dict):
        return all(_OPS[op](actual, operand) for op, operand in cond.items())
    return actual == cond


def _l2_squared(a: List[float], b: List[float]) -> float:
    return sum((x - y) ** 2 for x, y in zip(a, b))


class MemoryCollection:
    def __init__(self, name: str) -> None:
        self.name = name
        self._rows: Dict[str, Dict[str, Any]] = {}

    def upsert(self, ids, documents=None, embeddings=None, metadatas=None) -> None:
        for index, doc_id in enumerate(ids):
            self._rows[doc_id] = {
                "document": documents[index] if documents else None,
                "embedding": list(embeddings[index]),
                "metadata": dict(metadatas[index]) if metadatas else {},
            }

    add = upsert

    def delete(self, ids=None, where=None) -> None:
        for doc_id in list(self._rows):
            if (ids and doc_id in ids) or (where and _matches(self._rows[doc_id]["metadata"], where)):
                del self._rows[doc_id]

    def get(self, ids=None, where=None, include=None, limit=None, offset=0) -> Dict[str, Any]:
        rows = [
            (doc_id, row)
            for doc_id, row in self._rows.items()
            if (ids is None or doc_id in ids) and _matches(row["metadata"], where)
        ]
        rows = rows[offset:] if limit is None else rows[offset:offset + limit]
        return {
            "ids": [doc_id for doc_id, _ in rows],
            "documents": [row["document"] for _, row in rows],
            "metadatas": [row["metadata"] for _, row in rows],
        }

    def update(self, ids, metadatas=None, documents=None, embeddings=None) -> None:
        # 本物の Chroma と同じく、メタデータは項目ごとにマージする（渡さなかった項目は残る）
        for index, doc_id in enumerate(ids):
            row = self._rows.get(doc_id)
            if row is None:
                continue
            if metadatas:
                row["metadata"] = {**row["metadata"], **dict(metadatas[index])}
            if documents:
                row["document"] = documents[index]
            if embeddings:
                row["embedding"] = list(embeddings[index])

    def count(self) -> int:
        return len(self._rows)

    def query(self, query_embeddings, n_results=10, where=None, include=None) -> Dict[str, Any]:
        result: Dict[str, List[List[Any]]] = {"ids": [], "documents": [], "metadatas": [], "distances": []}
        for embedding in query_embeddings:
            # 挿入順を同点の順位に使い、結果を決定的にする
            candidates = [
                (_l2_squared(embedding, row["embedding"]), order, doc_id, row)
                for order, (doc_id, row) in enumerate(self._rows.items())
                if _matches(row["metadata"], where)
            ]
            candidates.sort(key=lambda item: (item[0], item[1]))
            top = candidates[:n_results]
            result["ids"].append([doc_id for _, _, doc_id, _ in top])
            result["documents"].append([row["document"] for _, _, _, row in top])
            result["metadatas"].append([row["metadata"] for _, _, _, row in top])
            result["distances"].append([distance for distance, _, _, _ in top])
        return result


class MemoryClient:
    def __init__(self) -> None:
        self._collections: Dict[str, MemoryCollection] = {}

    def list_collections(self) -> List[str]:
        return list(self._collections)

    def get_collection(self, name: str) -> MemoryCollection:
        if name not in self._collections:
            raise ValueError(f"collection {name} does not exist")
        return self._collections[name]

    def get_or_create_collection(self, name: str, **_: Any) -> MemoryCollection:
        return self._collections.setdefault(name, MemoryCollection(name))

    def delete_collection(self, name: str) -> None:
        self._collections.pop(name, None)
