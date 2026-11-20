"""検索クエリの短期キャッシュ（SOCAIAGENT-557）。

同じ企業・職種で続けて調査したときに、LLM で検索クエリを作り直さないためのキャッシュ。
TTL は短い（既定 300 秒）ので、コンテナ内のファイルに置く。

以前の実装（research.py の中の読み書き）の問題と、ここでの対処:

- ロックなしの read-modify-write。並列のリクエストで書き込みが失われ、書きかけのファイルを
  読んで壊れることもあった → プロセス内のロックとファイルロック（fcntl）で直列にし、
  一時ファイルへ書いてから os.replace で置き換える（読む側が書きかけを見ない）
- TTL 切れのエントリを消さず、ファイルが無制限に大きくなった → 保存のたびに期限切れを消し、
  件数の上限（新しいものを残す）も掛ける
- 壊れたファイルで毎回読み込みに失敗した → 読めなければ空として扱い、次の保存で作り直す

保存先はコンテナ内（RAG_SEARCH_LOG_DIR）のままで、ECS のタスク間や再起動では共有されない。
TTL が数分のキャッシュなので、共有しないことによる損は「タスクごとに1回クエリを作る」だけで、
共有のための外部ストア（Redis など）は入れていない。
"""
from __future__ import annotations

import json
import logging
import os
import tempfile
import threading
import time
from contextlib import contextmanager
from typing import Dict, Iterator, List, Optional

logger = logging.getLogger("rag-review")

# エントリ数の上限。企業×職種の組み合わせで増えるので、TTL 内でもこれを超えたら古いものから捨てる。
MAX_ENTRIES = 500

_process_lock = threading.Lock()

try:  # コンテナ（Linux）ではファイルロックでプロセス間も直列にする
    import fcntl
except ImportError:  # pragma: no cover - Windows での開発時はプロセス内のロックだけ
    fcntl = None  # type: ignore[assignment]


@contextmanager
def _locked(path: str) -> Iterator[None]:
    """同じキャッシュファイルの読み書きを、スレッド間・プロセス間で直列にする。"""
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    with _process_lock:
        if fcntl is None:
            yield
            return
        with open(path + ".lock", "a+", encoding="utf-8") as lock_file:
            fcntl.flock(lock_file.fileno(), fcntl.LOCK_EX)
            try:
                yield
            finally:
                fcntl.flock(lock_file.fileno(), fcntl.LOCK_UN)


def _read(path: str) -> Dict[str, dict]:
    """キャッシュを読む。無い・壊れている・形が違うときは空として扱う。"""
    if not os.path.exists(path):
        return {}
    try:
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except (OSError, ValueError) as exc:
        logger.warning("query cache is unreadable; starting over path=%s error=%s", path, exc)
        return {}
    return data if isinstance(data, dict) else {}


def _prune(cache: Dict[str, dict], ttl_seconds: int, now: float) -> Dict[str, dict]:
    """期限切れと形の壊れたエントリを消し、件数を上限までに抑える（新しいものを残す）。"""
    alive = {
        key: entry
        for key, entry in cache.items()
        if isinstance(entry, dict)
        and isinstance(entry.get("ts"), (int, float))
        and now - entry["ts"] < ttl_seconds
        and isinstance(entry.get("queries"), list)
    }
    if len(alive) > MAX_ENTRIES:
        newest = sorted(alive.items(), key=lambda kv: kv[1]["ts"], reverse=True)[:MAX_ENTRIES]
        alive = dict(newest)
    return alive


def get(path: str, key: str, ttl_seconds: int, now: Optional[float] = None) -> Optional[List[str]]:
    """TTL 内のクエリを返す。無ければ None。"""
    now = time.time() if now is None else now
    try:
        with _locked(path):
            entry = _read(path).get(key)
    except OSError as exc:
        logger.warning("query cache read failed error=%s", exc)
        return None
    if not isinstance(entry, dict):
        return None
    ts = entry.get("ts")
    queries = entry.get("queries")
    if not isinstance(ts, (int, float)) or not isinstance(queries, list) or now - ts >= ttl_seconds:
        return None
    return [q for q in queries if isinstance(q, str)]


def put(path: str, key: str, queries: List[str], ttl_seconds: int, now: Optional[float] = None) -> None:
    """クエリを保存する。同時に期限切れを消し、件数の上限を掛ける。失敗してもリクエストは止めない。"""
    now = time.time() if now is None else now
    try:
        with _locked(path):
            cache = _prune(_read(path), ttl_seconds, now)
            cache[key] = {"ts": int(now), "queries": list(queries)}
            cache = _prune(cache, ttl_seconds, now)
            # 一時ファイルへ書いてから置き換える。読む側が書きかけのファイルを見ない
            fd, tmp = tempfile.mkstemp(dir=os.path.dirname(path) or ".", prefix=".query_cache.", suffix=".tmp")
            try:
                with os.fdopen(fd, "w", encoding="utf-8") as f:
                    json.dump(cache, f, ensure_ascii=False)
                os.replace(tmp, path)
            except BaseException:
                if os.path.exists(tmp):
                    os.unlink(tmp)
                raise
    except OSError as exc:
        logger.warning("query cache save failed error=%s", exc)
