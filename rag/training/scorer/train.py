"""学習の CLI 本体: 書き出しを読む → 利用者単位で分ける → 学習 → 保存 → 読み直して評価 → レポート。

使い方（rag/ で）:

    python -m training.scorer --input <書き出し1回ぶんのディレクトリ> \\
        --model-out scorer.bin --report-out scorer-report.md
"""

from __future__ import annotations

import argparse
import logging
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Sequence

from training.scorer import evaluate, model
from training.scorer.data import ExportFormatError, load_export, split_by_subject

logger = logging.getLogger(__name__)

DEFAULT_EVAL_FRACTION = 0.2


@dataclass(frozen=True)
class RunResult:
    model_bytes: int
    model_sha256: str
    metrics: list[evaluate.AxisMetrics]


def run(
    input_dir: str | Path,
    model_out: str | Path,
    report_out: str | Path,
    cfg: model.TrainConfig = model.TrainConfig(),
    eval_fraction: float = DEFAULT_EVAL_FRACTION,
) -> RunResult:
    """学習を1回行い、モデルファイルとレポートを書く。

    評価は、保存したファイルを読み直したモデル（float16 の重み）で行う。Go が読むのと同じ中身で測るため。
    """
    ds = load_export(input_dir)
    if not ds.samples:
        raise ValueError("学習に使える行がありません（書き出しが空か、すべての行が読み飛ばされました）")
    train_set, eval_set = split_by_subject(ds.samples, eval_fraction, cfg.seed)
    weights, thresholds = model.train(train_set, ds.rubric_keys, cfg)
    info = {
        "export_run_id": ds.run_id,
        "train_samples": len(train_set),
        "eval_samples": len(eval_set),
        "train_subjects": len({s.subject for s in train_set}),
        "eval_subjects": len({s.subject for s in eval_set}),
        "skipped_lines": sum(ds.skipped.values()),
        "seed": cfg.seed,
        "epochs": cfg.epochs,
        "learning_rate": cfg.learning_rate,
        "l2": cfg.l2,
        "eval_fraction": eval_fraction,
    }
    built = model.build_model(weights, thresholds, ds.rubric_keys, info)
    size = model.save(built, model_out)
    data = Path(model_out).read_bytes()
    loaded = model.load(model_out)
    metrics = evaluate.evaluate(loaded, eval_set)
    sha = model.model_sha256(data)
    report = evaluate.render_report(metrics, loaded.header, ds.skipped, size, sha)
    Path(report_out).write_text(report, encoding="utf-8", newline="\n")
    logger.info("保存: model=%s bytes=%d report=%s", model_out, size, report_out)
    return RunResult(model_bytes=size, model_sha256=sha, metrics=metrics)


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="python -m training.scorer",
        description="書き出しジョブの JSONL から軽量の面接採点モデルを学習する（LLM ラベルで学習。#1834）",
    )
    p.add_argument("--input", required=True, help="書き出し1回ぶんのディレクトリ（manifest.json があるもの）")
    p.add_argument("--model-out", required=True, help="モデルファイルの出力先")
    p.add_argument("--report-out", required=True, help="評価レポート（Markdown）の出力先")
    d = model.TrainConfig()
    p.add_argument("--seed", type=int, default=d.seed, help=f"分割の乱数の種（既定 {d.seed}）")
    p.add_argument("--epochs", type=int, default=d.epochs, help=f"勾配法の反復回数（既定 {d.epochs}）")
    p.add_argument("--learning-rate", type=float, default=d.learning_rate, help=f"学習率（既定 {d.learning_rate}）")
    p.add_argument("--l2", type=float, default=d.l2, help=f"L2 正則化の強さ（既定 {d.l2}）")
    p.add_argument(
        "--eval-fraction", type=float, default=DEFAULT_EVAL_FRACTION,
        help=f"評価用に回す利用者の割合（既定 {DEFAULT_EVAL_FRACTION}）",
    )
    return p


def main(argv: Sequence[str] | None = None) -> int:
    """CLI の入口。入力の誤りは終了コード 2、成功は 0 を返す。"""
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")
    args = build_parser().parse_args(argv)
    if args.epochs <= 0 or args.learning_rate <= 0 or args.l2 < 0:
        logger.error("--epochs と --learning-rate は正の値、--l2 は 0 以上にしてください")
        return 2
    cfg = model.TrainConfig(epochs=args.epochs, learning_rate=args.learning_rate, l2=args.l2, seed=args.seed)
    try:
        result = run(args.input, args.model_out, args.report_out, cfg, args.eval_fraction)
    except (ExportFormatError, ValueError) as exc:
        logger.error("%s", exc)
        return 2
    logger.info("完了: %d バイト sha256=%s", result.model_bytes, result.model_sha256)
    return 0


if __name__ == "__main__":
    sys.exit(main())
