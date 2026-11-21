"""学生プロフィールの本文を Chroma に残さない（SOCAIAGENT-695）。

検索は埋め込みとメタデータ（user_id）だけで足り、本文は読み戻さない。新規の登録は本文を空にし、
既存分は scripts/clear_student_profile_documents.py で本文だけを空にする（埋め込みは保つ）。
"""
from __future__ import annotations

import os
import subprocess
import sys
import textwrap
from unittest.mock import MagicMock, patch

import pytest

import student_profile_store as sps

RAG_DIR = os.path.join(os.path.dirname(__file__), "..")


def test_upsert_student_does_not_store_text():
    """本文は保存しない。埋め込みとメタデータはそのまま渡す。"""
    collection = MagicMock()
    with patch.object(sps, "get_chroma_client", return_value=MagicMock(
        get_or_create_collection=MagicMock(return_value=collection)
    )):
        sps.upsert_student(7, "合成の自己PR本文", [0.1, 0.2, 0.3])

    kwargs = collection.upsert.call_args.kwargs
    assert kwargs["documents"] == [""]
    assert kwargs["embeddings"] == [[0.1, 0.2, 0.3]]
    assert kwargs["metadatas"][0]["user_id"] == 7


def _clear_with_fake(rows: dict[str, dict], dry_run: bool) -> dict[str, int]:
    """スタブの chromadb 下で、スクリプトの関数を偽のコレクションに対して動かす。"""
    from scripts.clear_student_profile_documents import clear_documents

    class Fake:
        def get(self, ids=None, include=None, limit=None, offset=None):
            keys = list(rows) if ids is None else [i for i in ids if i in rows]
            if ids is None:
                keys = keys[offset or 0:(offset or 0) + (limit or len(keys))]
            out: dict = {"ids": keys}
            if include and "documents" in include:
                out["documents"] = [rows[k]["document"] for k in keys]
            if include and "embeddings" in include:
                out["embeddings"] = [rows[k]["embedding"] for k in keys]
            return out

        def update(self, ids, documents, embeddings):
            for i, d, e in zip(ids, documents, embeddings):
                rows[i]["document"] = d
                rows[i]["embedding"] = list(e)

    return clear_documents(Fake(), dry_run=dry_run, batch=2)


def test_clear_documents_dry_run_changes_nothing():
    rows = {
        "student-1": {"document": "合成A", "embedding": [0.1, 0.2]},
        "student-2": {"document": "", "embedding": [0.3, 0.4]},
        "student-3": {"document": "合成C", "embedding": [0.5, 0.6]},
    }
    result = _clear_with_fake(rows, dry_run=True)
    assert result == {"total": 3, "with_text": 2, "cleared": 0}
    assert rows["student-1"]["document"] == "合成A"


def test_clear_documents_clears_only_text_and_is_idempotent():
    rows = {
        "student-1": {"document": "合成A", "embedding": [0.1, 0.2]},
        "student-2": {"document": "", "embedding": [0.3, 0.4]},
        "student-3": {"document": "合成C", "embedding": [0.5, 0.6]},
    }
    assert _clear_with_fake(rows, dry_run=False) == {"total": 3, "with_text": 2, "cleared": 2}
    assert all(r["document"] == "" for r in rows.values())
    assert rows["student-3"]["embedding"] == [0.5, 0.6]
    assert _clear_with_fake(rows, dry_run=False) == {"total": 3, "with_text": 0, "cleared": 0}


def test_clear_documents_empty_collection():
    assert _clear_with_fake({}, dry_run=False) == {"total": 0, "with_text": 0, "cleared": 0}


# conftest が chromadb をスタブに差し替えるため、本物の chromadb は別プロセスで確かめる。
_REAL_CHROMA = textwrap.dedent(
    """
    import sys, tempfile
    try:
        import chromadb
    except Exception as e:  # Python 3.14 など chromadb が import できない環境
        print("SKIP", type(e).__name__); sys.exit(0)
    sys.path.insert(0, sys.argv[1])
    from scripts.clear_student_profile_documents import clear_documents

    col = chromadb.PersistentClient(path=tempfile.mkdtemp()).get_or_create_collection("student_profiles")
    assert clear_documents(col, dry_run=False) == {"total": 0, "with_text": 0, "cleared": 0}
    embs = [[0.1, 0.2, 0.3], [0.9, 0.1, 0.0], [0.4, 0.4, 0.4]]
    col.upsert(ids=["student-1", "student-2", "student-3"], documents=["合成A", "合成B", ""],
               embeddings=embs, metadatas=[{"user_id": 1}, {"user_id": 2}, {"user_id": 3}])
    q = lambda: col.query(query_embeddings=[[0.2, 0.2, 0.3]], n_results=3, include=["metadatas", "distances"])
    before = q()

    assert clear_documents(col, dry_run=True) == {"total": 3, "with_text": 2, "cleared": 0}
    assert col.get(ids=["student-1"], include=["documents"])["documents"] == ["合成A"]

    assert clear_documents(col, dry_run=False, batch=2) == {"total": 3, "with_text": 2, "cleared": 2}
    got = col.get(ids=["student-1", "student-2", "student-3"], include=["documents", "embeddings", "metadatas"])
    assert got["documents"] == ["", "", ""], got["documents"]
    for e, want in zip(got["embeddings"], embs):
        assert [round(float(x), 5) for x in e] == want
    assert [m["user_id"] for m in got["metadatas"]] == [1, 2, 3]
    after = q()
    assert after["ids"] == before["ids"]
    assert [round(d, 6) for d in after["distances"][0]] == [round(d, 6) for d in before["distances"][0]]

    assert clear_documents(col, dry_run=False) == {"total": 3, "with_text": 0, "cleared": 0}
    print("OK")
    """
)


def test_clear_documents_keeps_embeddings_on_real_chroma(tmp_path):
    env = {**os.environ, "ANONYMIZED_TELEMETRY": "False"}
    proc = subprocess.run(
        [sys.executable, "-c", _REAL_CHROMA, RAG_DIR],
        capture_output=True, text=True, timeout=240, env=env, cwd=tmp_path,
    )
    out = proc.stdout.strip().splitlines()
    if out and out[-1].startswith("SKIP"):
        pytest.skip(f"chromadb を import できない環境: {out[-1]}")
    assert proc.returncode == 0, proc.stderr[-2000:]
    assert out and out[-1] == "OK"
