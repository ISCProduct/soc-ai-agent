"""Backlog API 共通クライアント（GitHub Actions 用）"""

from __future__ import annotations

import json
import os
import re
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


# GitHub -> Backlog 同期で、Backlog 起源の Issue に対して何を同期してよいか。
#
# Backlog 起源(本文に <!-- backlog-synced -->)の Issue はタイトル・本文を
# 書き戻すとループするため同期しない。ただし開閉だけは返す必要がある。
# 返さないと、Backlog 起源の課題は GitHub を閉じても Backlog が未対応のまま残る
# (実際に SOCAIAGENT-146/147/148 が本番反映後も未対応のままだった)。
#
# 逆方向(backlog-to-github-issue.yml)も、GitHub 起源の課題について
# 「タイトル・本文は書き換えず、状態だけ反映する」という同じ扱いをしている。
# set_issue_status は同じステータスなら PATCH しないので、往復しても止まる。
_STATE_ACTIONS = ("closed", "reopened")


def should_skip_backlog_sync(body: str, action: str) -> bool:
    """Backlog 起源の Issue で、この action の同期を丸ごと飛ばすべきか。"""
    if "<!-- backlog-synced -->" not in (body or ""):
        return False
    return action not in _STATE_ACTIONS


# Backlog の優先度ID。プロジェクト共通の固定値（2=高 / 3=中 / 4=低）。
BACKLOG_PRIORITY_IDS = {"高": 2, "中": 3, "低": 4}

# 優先度が読み取れないときの既定。従来の固定値と同じ「中」。
DEFAULT_PRIORITY_ID = 3

# Issue Form (.github/ISSUE_TEMPLATE/bug_report.yml) の
# 「### 優先度」セクション直下の1行を拾う。
_PRIORITY_SECTION = re.compile(r"^###\s*優先度\s*$\n+(.+)$", re.MULTILINE)


def parse_priority_id(body: str) -> int:
    """GitHub Issue 本文の優先度セレクトから Backlog の priorityId を返す。

    Issue Form の dropdown は本文へ

        ### 優先度

        高 — サービス停止・情報漏えい・データ破損・金銭被害

    の形で展開される。選択肢は説明つきなので、先頭の 高/中/低 だけを見る。
    セクションが無い（フォームを使わず起票された、Backlog起源など）場合は
    既定の「中」を返す。優先度が読めないという理由で同期を止めない。
    """
    m = _PRIORITY_SECTION.search(body or "")
    if not m:
        return DEFAULT_PRIORITY_ID
    value = m.group(1).strip()
    if value in ("_No response_", ""):
        return DEFAULT_PRIORITY_ID
    for name, pid in BACKLOG_PRIORITY_IDS.items():
        if value.startswith(name):
            return pid
    return DEFAULT_PRIORITY_ID


# マージ同期が課題を「前へ進めるだけ」にするための順序。
#
# マージ同期はPRのコミット範囲から過去の課題キーまで拾うため、順序を見ずに
# マージ先ブランチのステータスを当てると、develop へマージするたびに
# 本番反映済み(完了)の課題まで「ステージング環境反映」へ引き戻される。
# 実際に SOCAIAGENT-289 が2回、SOCAIAGENT-311 が1回巻き戻された。
#
# 「保留」は流れの外だが、マージが起きた＝作業が再開したとみなして
# 未対応と同じ位置に置き、前進を許す。
MERGE_STATUS_ORDER = (
    "未対応",
    "保留",
    "処理中",
    "PR確認待ち",
    "処理済み",
    "ステージング環境反映",
    "リリース待ち",
    "完了",
)


def is_status_advance(current_name: str, target_name: str) -> bool:
    """target が current より後（前進）なら True。

    どちらかが順序表に無い（プロジェクト固有のステータスが増えた等）場合は
    判断できないので True を返し、従来どおり更新する。
    順序を知らないという理由で同期が止まる方が困るため。
    """
    order = {name: i for i, name in enumerate(MERGE_STATUS_ORDER)}
    current = order.get((current_name or "").strip())
    target = order.get((target_name or "").strip())
    if current is None or target is None:
        return True
    return target > current


def set_issue_status_unless_same(
    base: str,
    api_key: str,
    issue_key: str,
    status_id: int,
    request=None,
) -> str:
    """PRオープン/更新時のステータス更新。同じステータスなら PATCH しない。

    Backlog は無変更の PATCH を 400 `No comment content.` (code 7) で拒否する。
    毎回無条件に PATCH していたため、既に「処理中」の課題を持つ PR を更新するたびに
    400 が出て失敗扱いになり、**PR URL の追記まで巻き添えでスキップ**されていた
    (SOCAIAGENT-153 で実際に発生)。

    後退（例: ステージング反映 → 処理中）は許す。
    そのPRの課題1件に対する明示的な操作であり、advance_issue_status とは
    意図が違う（あちらはコミット範囲から拾った過去の課題を守るためのもの）。

    戻り値: "updated" / "skipped"（既に同じ）/ "failed"。
    request はテスト用の差し替え口（省略時は bl_request）。
    """
    call = request or (lambda method, path, data=None: bl_request(base, api_key, method, path, data, fatal=False))

    issue = call("GET", f"/issues/{issue_key}")
    current_id = ((issue or {}).get("status") or {}).get("id")
    if current_id == status_id:
        print(f"{issue_key}: 既に同じステータスのため更新しない")
        return "skipped"

    if call("PATCH", f"/issues/{issue_key}", {"statusId": status_id}) is None:
        return "failed"
    return "updated"


def advance_issue_status(
    base: str,
    api_key: str,
    issue_key: str,
    status_id: int,
    status_name: str,
    request=None,
) -> str:
    """マージ同期用のステータス更新。後退させない。

    戻り値: "updated" / "skipped"（前進でない）/ "failed"。
    request はテスト用の差し替え口（省略時は bl_request）。
    """
    call = request or (lambda method, path, data=None: bl_request(base, api_key, method, path, data, fatal=False))

    issue = call("GET", f"/issues/{issue_key}")
    current_name = ((issue or {}).get("status") or {}).get("name", "")
    if current_name and not is_status_advance(current_name, status_name):
        print(f"{issue_key}: 既に '{current_name}' のため '{status_name}' へは戻さない")
        return "skipped"

    if call("PATCH", f"/issues/{issue_key}", {"statusId": status_id}) is None:
        return "failed"
    return "updated"


def set_issue_status(base: str, api_key: str, issue_key: str, status_id: int) -> bool:
    """課題のステータスを更新する。既に同じステータスなら何もしない。

    Backlog は「変化のない更新」を 400 `No comment content.` (code 7) で拒否する。
    GitHub より先に Backlog 側でステータスを変えていると必ずこれに当たり、
    同期結果は正しいのにワークフローだけが赤くなる
    （実際に SOCAIAGENT-256/257/258/262/263/267/303 の7件が同時に失敗した）。

    エラーはここでは握り潰さない。APIキー失効などをサイレントに通すと
    同期が止まったことに誰も気づけないため、変更不要のケースだけを除外する。

    戻り値: 実際に更新したら True、変更不要なら False。
    """
    issue = bl_request(base, api_key, "GET", f"/issues/{issue_key}")
    current = (issue or {}).get("status") or {}
    if current.get("id") == status_id:
        print(f"{issue_key}: 既に '{current.get('name')}' のため変更不要です。")
        return False
    bl_request(base, api_key, "PATCH", f"/issues/{issue_key}", {"statusId": status_id})
    print(f"{issue_key}: '{current.get('name')}' → ステータス {status_id} に更新しました。")
    return True


if __name__ == "__main__":
    # 自己チェック: python3 .github/scripts/backlog_client.py
    import types

    calls: list[tuple] = []

    def _fake(base, api_key, method, path, data=None, **kw):
        calls.append((method, path, data))
        if method == "GET":
            return {"status": {"id": _fake.current_id, "name": "現在"}}
        return {}

    _orig = bl_request
    globals()["bl_request"] = _fake

    _fake.current_id = 4
    calls.clear()
    assert set_issue_status("b", "k", "X-1", 4) is False, "同ステータスなら更新しない"
    assert all(m == "GET" for m, _, _ in calls), f"PATCH を送ってはいけない: {calls}"

    _fake.current_id = 1
    calls.clear()
    assert set_issue_status("b", "k", "X-1", 4) is True, "違うステータスなら更新する"
    assert ("PATCH", "/issues/X-1", {"statusId": 4}) in calls, f"PATCH が無い: {calls}"

    # --- 前進のみガード ---
    # 実際に起きた巻き戻しをそのままケースにする
    assert not is_status_advance("完了", "ステージング環境反映"), "SOCAIAGENT-289/311 の巻き戻し"
    assert not is_status_advance("リリース待ち", "ステージング環境反映")
    assert not is_status_advance("完了", "リリース待ち")
    assert not is_status_advance("完了", "完了"), "同じステータスは前進ではない"
    # 正常な前進は通す
    assert is_status_advance("処理中", "ステージング環境反映")
    assert is_status_advance("ステージング環境反映", "リリース待ち")
    assert is_status_advance("リリース待ち", "完了")
    assert is_status_advance("未対応", "完了")
    assert is_status_advance("保留", "ステージング環境反映"), "保留はマージで再開したとみなす"
    # 判断できないときは従来どおり更新する（順序を知らないだけで同期を止めない）
    assert is_status_advance("知らないステータス", "完了")
    assert is_status_advance("完了", "知らないステータス")
    assert is_status_advance("", "完了")

    # advance_issue_status: 後退は PATCH を送らない
    sent: list[tuple] = []

    def _req(current):
        def call(method, path, data=None):
            sent.append((method, path, data))
            if method == "GET":
                return {"status": {"name": current}}
            return {}
        return call

    sent.clear()
    assert advance_issue_status("b", "k", "X-1", 4, "ステージング環境反映", request=_req("完了")) == "skipped"
    assert all(m == "GET" for m, _, _ in sent), f"後退時に PATCH を送っている: {sent}"

    sent.clear()
    assert advance_issue_status("b", "k", "X-1", 4, "完了", request=_req("リリース待ち")) == "updated"
    assert ("PATCH", "/issues/X-1", {"statusId": 4}) in sent, f"前進時に PATCH が無い: {sent}"

    # 現在ステータスを取得できないときは従来どおり更新する。
    # 取得失敗を理由に同期が止まる方が困る。
    sent.clear()

    def _req_get_failed(method, path, data=None):
        sent.append((method, path, data))
        return None if method == "GET" else {}

    assert advance_issue_status("b", "k", "X-1", 4, "完了", request=_req_get_failed) == "updated"
    assert ("PATCH", "/issues/X-1", {"statusId": 4}) in sent, f"取得失敗時に PATCH が無い: {sent}"

    # PATCH が失敗したら failed を返す（成功したことにしない）
    sent.clear()

    def _req_patch_failed(method, path, data=None):
        sent.append((method, path, data))
        return {"status": {"name": "リリース待ち"}} if method == "GET" else None

    assert advance_issue_status("b", "k", "X-1", 4, "完了", request=_req_patch_failed) == "failed"

    # set_issue_status_unless_same: 同じステータスなら PATCH を送らない。
    # 無変更 PATCH は Backlog が 400 code 7 で拒否し、PR URL 追記まで巻き添えになる。
    def _req_id(current_id):
        def call(method, path, data=None):
            sent.append((method, path, data))
            if method == "GET":
                return {"status": {"id": current_id, "name": "処理中"}}
            return {}
        return call

    sent.clear()
    assert set_issue_status_unless_same("b", "k", "X-1", 2, request=_req_id(2)) == "skipped"
    assert all(m == "GET" for m, _, _ in sent), f"同ステータスで PATCH を送っている: {sent}"

    sent.clear()
    assert set_issue_status_unless_same("b", "k", "X-1", 2, request=_req_id(1)) == "updated"
    assert ("PATCH", "/issues/X-1", {"statusId": 2}) in sent, f"PATCH が無い: {sent}"

    # 後退は許す（PRを開き直したときに「処理中」へ戻すのは意図どおり）
    sent.clear()
    assert set_issue_status_unless_same("b", "k", "X-1", 2, request=_req_id(4)) == "updated"

    # 取得できないときは更新する（取得失敗で同期が止まる方が困る）
    sent.clear()
    assert set_issue_status_unless_same("b", "k", "X-1", 2, request=_req_get_failed) == "updated"

    # PATCH 失敗は failed（成功したことにしない）
    sent.clear()

    def _req_id_patch_failed(method, path, data=None):
        sent.append((method, path, data))
        return {"status": {"id": 1, "name": "未対応"}} if method == "GET" else None

    assert set_issue_status_unless_same("b", "k", "X-1", 2, request=_req_id_patch_failed) == "failed"

    # --- 優先度セレクト → priorityId ---
    form = "### 優先度\n\n高 — サービス停止・情報漏えい・データ破損・金銭被害\n\n### 事象\n\nx"
    assert parse_priority_id(form) == 2, "セレクトの「高」が Backlog の高(2)になる"
    assert parse_priority_id("### 優先度\n\n中 — 一部機能が使えない") == 3
    assert parse_priority_id("### 優先度\n\n低 — 軽微") == 4
    # マーカーが先頭に付いた本文（github-issue-to-backlog が書き戻す形）でも読める
    assert parse_priority_id("<!-- backlog-key:X-1 -->\n\n### 優先度\n\n高") == 2
    # フォーム未使用・未回答・未知の値は既定の「中」。同期を止めない。
    assert parse_priority_id("ただの本文") == DEFAULT_PRIORITY_ID
    assert parse_priority_id("") == DEFAULT_PRIORITY_ID
    assert parse_priority_id(None) == DEFAULT_PRIORITY_ID
    assert parse_priority_id("### 優先度\n\n_No response_") == DEFAULT_PRIORITY_ID
    assert parse_priority_id("### 優先度\n\n激高") == DEFAULT_PRIORITY_ID
    # 見出しが優先度で「始まる」だけの別セクションは拾わない
    assert parse_priority_id("### 優先度の根拠\n\n高") == DEFAULT_PRIORITY_ID

    # --- Backlog 起源 Issue の同期範囲 ---
    synced = "<!-- backlog-synced -->\n<!-- backlog-key:X-1 -->\n本文"
    # 開閉は Backlog へ返す。返さないと GitHub を閉じても Backlog が未対応のまま残る。
    assert should_skip_backlog_sync(synced, "closed") is False
    assert should_skip_backlog_sync(synced, "reopened") is False
    # タイトル・本文の書き戻しはループするので飛ばす
    assert should_skip_backlog_sync(synced, "opened") is True
    assert should_skip_backlog_sync(synced, "edited") is True
    # GitHub 起源はどの action も通常どおり同期する
    assert should_skip_backlog_sync("ふつうの本文", "edited") is False
    assert should_skip_backlog_sync("", "opened") is False
    assert should_skip_backlog_sync(None, "closed") is False

    globals()["bl_request"] = _orig
    assert normalize_space_id("https://myspace.backlog.jp") == "myspace"
    assert build_url("https://x/api/v2", "/issues?a=b", "K").count("?") == 1
    print("OK")
