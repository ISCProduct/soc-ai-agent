"""学生プロフィールの本文を Chroma に残さない（SOCAIAGENT-695）。

検索は埋め込みとメタデータ（user_id）だけで足り、本文は読み戻さない。新規の登録は本文を空にし、
既存分は scripts/clear_student_profile_documents.py で本文のないコレクションへ差し替える（埋め込みは保つ）。
chromadb 0.6.3 は get() で本文が空に見えても SQLite の embeddings_queue などに本文を残すことがあるため、
本物の chromadb を一時ディレクトリで動かし、SQLite の中身まで確かめる。
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


# conftest が chromadb をスタブに差し替えるため、本物の chromadb は別プロセスで確かめる。
# 本文は合成の ASCII 文字列（キューの JSON は非 ASCII をエスケープするので、検索しやすくするため）。
_PRELUDE = textwrap.dedent(
    """
    import os, sqlite3, sys, tempfile
    try:
        import chromadb
    except Exception as e:  # chromadb が import できない環境
        print("SKIP", type(e).__name__); sys.exit(0)
    sys.path.insert(0, sys.argv[1])
    MARK = "synthetic-profile-text"
    TABLES = {
        "embeddings_queue": "metadata",
        "embedding_metadata": "string_value",
        "embedding_fulltext_search": "string_value",
        "embedding_fulltext_search_content": "c0",
    }

    def text_rows(path):
        db = sqlite3.connect(os.path.join(path, "chroma.sqlite3"))
        try:
            return {t: db.execute(f"select count(*) from {t} where {c} like ?", (f"%{MARK}%",)).fetchone()[0]
                    for t, c in TABLES.items()}
        finally:
            db.close()

    def raw_bytes_after_vacuum(path):
        db = sqlite3.connect(os.path.join(path, "chroma.sqlite3"))
        db.execute("VACUUM"); db.close()
        with open(os.path.join(path, "chroma.sqlite3"), "rb") as f:
            return MARK.encode() in f.read()
    """
)

_CLEAR_SCRIPT = _PRELUDE + textwrap.dedent(
    """
    from scripts.clear_student_profile_documents import clear_documents

    path = tempfile.mkdtemp()
    client = chromadb.PersistentClient(path=path)
    col = client.get_or_create_collection("student_profiles", metadata={"hnsw:space": "l2"})
    assert clear_documents(client, dry_run=False) == {"total": 0, "with_text": 0, "cleared": 0}
    embs = [[0.1, 0.2, 0.3], [0.9, 0.1, 0.0], [0.4, 0.4, 0.4]]
    col.upsert(ids=["student-1", "student-2", "student-3"], documents=[MARK + "-1", MARK + "-2", ""],
               embeddings=embs, metadatas=[{"user_id": 1}, {"user_id": 2}, {"user_id": 3}])
    q = lambda: client.get_collection("student_profiles").query(
        query_embeddings=[[0.2, 0.2, 0.3]], n_results=3, include=["metadatas", "distances"])
    before = q()

    assert clear_documents(client, dry_run=True) == {"total": 3, "with_text": 2, "cleared": 0}
    assert col.get(ids=["student-1"], include=["documents"])["documents"] == [MARK + "-1"]
    assert text_rows(path)["embeddings_queue"] == 2, text_rows(path)

    assert clear_documents(client, dry_run=False, batch=2) == {"total": 3, "with_text": 2, "cleared": 2}
    assert set(client.list_collections()) == {"student_profiles"}, client.list_collections()
    new = client.get_collection("student_profiles")
    assert new.metadata == {"hnsw:space": "l2"}, new.metadata
    got = new.get(ids=["student-1", "student-2", "student-3"], include=["documents", "embeddings", "metadatas"])
    assert got["documents"] == ["", "", ""], got["documents"]
    for e, want in zip(got["embeddings"], embs):
        assert [round(float(x), 5) for x in e] == want
    assert [m["user_id"] for m in got["metadatas"]] == [1, 2, 3]
    after = q()
    assert after["ids"] == before["ids"]
    assert [round(d, 6) for d in after["distances"][0]] == [round(d, 6) for d in before["distances"][0]]

    # get() で空に見えるだけでなく、キューや全文検索の表にも本文が残っていないこと
    assert all(n == 0 for n in text_rows(path).values()), text_rows(path)
    assert clear_documents(client, dry_run=False) == {"total": 3, "with_text": 0, "cleared": 0}
    assert set(client.list_collections()) == {"student_profiles"}
    # 行を消しても空き領域にはバイトが残る。手順どおり VACUUM すればファイルからも消える
    assert not raw_bytes_after_vacuum(path)
    print("OK")
    """
)

_UPSERT_SCRIPT = _PRELUDE + textwrap.dedent(
    """
    path = tempfile.mkdtemp()
    os.environ.pop("CHROMA_HOST", None)
    os.environ["RAG_CHROMA_DATA_DIR"] = path
    import student_profile_store as sps

    sps.upsert_student(7, MARK + "-7", [0.1, 0.2, 0.3])
    sps.upsert_student(7, MARK + "-7b", [0.2, 0.2, 0.3])
    assert sps._collection().get(ids=["student-7"], include=["documents"])["documents"] == [""]
    assert all(n == 0 for n in text_rows(path).values()), text_rows(path)
    with open(os.path.join(path, "chroma.sqlite3"), "rb") as f:
        assert MARK.encode() not in f.read()
    print("OK")
    """
)


@pytest.mark.parametrize("script", [_CLEAR_SCRIPT, _UPSERT_SCRIPT], ids=["clear-script", "upsert-student"])
def test_no_text_left_in_real_chroma_sqlite(tmp_path, script):
    env = {**os.environ, "ANONYMIZED_TELEMETRY": "False"}
    proc = subprocess.run(
        [sys.executable, "-c", script, RAG_DIR],
        capture_output=True, text=True, timeout=240, env=env, cwd=tmp_path,
    )
    out = proc.stdout.strip().splitlines()
    if out and out[-1].startswith("SKIP"):
        pytest.skip(f"chromadb を import できない環境: {out[-1]}")
    assert proc.returncode == 0, proc.stderr[-2000:]
    assert out and out[-1] == "OK"
