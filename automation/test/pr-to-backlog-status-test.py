#!/usr/bin/env python3
"""pr-to-backlog.yml の埋め込み Python が、ブランチ作成・PR オープン/更新で
Backlog のステータスを意図どおりに動かすかの検査。

ワークフローから埋め込み Python を取り出し、偽の Backlog / GitHub に対して
シナリオごとに実行して、送られた statusId の PATCH を確かめる。
ネットワークも秘密情報も使わない。
"""
import os, re, sys, json, subprocess, tempfile, shutil, textwrap
REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..'))
yml = open(f'{REPO}/.github/workflows/pr-to-backlog.yml', encoding='utf-8').read()
body = yml.split("python3 - << 'PYEOF'\n", 1)[1].split("\n          PYEOF", 1)[0]
script = textwrap.dedent(body)

FAILED = []

def run(name, env, issue_status, pr=None, expect_patch=None, hits=(), expect_warn=False):
    d = tempfile.mkdtemp()
    os.makedirs(f'{d}/.github/scripts')
    src = open(f'{REPO}/.github/scripts/backlog_client.py', encoding='utf-8').read()
    # 偽の Backlog: 課題1件をメモリ上に持ち、PATCH を記録する
    src += f'''
_STATUSES = [{{"id":1,"name":"未対応"}},{{"id":2,"name":"処理中"}},{{"id":5,"name":"PR確認待ち"}},{{"id":3,"name":"処理済み"}},{{"id":6,"name":"ステージング環境反映"}},{{"id":4,"name":"完了"}}]
_ISSUE = {{"status": {{"id": {issue_status[0]}, "name": "{issue_status[1]}"}}, "description": "本文"}}
_LOG = []
def load_backlog_env(): return ("k","s","SOCAIAGENT","backlog.jp")
def resolve_bl_base(*a, **k): return ("http://fake","backlog.jp")
def bl_request(base, api_key, method, path, data=None, fatal=True, **k):
    if method == "GET" and path.endswith("/statuses"): return _STATUSES
    if method == "GET" and path == "/projects/SOCAIAGENT": return {{"id": 99}}
    if method == "GET" and path == "/issues": return {json.dumps(list(hits), ensure_ascii=False)}
    if method == "GET" and path.startswith("/issues/"): return _ISSUE
    if method == "PATCH": _LOG.append((path, data)); return {{}}
    return None
import atexit
atexit.register(lambda: print("PATCHES=" + json.dumps(_LOG, ensure_ascii=False)))
'''
    open(f'{d}/.github/scripts/backlog_client.py', 'w', encoding='utf-8').write(src)
    prelude = ''
    if pr is not None:
        prelude = f'''
import urllib.request, io, json as _j
class _R(io.BytesIO):
    def __enter__(self): return self
    def __exit__(self, *a): pass
def _fake(req, *a, **k): return _R(b'[]' if '/commits' in req.full_url else {json.dumps(json.dumps(pr))}.encode())
urllib.request.urlopen = _fake
'''
    e = dict(os.environ, GH_TOKEN='t', GH_REPO='o/r', **env)
    out = subprocess.run([sys.executable, '-c', prelude + script], cwd=d, env=e, capture_output=True, text=True)
    shutil.rmtree(d)
    m = re.search(r'PATCHES=(.*)', out.stdout)
    patches = json.loads(m.group(1)) if m else None
    status_patches = [p for p in (patches or []) if 'statusId' in (p[1] or {})]
    ok = out.returncode == 0 and (expect_patch is None and not status_patches) or (expect_patch is not None and [p[1]['statusId'] for p in status_patches] == [expect_patch])
    ok = ok and (('::warning' in out.stdout) == expect_warn)
    print(('OK  ' if ok else 'FAIL'), name, '| statusPatches=', [p[1] for p in status_patches], '| rc=', out.returncode)
    if not ok:
        FAILED.append(name)
        print(out.stdout[-600:], out.stderr[-600:])

PRBASE = {'number': 9, 'title': '[SOCAIAGENT-123] x', 'body': '', 'html_url': 'u', 'merged': False, 'base': {'ref': 'develop'}, 'head': {'ref': 'feature/x'}}
run('A ブランチ作成(未対応→処理中)', dict(ACTION='branch_created', BRANCH='feature/SOCAIAGENT-123-foo', PR_NUMBER=''), (1, '未対応'), expect_patch=2)
run('B ブランチ作成(PR確認待ちは動かさない)', dict(ACTION='branch_created', BRANCH='feature/SOCAIAGENT-123-foo', PR_NUMBER=''), (5, 'PR確認待ち'))
run('B2 ブランチ作成(処理中のまま)', dict(ACTION='branch_created', BRANCH='feature/SOCAIAGENT-123-foo', PR_NUMBER=''), (2, '処理中'))
run('B3 ブランチ名にキーなし→警告(黙ってスキップしない)', dict(ACTION='branch_created', BRANCH='chore/cleanup', PR_NUMBER=''), (1, '未対応'), expect_warn=True)
run('C PR opened(処理中→PR確認待ち)', dict(ACTION='opened', PR_NUMBER='9'), (2, '処理中'), pr=PRBASE, expect_patch=5)
run('D PR synchronize(ステータス不変)', dict(ACTION='synchronize', PR_NUMBER='9'), (5, 'PR確認待ち'), pr=PRBASE)
run('D2 PR synchronize(処理中のまま不変)', dict(ACTION='synchronize', PR_NUMBER='9'), (2, '処理中'), pr=PRBASE)
run('E PR opened ドラフト(未対応→処理中)', dict(ACTION='opened', PR_NUMBER='9'), (1, '未対応'), pr=dict(PRBASE, draft=True), expect_patch=2)
run('E2 ドラフト解除(処理中→PR確認待ち)', dict(ACTION='ready_for_review', PR_NUMBER='9'), (2, '処理中'), pr=PRBASE, expect_patch=5)
run('F PR opened(先へ進んだ課題は戻さない)', dict(ACTION='opened', PR_NUMBER='9'), (6, 'ステージング環境反映'), pr=PRBASE)
run('G PR reopened(未対応→PR確認待ち)', dict(ACTION='reopened', PR_NUMBER='9'), (1, '未対応'), pr=PRBASE, expect_patch=5)
run('H PRクローズ(非マージ)は従来どおり未対応', dict(ACTION='closed', PR_NUMBER='9'), (2, '処理中'), pr=PRBASE, expect_patch=1)
run('I ブランチ名の番号→GitHub Issue→Backlogキー', dict(ACTION='branch_created', BRANCH='fix/1374-guest', PR_NUMBER=''), (1, '未対応'),
    pr={'title': '', 'body': '<!-- backlog-key:SOCAIAGENT-77 -->'}, expect_patch=2)
NOKEY = dict(PRBASE, title='feat(company): 企業情報の実データを検証する (#9)')
HIT = [{'issueKey': 'SOCAIAGENT-326', 'summary': 'fix(company): 企業情報の実データを検証する'}]
run('J キー無しPR→課題名の完全一致でPR確認待ち', dict(ACTION='opened', PR_NUMBER='9'), (2, '処理中'), pr=NOKEY, hits=HIT, expect_patch=5)
run('K キー無しPR→一致なしは警告', dict(ACTION='opened', PR_NUMBER='9'), (2, '処理中'), pr=NOKEY, hits=[{'issueKey': 'SOCAIAGENT-1', 'summary': '別の課題'}], expect_warn=True)
run('L キー無しPR→複数一致は更新せず警告', dict(ACTION='opened', PR_NUMBER='9'), (2, '処理中'), pr=NOKEY, hits=HIT + [dict(HIT[0], issueKey='SOCAIAGENT-327')], expect_warn=True)
run('M キー無しPR→マージで警告(develop)', dict(ACTION='closed', PR_NUMBER='9'), (2, '処理中'), pr=dict(NOKEY, merged=True), hits=[], expect_warn=True)
if FAILED:
    print('FAIL:', ', '.join(FAILED))
    sys.exit(1)
print('PASS')
