"""学習用データの書き出し（training_api.py、SOCAIAGENT-561）。"""
import json
import os

import pytest

SESSIONS = [
    {"id": 1, "application_status": "offered", "utterances": [{"role": "user", "text": "文化祭の実行委員をしました"}]},
    {"id": 2, "application_status": "unknown", "utterances": [{"role": "user", "text": "対象外"}]},
    {"id": 3, "application_status": "rejected", "utterances": [{"role": "ai", "text": "AI の発話は使わない"}]},
]


@pytest.fixture
def export_dir(tmp_path, monkeypatch):
    monkeypatch.setenv("RAG_TRAINING_EXPORT_DIR", str(tmp_path))
    return tmp_path


def test_export_streams_jsonl_for_labeled_sessions(client):
    res = client.post("/training/export", json=SESSIONS)
    assert res.status_code == 200
    lines = [json.loads(line) for line in res.text.splitlines() if line]
    # 選考結果のあるセッションで、受験者の発話があるものだけ
    assert len(lines) == 1


def test_export_rejects_non_array_body(client):
    """本文はセッションの配列だけ（Pydantic で検証する）。"""
    assert client.post("/training/export", json={"id": 1}).status_code == 422
    assert client.post("/training/export", json=["not-an-object"]).status_code == 422


def test_export_to_file_writes_under_export_dir(client, export_dir):
    res = client.post("/training/export_to_file?output_filename=train.jsonl", json=SESSIONS)
    assert res.status_code == 200
    assert res.json() == {"output": "train.jsonl", "count": 1}
    assert (export_dir / "train.jsonl").exists()


@pytest.mark.parametrize("name", [
    "../outside.jsonl",
    "sub/train.jsonl",
    "/etc/passwd.jsonl",
    "C:\\\\Windows\\\\x.jsonl",
    "train.txt",
    ".hidden.jsonl",
    "a" * 120 + ".jsonl",
])
def test_export_to_file_rejects_paths_and_bad_names(client, export_dir, name):
    """書き込み先はディレクトリを含まない .jsonl のファイル名だけ。"""
    res = client.post("/training/export_to_file", params={"output_filename": name}, json=SESSIONS)
    assert res.status_code == 400, name
    assert list(export_dir.iterdir()) == []


def test_export_to_file_rejects_symlink_escape(client, export_dir, tmp_path_factory):
    """書き出し先の中のシンボリックリンクで外を指していても書かない。"""
    outside = tmp_path_factory.mktemp("outside") / "target.jsonl"
    outside.write_text("keep", encoding="utf-8")
    try:
        os.symlink(outside, export_dir / "link.jsonl")
    except (OSError, NotImplementedError):
        pytest.skip("シンボリックリンクを作れない環境")
    res = client.post("/training/export_to_file?output_filename=link.jsonl", json=SESSIONS)
    assert res.status_code == 400
    assert outside.read_text(encoding="utf-8") == "keep"


def test_export_to_file_limit(client, export_dir):
    sessions = [dict(SESSIONS[0], id=i) for i in range(5)]
    res = client.post("/training/export_to_file?output_filename=t.jsonl&limit=2", json=sessions)
    assert res.json()["count"] == 2
    assert client.post("/training/export_to_file?output_filename=t.jsonl&limit=-1", json=sessions).status_code == 400
