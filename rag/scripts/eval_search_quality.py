"""検索（学生検索・企業リサーチ）の品質を固定セットで測る（#1773）。

例:
  # CI と同じ（費用なし）。検索経路のコードを変えたときの回帰確認
  python scripts/eval_search_quality.py

  # 本番と同じ埋め込みと本物の Chroma（メモリ上）で測る。OPENAI_API_KEY が必要で費用がかかる
  python scripts/eval_search_quality.py --embedder openai --store chroma --output /tmp/before.json
  # 設定（OPENAI_EMBEDDING_MODEL など）を変えてもう一度測り、前の結果と比べる
  OPENAI_EMBEDDING_MODEL=text-embedding-3-large \
    python scripts/eval_search_quality.py --embedder openai --store chroma --baseline /tmp/before.json

基準値より下がった指標があれば終了コード 1 を返す。
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path
from typing import Any, Dict

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from evaluation.search_quality.dataset import DATA_DIR, default_dataset_path, load_dataset  # noqa: E402
from evaluation.search_quality.embedders import get_embedder  # noqa: E402
from evaluation.search_quality.memory_store import MemoryClient  # noqa: E402
from evaluation.search_quality.runner import (  # noqa: E402
    INVARIANTS,
    compare_to_baseline,
    evaluate_company_research,
    evaluate_student_search,
)

HASH_BASELINE = DATA_DIR / "baseline_hash_memory.json"


def new_client(store: str) -> Any:
    if store == "memory":
        return MemoryClient()
    if store == "chroma":
        import chromadb

        return chromadb.EphemeralClient()
    raise ValueError(f"unknown store: {store}")


def run(embedder: str, store: str) -> Dict[str, Any]:
    embed = get_embedder(embedder)
    return {
        # 学生検索と企業リサーチは別のクライアントで測り、互いのコレクションを混ぜない
        "student_search": evaluate_student_search(
            load_dataset(default_dataset_path("student_search")), embed, new_client(store)
        ),
        "company_research": evaluate_company_research(
            load_dataset(default_dataset_path("company_research")), embed, new_client(store)
        ),
    }


def to_markdown(report: Dict[str, Any]) -> str:
    lines = []
    for target, summary in report.items():
        lines.append(f"### {target}（文書 {summary['documents']} 件・クエリ {summary['queries']} 件）")
        lines.append("| 指標 | 値 |")
        lines.append("|---|---|")
        for metric, value in summary["mean"].items():
            lines.append(f"| {metric} | {value:.4f} |")
        for key, label in INVARIANTS.items():
            if key in summary:
                lines.append(f"| {label}件数（0 でなければ失敗） | {summary[key]} |")
        lines.append("")
    return "\n".join(lines)


def strip_details(report: Dict[str, Any]) -> Dict[str, Any]:
    """基準値には平均と件数だけを残す（クエリごとの順位は差分が読みにくくなるので残さない）。"""
    return {
        target: {k: v for k, v in summary.items() if k != "per_query"} for target, summary in report.items()
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--embedder", choices=["hash", "openai"], default="hash")
    parser.add_argument("--store", choices=["memory", "chroma"], default="memory")
    parser.add_argument("--baseline", type=Path, help="比べる基準値の JSON。省略時は hash/memory のときだけ同梱の基準値")
    parser.add_argument("--tolerance", type=float, default=0.0, help="許容する下げ幅")
    parser.add_argument("--output", type=Path, help="結果（クエリごとの順位を含む）を書き出す JSON")
    parser.add_argument("--update-baseline", action="store_true", help="同梱の基準値（hash/memory）を書き換える")
    args = parser.parse_args()

    report = run(args.embedder, args.store)
    print(to_markdown(report))
    if args.output:
        args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    if args.update_baseline:
        if (args.embedder, args.store) != ("hash", "memory"):
            parser.error("同梱の基準値は --embedder hash --store memory の結果だけを置く")
        HASH_BASELINE.write_text(json.dumps(strip_details(report), ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print(f"基準値を更新しました: {HASH_BASELINE}")
        return 0

    baseline_path = args.baseline
    if baseline_path is None and (args.embedder, args.store) == ("hash", "memory"):
        baseline_path = HASH_BASELINE
    if baseline_path is None:
        return 0
    baseline = json.loads(baseline_path.read_text(encoding="utf-8"))
    regressions = compare_to_baseline(report, baseline, tolerance=args.tolerance)
    if regressions:
        print("基準値より下がった指標があります:")
        for line in regressions:
            print(f"- {line}")
        return 1
    print(f"基準値（{baseline_path}）以上です")
    return 0


if __name__ == "__main__":
    sys.exit(main())
