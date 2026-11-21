"""検索ログ（search_log.jsonl）の大きさの上限と世代（SOCAIAGENT-564）。"""
import json


def _save(main_module, n=1):
    from services.research import _save_search_log

    for i in range(n):
        _save_search_log(
            company_name=f"会社{i}", job_title="", queries=["q"], raw_results=["r"], summary="s",
        )


def test_rotates_when_over_limit_and_keeps_backups(tmp_path, monkeypatch):
    import main

    monkeypatch.setattr(main, "SEARCH_LOG_DIR", str(tmp_path))
    # 1行がおよそ 300 バイトなので、2行ごとに世代を送る
    monkeypatch.setenv("RAG_SEARCH_LOG_MAX_BYTES", "400")
    monkeypatch.setenv("RAG_SEARCH_LOG_BACKUPS", "2")

    _save(main, 10)

    names = sorted(p.name for p in tmp_path.iterdir())
    # 世代は2つまで。それより古いものは消える
    assert names == ["search_log.jsonl", "search_log.jsonl.1", "search_log.jsonl.2"]
    # 今のファイルは上限を大きく超えない（追記の前に送るので、上限 + 1行まで）
    assert (tmp_path / "search_log.jsonl").stat().st_size < 400 + 400
    # 最新の行は今のファイルにある
    last = (tmp_path / "search_log.jsonl").read_text(encoding="utf-8").strip().splitlines()[-1]
    assert json.loads(last)["company_name"] == "会社9"


def test_no_rotation_under_limit(tmp_path, monkeypatch):
    import main

    monkeypatch.setattr(main, "SEARCH_LOG_DIR", str(tmp_path))
    monkeypatch.delenv("RAG_SEARCH_LOG_MAX_BYTES", raising=False)
    _save(main, 3)
    assert [p.name for p in tmp_path.iterdir()] == ["search_log.jsonl"]
    assert len((tmp_path / "search_log.jsonl").read_text(encoding="utf-8").splitlines()) == 3


def test_zero_backups_truncates(tmp_path, monkeypatch):
    import main

    monkeypatch.setattr(main, "SEARCH_LOG_DIR", str(tmp_path))
    monkeypatch.setenv("RAG_SEARCH_LOG_MAX_BYTES", "400")
    monkeypatch.setenv("RAG_SEARCH_LOG_BACKUPS", "0")
    _save(main, 10)
    assert [p.name for p in tmp_path.iterdir()] == ["search_log.jsonl"]


def test_invalid_env_uses_defaults(tmp_path, monkeypatch):
    import main

    monkeypatch.setattr(main, "SEARCH_LOG_DIR", str(tmp_path))
    monkeypatch.setenv("RAG_SEARCH_LOG_MAX_BYTES", "abc")
    monkeypatch.setenv("RAG_SEARCH_LOG_BACKUPS", "xyz")
    _save(main, 2)  # 既定（10MB）なので送らない。例外にもならない
    assert [p.name for p in tmp_path.iterdir()] == ["search_log.jsonl"]
