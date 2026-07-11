"""Backlog API 共通クライアント（GitHub Actions 用）"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request


def env_required(name: str) -> str:
    raw = os.environ.get(name)
    if raw is None or not str(raw).strip():
        print(
            f"エラー: GitHub Secret `{name}` が未設定または空です。"
            f" Repository Settings → Secrets and variables → Actions で設定してください。",
            file=sys.stderr,
        )
        sys.exit(1)
    return str(raw).strip()


def env_optional(name: str) -> str:
    return (os.environ.get(name) or "").strip()


def normalize_space_id(space_id: str) -> str:
    """myspace.backlog.jp 形式で入っていてもスペースIDだけにする。"""
    s = space_id.strip()
    for suffix in (".backlog.jp", ".backlog.com", ".backlogtool.com"):
        if s.endswith(suffix):
            s = s[: -len(suffix)]
    if "://" in s:
        # https://myspace.backlog.jp → myspace
        host = urllib.parse.urlparse(s if "://" in s else f"https://{s}").hostname or s
        s = host.split(".")[0]
    return s.strip()


def _api_key_query(api_key: str) -> str:
    return urllib.parse.urlencode({"apiKey": api_key})


def build_url(base: str, path: str, api_key: str, extra_query: str = "") -> str:
    """path に既存クエリがあっても apiKey を正しく付与する。"""
    path = path if path.startswith("/") else f"/{path}"
    url = f"{base}{path}"
    parts = [ _api_key_query(api_key) ]
    if extra_query:
        parts.append(extra_query.lstrip("?&"))
    if "?" in url:
        return f"{url}&{'&'.join(parts)}"
    return f"{url}?{'&'.join(parts)}"


def resolve_bl_base(space_id: str, api_key: str, domain: str = "") -> tuple[str, str]:
    """ドメインを自動判定して Backlog API のベース URL を返す。"""
    space_id = normalize_space_id(space_id)
    candidates = [domain] if domain else ["backlog.jp", "backlog.com"]
    last_auth_error: str | None = None

    for d in candidates:
        url = build_url(f"https://{space_id}.{d}/api/v2", "/space", api_key)
        try:
            with urllib.request.urlopen(urllib.request.Request(url), timeout=30) as r:
                r.read()
            print(f"Backlog ドメイン: {d}", flush=True)
            return f"https://{space_id}.{d}/api/v2", d
        except urllib.error.HTTPError as e:
            body = e.read().decode(errors="replace")
            # スペース未存在 → 別ドメインを試す
            if e.code == 404 and '"code":6' in body:
                print(f"  {d}: スペース未検出、次を試行", flush=True)
                continue
            # 自動判定時の 401 はドメイン違いの可能性もあるため次を試す
            if e.code == 401 and not domain:
                last_auth_error = body
                print(f"  {d}: 認証失敗、次を試行", flush=True)
                continue
            _print_auth_help(e.code, body, space_id, d)
            sys.exit(1)
        except urllib.error.URLError as e:
            print(f"  {d}: 接続失敗 ({e.reason})、次を試行", flush=True)
            continue

    if last_auth_error:
        _print_auth_help(401, last_auth_error, space_id, candidates[-1])
        sys.exit(1)

    print(
        f"エラー: スペース '{space_id}' が backlog.jp / backlog.com のいずれにも見つかりませんでした。"
        f" BACKLOG_SPACE_ID / BACKLOG_DOMAIN を確認してください。",
        file=sys.stderr,
    )
    sys.exit(1)


def _print_auth_help(code: int, body: str, space_id: str, domain: str) -> None:
    print(f"Backlog API エラー {code}: {body}", file=sys.stderr)
    print(f"接続先: https://{space_id}.{domain}/api/v2", file=sys.stderr)
    if code == 401:
        print(
            "対処: Backlog → 個人設定 → API → APIキー を再発行し、"
            "GitHub Secrets の BACKLOG_API_KEY を更新してください。"
            "（前後の空白・改行が入っていると 401 になります）",
            file=sys.stderr,
        )


def bl_request(
    base: str,
    api_key: str,
    method: str,
    path: str,
    data: dict | None = None,
    *,
    fatal: bool = True,
):
    """Backlog API を呼び出す。fatal=True のとき失敗でプロセス終了。"""
    extra = ""
    body = None
    headers = {}
    if data is not None and method.upper() == "GET":
        extra = urllib.parse.urlencode(data, doseq=True)
    elif data is not None:
        body = urllib.parse.urlencode(data, doseq=True).encode()
        headers["Content-Type"] = "application/x-www-form-urlencoded"

    # path が "?foo=bar" 付きのレガシー呼び出しにも対応
    if "?" in path:
        path_only, q = path.split("?", 1)
        extra = "&".join(p for p in (q, extra) if p)
        path = path_only

    url = build_url(base, path, api_key, extra)
    req = urllib.request.Request(url, data=body, method=method.upper())
    for k, v in headers.items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=60) as r:
            raw = r.read()
            return json.loads(raw) if raw else None
    except urllib.error.HTTPError as e:
        err_body = e.read().decode(errors="replace")
        print(f"Backlog API エラー {e.code}: {err_body}", file=sys.stderr)
        print(f"リクエスト: {method.upper()} {path}", file=sys.stderr)
        if e.code == 401:
            print(
                "対処: BACKLOG_API_KEY が無効か期限切れです。GitHub Secrets を更新してください。",
                file=sys.stderr,
            )
        if fatal:
            sys.exit(1)
        return None


def load_backlog_env() -> tuple[str, str, str, str]:
    """API_KEY, SPACE_ID, PROJECT_KEY, DOMAIN を読み込む。"""
    api_key = env_required("BACKLOG_API_KEY")
    space_id = normalize_space_id(env_required("BACKLOG_SPACE_ID"))
    proj_key = env_required("BACKLOG_PROJECT_KEY")
    domain = env_optional("BACKLOG_DOMAIN")
    # 鍵の中身は出さず、設定漏れ検知用に長さだけ出す
    print(f"Backlog 設定: space={space_id}, project={proj_key}, apiKey_len={len(api_key)}", flush=True)
    return api_key, space_id, proj_key, domain
