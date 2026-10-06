#!/usr/bin/env bash
# ワークフローへ埋め込んだ Python が構文的に通るか検査する。
#
# .github/workflows/*.yml の `python3 - << 'PYEOF' ... PYEOF` は、その
# ワークフローが実際に起動するまで構文エラーに気づけない。
# github-issue-to-backlog.yml は issues イベントで動くため、
# 「Issue を閉じたときに初めて落ちる」という形で表に出る。
#
# test.yml の「Backlog クライアントの自己チェック」は
# 「埋め込みコードのコンパイルも合わせて確認する」とコメントしていたが、
# 実際に走っていたのは backlog_client.py の自己チェックだけで、
# 埋め込み側は検査されていなかった（SOCAIAGENT-415 の修正中に実証）。
set -euo pipefail

WORKFLOW_DIR="${1:-.github/workflows}"

if [ ! -d "$WORKFLOW_DIR" ]; then
  echo "::error::ワークフローのディレクトリが見つかりません: $WORKFLOW_DIR"
  exit 1
fi

python3 - "$WORKFLOW_DIR" <<'PY'
import pathlib
import re
import sys

workflow_dir = pathlib.Path(sys.argv[1])
# ヒアドキュメントの終端ラベルは PYEOF 以外も使われうるので拾える形にする。
pattern = re.compile(r"python3 - ?<< ?'(?P<label>\w+)'\n(?P<body>.*?)\n\s*(?P=label)\s*$", re.S | re.M)

checked = 0
failed = 0
for path in sorted(workflow_dir.glob("*.yml")):
    src = path.read_text(encoding="utf-8")
    for match in pattern.finditer(src):
        block = match.group("body")
        lines = block.split("\n")
        # YAML のインデントぶんを落とす。空行はそのまま残す。
        widths = [len(l) - len(l.lstrip()) for l in lines if l.strip()]
        indent = min(widths) if widths else 0
        code = "\n".join(l[indent:] if len(l) >= indent else l for l in lines)
        # ブロックの開始行番号。エラー位置を元ファイルの行で示すために足す。
        offset = src[: match.start("body")].count("\n")
        checked += 1
        try:
            compile(code, str(path), "exec")
        except SyntaxError as e:
            failed += 1
            lineno = (e.lineno or 1) + offset
            print(f"::error file={path},line={lineno}::埋め込み Python の構文エラー: {e.msg}")
            print(f"  {path}:{lineno} {(e.text or '').rstrip()}", file=sys.stderr)

if checked == 0:
    print("::error::埋め込み Python のブロックが1つも見つかりません。抽出パターンが古い可能性があります。")
    sys.exit(1)

if failed:
    print(f"::error::{failed}/{checked} 件の埋め込み Python が構文エラーです。")
    sys.exit(1)

print(f"埋め込み Python {checked} ブロックの構文 OK")
PY
