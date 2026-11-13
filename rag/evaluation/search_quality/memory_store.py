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
    return all(metadata.get(key) == value for key, value in where.items())


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
