#!/usr/bin/env python3
"""pip-audit の JSON 出力を例外リストと突き合わせ、残った脆弱性があれば失敗させる（SOCAIAGENT-578）。

RAG（Python）の依存には、修正版のない脆弱性が出ることがある。そのまま失敗させると、
該当機能を使っていなくても RAG を触る全 PR の CI が止まる。govulncheck（Go）と同じく、
例外は理由と期限を付けて最小限にする（.github/scripts/govulncheck_filter.py）。

例外が効くのは次をすべて満たす場合だけ。外れたら CI を失敗させて見直しを促す。

- 期限（expires）を過ぎていない
- 検出された脆弱性に修正版が出ていない（出たら上げる）
- 例外の対象のパッケージで検出された
- 例外の前提である使い方をコードがしていない（forbidden_patterns）

使い方: pip_audit_filter.py --report <json> --allowlist <json> --source-root <dir> [--today YYYY-MM-DD]
"""

import argparse
import datetime
import json
import re
import sys
from pathlib import Path

# 前提の使い方を探すファイル。テストは本番で動かないので見ない
_SCANNED_SUFFIXES = {".py", ".sh", ".toml", ".cfg", ".ini", ".yml", ".yaml"}


def findings_from_report(report):
    """{脆弱性 ID: {"packages": set, "fixed": set}} を返す。同じ ID が複数のパッケージで出ることもある。"""
    findings = {}
    for dep in report.get("dependencies", []):
        for vuln in dep.get("vulns", []):
            entry = findings.setdefault(vuln["id"], {"packages": set(), "fixed": set()})
            entry["packages"].add(dep["name"].lower())
            entry["fixed"].update(vuln.get("fix_versions") or [])
    return findings


def scanned_files(source_root):
    for path in sorted(Path(source_root).rglob("*")):
        if not path.is_file() or "tests" in path.parts or "__pycache__" in path.parts:
            continue
        if path.suffix in _SCANNED_SUFFIXES or path.name.startswith("Dockerfile"):
            yield path


def find_forbidden(source_root, patterns):
    """テスト以外のコード・設定から、禁止した使い方を探す（# のコメント行は見ない）。"""
    compiled = [re.compile(pattern) for pattern in patterns]
    hits = []
    for path in scanned_files(source_root):
        for number, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            if line.lstrip().startswith("#"):
                continue
            if any(pattern.search(line) for pattern in compiled):
                hits.append(f"{path}:{number}: {line.strip()}")
    return hits


def rejection_reason(entry, finding, today, source_root):
    """例外を適用できない理由を返す。適用できるなら None。"""
    if today > datetime.date.fromisoformat(entry["expires"]):
        return f'例外の期限（{entry["expires"]}）を過ぎています。理由を見直してください'
    if finding["fixed"]:
        return f'修正版が出ています（{", ".join(sorted(finding["fixed"]))}）。依存を更新し、例外を消してください'
    if finding["packages"] != {entry["package"].lower()}:
        return f'例外の対象外のパッケージで検出されました（{", ".join(sorted(finding["packages"]))}）'
    patterns = entry.get("forbidden_patterns")
    if not patterns:
        return "例外リストの設定エラーです。forbidden_patterns を指定してください"
    hits = find_forbidden(source_root, patterns)
    if hits:
        return "例外の前提である使い方をしています:\n      " + "\n      ".join(hits)
    return None


def evaluate(findings, allowlist, today, source_root):
    """(失敗の一覧, 例外を適用した ID, 検出されなくなった例外の ID) を返す。"""
    entries = {entry["id"]: entry for entry in allowlist}
    failures = []
    allowed = []
    for vuln_id, finding in sorted(findings.items()):
        entry = entries.get(vuln_id)
        if entry is None:
            failures.append(f'{vuln_id}: 例外リストにありません（{", ".join(sorted(finding["packages"]))}）')
            continue
        reason = rejection_reason(entry, finding, today, source_root)
        if reason:
            failures.append(f"{vuln_id}: {reason}")
        else:
            allowed.append(vuln_id)
    stale = sorted(set(entries) - set(findings))
    return failures, allowed, stale


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--report", required=True)
    parser.add_argument("--allowlist", required=True)
    parser.add_argument("--source-root", required=True)
    parser.add_argument("--today", default=None)
    args = parser.parse_args()

    today = datetime.date.fromisoformat(args.today) if args.today else datetime.datetime.now(datetime.timezone.utc).date()
    try:
        report = json.loads(Path(args.report).read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        print(f"FAIL pip-audit の出力を読めません（{exc}）")
        return 1
    # 空の出力を「脆弱性なし」と読まない。スキャンが動かなかっただけかもしれない。
    if not isinstance(report, dict) or not report.get("dependencies"):
        print("FAIL pip-audit の出力を読めません（dependencies がありません）")
        return 1
    allowlist = json.loads(Path(args.allowlist).read_text(encoding="utf-8"))

    failures, allowed, stale = evaluate(findings_from_report(report), allowlist, today, args.source_root)
    print(f"検査したパッケージ: {len(report['dependencies'])} 件")
    for vuln_id in allowed:
        print(f"例外を適用: {vuln_id}")
    for vuln_id in stale:
        print(f"注意: {vuln_id} は検出されなくなりました。例外リストから消してください")
    if failures:
        print(f"FAIL {len(failures)} 件の脆弱性が残っています")
        for failure in failures:
            print(f"  {failure}")
        return 1
    print("PASSED")
    return 0


if __name__ == "__main__":
    sys.exit(main())
