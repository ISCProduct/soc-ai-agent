"""
データ収集・サニタイズ・JSONL 出力スクリプト（スケルトン）
使用例:
    python3 rag/training/export_jsonl.py --output data/train_data.jsonl --source-file data/raw_events.jsonl

注意: 実運用では DB コネクションやPII除去ルールをプロダクション要件に合わせて拡張すること。
"""
import argparse
import json
import re
import sys
from typing import Iterator, Dict, Any

PII_EMAIL_RE = re.compile(r"[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}")
PII_PHONE_RE = re.compile(r"\+?[0-9][0-9\-\s]{6,}[0-9]")


def sanitize_text(text: str) -> str:
    """簡易 PII マスキング: メール・電話番号を置換する。必要に応じて拡張する。"""
    text = PII_EMAIL_RE.sub("[EMAIL_REDACTED]", text)
    text = PII_PHONE_RE.sub("[PHONE_REDACTED]", text)
    # 追加のサニタイズルールをここに
    return text


def read_source_file(path: str) -> Iterator[Dict[str, Any]]:
    """既存の JSONL ファイルを読み、辞書を返す（サンプル用）。
    運用時は DB からの抽出や API 経由収集に置き換える。"""
    with open(path, "r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                yield json.loads(line)
            except Exception:
                # 読めない行はスキップ
                continue


def to_training_record(item: Dict[str, Any]) -> Dict[str, Any]:
    """入力データから学習用レコードに変換するサンプル関数。
    出力は JSON Lines の各行にそのまま書き込める辞書を返す。"""
    text = item.get("text") or item.get("transcript") or ""
    text = sanitize_text(str(text))
    meta = {k: v for k, v in item.items() if k != "text" and k != "transcript"}
    return {"text": text, "meta": meta}


def export_jsonl(source_file: str, output_file: str, limit: int = 0) -> int:
    count = 0
    with open(output_file, "w", encoding="utf-8") as out:
        for item in read_source_file(source_file):
            rec = to_training_record(item)
            out.write(json.dumps(rec, ensure_ascii=False) + "\n")
            count += 1
            if limit and count >= limit:
                break
    return count


def main(argv=None):
    parser = argparse.ArgumentParser()
    parser.add_argument("--source-file", required=True, help="入力 JSONL ファイル（例: data/raw_events.jsonl）")
    parser.add_argument("--output", required=True, help="出力先 JSONL ファイル")
    parser.add_argument("--limit", type=int, default=0, help="出力件数制限（0=制限なし）")
    args = parser.parse_args(argv)

    printed = export_jsonl(args.source_file, args.output, args.limit)
    print(f"exported {printed} records to {args.output}")


if __name__ == "__main__":
    main()
