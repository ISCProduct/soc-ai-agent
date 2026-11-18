"""評価用の分割での軸ごとの MAE と QWK、および評価レポート。

ここで測るのは「LLM ラベルとの一致」で、教員が採点したセットでの精度ではない（#1834 の大橋さんの回答 5.）。
実運用の精度は #2172 で、教員の評価セットを使って測る。
"""

from __future__ import annotations

from collections import Counter
from dataclasses import dataclass
from typing import Any, Sequence

import numpy as np

from training.scorer.data import SCORE_MAX, SCORE_MIN, Sample
from training.scorer.model import ScorerModel

# レポートの先頭に必ず置く注意書き。#1834 の条件「LLM ラベルで学習と明記する」「LLM ラベルとの一致では精度を測らない」のため。
REPORT_NOTICE_LINES = (
    "> **LLM ラベルで学習**したモデルです（labels は LLM の採点で、教員の採点ではありません）。",
    "> 下の数値は **LLM ラベルとの一致であり、教員の評価セットでの精度ではありません**。"
    "実運用の精度は、教員が採点した評価セットで測ります（#2172）。",
)


def mae(y_true: Sequence[int], y_pred: Sequence[int]) -> float:
    """平均絶対誤差。"""
    if len(y_true) == 0:
        raise ValueError("評価データが空です")
    return float(np.mean(np.abs(np.asarray(y_true) - np.asarray(y_pred))))


def qwk(y_true: Sequence[int], y_pred: Sequence[int], lo: int = SCORE_MIN, hi: int = SCORE_MAX) -> float | None:
    """二次の重み付きカッパ（QWK）。

    期待される不一致が 0（正解と予測の両方が1つの値だけ）のときは定義できないので None を返す
    （1.0 や 0.0 にすると、評価データが偏っているだけなのに一致・不一致と誤読されるため）。
    """
    if len(y_true) == 0:
        raise ValueError("評価データが空です")
    k = hi - lo + 1
    observed = np.zeros((k, k))
    for t, p in zip(y_true, y_pred):
        observed[t - lo, p - lo] += 1
    i, j = np.meshgrid(np.arange(k), np.arange(k), indexing="ij")
    weights = (i - j) ** 2 / (k - 1) ** 2
    expected = np.outer(observed.sum(axis=1), observed.sum(axis=0)) / observed.sum()
    denom = float((weights * expected).sum())
    if denom == 0:
        return None
    return 1.0 - float((weights * observed).sum()) / denom


@dataclass(frozen=True)
class AxisMetrics:
    key: str
    mae: float
    qwk: float | None


def evaluate(model: ScorerModel, samples: Sequence[Sample]) -> list[AxisMetrics]:
    """評価用のサンプルで、軸ごとの MAE と QWK を出す。保存するのと同じ float16 の重みで予測する。"""
    if not samples:
        raise ValueError("評価データが空です")
    preds = [model.predict(s.turns) for s in samples]
    out = []
    for key in model.rubric_keys:
        y = [s.labels[key] for s in samples]
        p = [pr[key] for pr in preds]
        out.append(AxisMetrics(key=key, mae=mae(y, p), qwk=qwk(y, p)))
    return out


def render_report(
    metrics: Sequence[AxisMetrics],
    header: dict[str, Any],
    skipped: Counter[str],
    model_bytes: int,
    model_sha256: str,
) -> str:
    """評価レポート（Markdown）を作る。先頭に REPORT_NOTICE_LINES を置く。"""
    training = header["training"]
    lines = ["# 軽量採点モデルの学習レポート", "", *REPORT_NOTICE_LINES, ""]
    lines += [
        "## モデル",
        f"- 形式: {header['format']} v{header['format_version']}",
        f"- ルーブリックの版: {header['rubric']['version']}（軸: {', '.join(header['rubric']['keys'])}）",
        f"- ラベルの出どころ: {header['label_source']}",
        f"- ファイルの大きさ: {model_bytes} バイト",
        f"- ファイルの SHA-256: {model_sha256}",
        f"- 重みの SHA-256: {header['weights']['sha256']}",
        "",
        "## データ",
        f"- 書き出しの実行 ID: {training['export_run_id']}",
        f"- 学習: {training['train_samples']} 件（{training['train_subjects']} 人）",
        f"- 評価: {training['eval_samples']} 件（{training['eval_subjects']} 人）。利用者単位で分け、同じ人は学習と評価の両方に入れていません",
        f"- 乱数の種: {training['seed']}",
    ]
    if skipped:
        detail = "、".join(f"{reason} {n} 件" for reason, n in sorted(skipped.items()))
        lines.append(f"- 読み飛ばした行: {sum(skipped.values())} 件（{detail}）")
    else:
        lines.append("- 読み飛ばした行: 0 件")
    lines += [
        "",
        "## 評価用の分割での LLM ラベルとの一致",
        "",
        "| 軸 | MAE | QWK |",
        "| --- | --- | --- |",
    ]
    for m in metrics:
        q = "N/A（評価データのラベルと予測が1種類だけで定義できない）" if m.qwk is None else f"{m.qwk:.3f}"
        lines.append(f"| {m.key} | {m.mae:.3f} | {q} |")
    lines.append("")
    return "\n".join(lines)
