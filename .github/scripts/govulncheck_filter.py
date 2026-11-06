#!/usr/bin/env python3
"""govulncheck の JSON 出力を例外リストと突き合わせ、残った脆弱性があれば失敗させる。

govulncheck には例外の仕組みがない。修正版のない脆弱性が公開されると、該当機能を
使っていなくても全 PR の CI が止まる。例外は理由と期限を付けて最小限にする。

例外が効くのは次をすべて満たす場合だけ。外れたら CI を失敗させて見直しを促す。

- 期限（expires）を過ぎていない
- 検出された脆弱性に修正版が出ていない（出たら上げる）
- 例外の前提である機能をコードが使っていない（forbidden_patterns）

使い方: govulncheck_filter.py --report <json> --allowlist <json> --source-root <dir> [--today YYYY-MM-DD]
"""

import argparse
import datetime
import json
import re
import sys
from pathlib import Path


def parse_stream(text):
    """govulncheck -format json は JSON オブジェクトを連結して出力する。"""
    decoder = json.JSONDecoder()
    messages = []
    pos = 0
    while True:
        while pos < len(text) and text[pos].isspace():
            pos += 1
        if pos >= len(text):
            return messages
        message, pos = decoder.raw_decode(text, pos)
        messages.append(message)


def called_findings(messages):
    """コードから到達するものだけを返す。先頭のフレームに function があるものが該当する。"""
    findings = {}
    for message in messages:
        finding = message.get("finding")
        if not finding or not finding.get("trace"):
            continue
        top = finding["trace"][0]
        if not top.get("function"):
            continue
        entry = findings.setdefault(finding["osv"], {"modules": set(), "fixed": set(), "symbols": set()})
        entry["modules"].add(top.get("module", ""))
        if finding.get("fixed_version"):
            entry["fixed"].add(finding["fixed_version"])
        symbol = top.get("function", "")
        if top.get("receiver"):
            symbol = f'{top["receiver"]}.{symbol}'
        entry["symbols"].add(f'{top.get("package", "")}.{symbol}')
    return findings


def find_forbidden(source_root, patterns):
    """テスト以外の Go コードから、禁止した使い方を探す。"""
    compiled = [re.compile(pattern) for pattern in patterns]
    hits = []
    for path in sorted(Path(source_root).rglob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            if line.lstrip().startswith("//"):
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
    if finding["modules"] != {entry["module"]}:
        return f'例外の対象外のモジュールで検出されました（{", ".join(sorted(finding["modules"]))}）'
    hits = find_forbidden(source_root, entry.get("forbidden_patterns", []))
    if hits:
        return "例外の前提である機能を使っています:\n      " + "\n      ".join(hits)
    return None


def evaluate(findings, allowlist, today, source_root):
    """(失敗の一覧, 例外を適用した ID, 検出されなくなった例外の ID) を返す。"""
    entries = {entry["id"]: entry for entry in allowlist}
    failures = []
    allowed = []
    for osv, finding in sorted(findings.items()):
        entry = entries.get(osv)
        if entry is None:
            failures.append(f'{osv}: 例外リストにありません（例: {min(finding["symbols"])}）')
            continue
        reason = rejection_reason(entry, finding, today, source_root)
        if reason:
            failures.append(f"{osv}: {reason}")
        else:
            allowed.append(osv)
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
    messages = parse_stream(Path(args.report).read_text(encoding="utf-8"))
    # 空の出力を「脆弱性なし」と読まない。スキャンが動かなかっただけかもしれない。
    if not any("config" in message for message in messages):
        print("FAIL govulncheck の出力を読めません（config がありません）")
        return 1
    allowlist = json.loads(Path(args.allowlist).read_text(encoding="utf-8"))

    failures, allowed, stale = evaluate(called_findings(messages), allowlist, today, args.source_root)
    for osv in allowed:
        print(f"例外を適用: {osv}")
    for osv in stale:
        print(f"注意: {osv} は検出されなくなりました。例外リストから消してください")
    if failures:
        print(f"FAIL {len(failures)} 件の脆弱性が残っています")
        for failure in failures:
            print(f"  {failure}")
        return 1
    print("PASSED")
    return 0


if __name__ == "__main__":
    sys.exit(main())
