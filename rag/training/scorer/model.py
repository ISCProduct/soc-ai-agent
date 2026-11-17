"""軸ごとの線形の順序回帰（all-threshold のロジスティック）の学習と、モデルファイルの保存・読み込み。

モデルファイルの形（Go の推論 #2171 が読む。詳しくは README.md の「モデルファイルの形式」）:

    先頭 8 バイト   MAGIC（b"SOCSCORE"）
    4 バイト        ヘッダの長さ（uint32、リトルエンディアン）
    ヘッダ          JSON（UTF-8）。形式の版・ルーブリック・特徴量の設定・閾値・件数・重みのチェックサムなど
    重み            float16 のリトルエンディアン。軸の順（rubric.keys）に、各軸 HASH_DIM 個を添字の順に並べる
"""

from __future__ import annotations

import hashlib
import json
import logging
import struct
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Sequence

import numpy as np

from training.scorer import features
from training.scorer.data import LABEL_SOURCE_LLM, SCORE_MAX, SCORE_MIN, Sample

logger = logging.getLogger(__name__)

MAGIC = b"SOCSCORE"
FORMAT_NAME = "soc-interview-scorer"
# MODEL_FORMAT_VERSION はファイルの形の版。形を変えたら上げる（Go は知らない版を読まない）。
MODEL_FORMAT_VERSION = 1
# MAX_MODEL_BYTES はモデルファイルの大きさの上限（非機能要件の目安 2.7MB。#2170）。
# MB を 1000 × 1000 バイトとして数える（MiB より厳しい側に倒す）。
MAX_MODEL_BYTES = 2_700_000
# MAX_HEADER_BYTES はヘッダの長さの上限。壊れたファイルの長さの欄を信じて大きく読まないため。
MAX_HEADER_BYTES = 1 << 20
_FLOAT16_MAX = float(np.finfo(np.float16).max)


class ModelFormatError(ValueError):
    """モデルファイルが壊れている・形が違う・チェックサムが合わない。"""


@dataclass(frozen=True)
class TrainConfig:
    """学習の設定。

    既定値は、テストの人工データ（目印の語でラベルが決まるもの）で、評価用の分割の MAE が下がることを
    確かめた値。実データでの調整は、教員の評価セットができてから行う（#2172）。
    """

    epochs: int = 300
    learning_rate: float = 0.2
    l2: float = 1e-5
    seed: int = 20261117


@dataclass
class ScorerModel:
    """学習済みのモデル。weights は (軸の数, HASH_DIM) の float16、thresholds は (軸の数, 閾値の数) の float64。"""

    rubric_keys: list[str]
    weights: np.ndarray
    thresholds: np.ndarray
    header: dict[str, Any]

    def decision(self, feats: features.SparseFeatures) -> np.ndarray:
        """軸ごとの w·x を返す。重みは float16 を float64 に広げて計算する（Go と同じ）。"""
        if not feats.indices:
            return np.zeros(len(self.rubric_keys))
        idx = np.asarray(feats.indices, dtype=np.int64)
        val = np.asarray(feats.values, dtype=np.float64)
        return self.weights[:, idx].astype(np.float64) @ val

    def predict(self, turns: Sequence[dict[str, str]]) -> dict[str, int]:
        """軸ごとのスコア。w·x が閾値を上回った数（SCORE_MIN からの段数）。"""
        z = self.decision(features.extract(turns))
        return {
            key: SCORE_MIN + int(np.sum(z[a] > self.thresholds[a]))
            for a, key in enumerate(self.rubric_keys)
        }

    def expected(self, turns: Sequence[dict[str, str]]) -> dict[str, float]:
        """軸ごとのスコアの期待値（SCORE_MIN + Σ σ(w·x − θ_k)）。確信度の目安に使う。"""
        z = self.decision(features.extract(turns))
        return {
            key: SCORE_MIN + float(np.sum(_sigmoid(z[a] - self.thresholds[a])))
            for a, key in enumerate(self.rubric_keys)
        }


def _sigmoid(x: np.ndarray) -> np.ndarray:
    # exp のあふれを避けるため、大きな値は切る（±35 で σ は 0・1 と区別できなくなる）。
    return 1.0 / (1.0 + np.exp(-np.clip(x, -35.0, 35.0)))


def rubric_version(rubric_keys: Sequence[str]) -> str:
    """ルーブリックの版。Go 側に版の定数が無いため、軸の並びと値域から決まる指紋にする。

    「logic,specificity,...|0-5」の UTF-8 の SHA-256 の先頭16桁。軸の増減・並び替え・値域の変更で変わる。
    Go は strings.Join(interview.RubricKeys(), ",") + "|0-5" で同じ値を作り、合わなければモデルを使わない。
    """
    canonical = ",".join(rubric_keys) + f"|{SCORE_MIN}-{SCORE_MAX}"
    return "sha256:" + hashlib.sha256(canonical.encode("utf-8")).hexdigest()[:16]


@dataclass
class _Matrix:
    """疎行列（COO）。scipy に依存しないよう、numpy の配列だけで持つ。"""

    rows: np.ndarray
    cols: np.ndarray
    vals: np.ndarray
    n_rows: int


def _build_matrix(samples: Sequence[Sample]) -> _Matrix:
    rows: list[int] = []
    cols: list[int] = []
    vals: list[float] = []
    for i, s in enumerate(samples):
        f = features.extract(s.turns)
        rows.extend([i] * len(f.indices))
        cols.extend(f.indices)
        vals.extend(f.values)
    return _Matrix(
        rows=np.asarray(rows, dtype=np.int64),
        cols=np.asarray(cols, dtype=np.int64),
        vals=np.asarray(vals, dtype=np.float64),
        n_rows=len(samples),
    )


def _matvec(m: _Matrix, w: np.ndarray) -> np.ndarray:
    return np.bincount(m.rows, weights=m.vals * w[m.cols], minlength=m.n_rows)


def _rmatvec(m: _Matrix, g: np.ndarray) -> np.ndarray:
    return np.bincount(m.cols, weights=m.vals * g[m.rows], minlength=features.HASH_DIM)


def _fit_axis(m: _Matrix, y: np.ndarray, cfg: TrainConfig) -> tuple[np.ndarray, np.ndarray]:
    """1軸ぶんを学習する。損失は all-threshold のロジスティック（各閾値 k で「y > k か」の2値の対数損失の和）
    の平均に、重みの L2 正則化を足したもの。全件の勾配で Adam をかける（乱数を使わないので、結果は入力で決まる）。
    """
    ks = np.arange(SCORE_MIN, SCORE_MAX)
    target = (y[:, None] > ks[None, :]).astype(np.float64)  # (N, K)
    # 閾値は、ラベルの分布から決まる初期値（重み 0 のときの最適値）から始める。
    rate = np.clip(target.mean(axis=0), 1e-3, 1 - 1e-3)
    theta = -np.log(rate / (1 - rate))
    w = np.zeros(features.HASH_DIM)
    params = [w, theta]
    m1 = [np.zeros_like(p) for p in params]
    m2 = [np.zeros_like(p) for p in params]
    beta1, beta2, eps = 0.9, 0.999, 1e-8
    n = max(m.n_rows, 1)
    for step in range(1, cfg.epochs + 1):
        z = _matvec(m, w)
        g = (_sigmoid(z[:, None] - theta[None, :]) - target) / n  # 損失の u = z - θ についての勾配
        grads = [_rmatvec(m, g.sum(axis=1)) + cfg.l2 * w, -g.sum(axis=0)]
        for i, (p, gr) in enumerate(zip(params, grads)):
            m1[i] = beta1 * m1[i] + (1 - beta1) * gr
            m2[i] = beta2 * m2[i] + (1 - beta2) * gr * gr
            mhat = m1[i] / (1 - beta1**step)
            vhat = m2[i] / (1 - beta2**step)
            p -= cfg.learning_rate * mhat / (np.sqrt(vhat) + eps)
    return w, theta


def train(samples: Sequence[Sample], rubric_keys: Sequence[str], cfg: TrainConfig) -> tuple[np.ndarray, np.ndarray]:
    """全軸を学習し、(重み (軸, HASH_DIM) の float64, 閾値 (軸, 閾値の数)) を返す。"""
    if not samples:
        raise ValueError("学習データが空です")
    m = _build_matrix(samples)
    ws, thetas = [], []
    for key in rubric_keys:
        y = np.asarray([s.labels[key] for s in samples], dtype=np.int64)
        w, theta = _fit_axis(m, y, cfg)
        ws.append(w)
        thetas.append(theta)
        logger.info("学習: axis=%s samples=%d", key, len(samples))
    return np.vstack(ws), np.vstack(thetas)


def quantize(weights: np.ndarray) -> np.ndarray:
    """重みを float16 にする。float16 で表せない大きさの重みは、黙って inf にせずエラーにする。"""
    if not np.all(np.isfinite(weights)) or np.max(np.abs(weights), initial=0.0) > _FLOAT16_MAX:
        raise ValueError("重みが float16 で表せる範囲を超えています（学習率や正則化を見直してください）")
    return weights.astype("<f2")


def build_model(
    weights: np.ndarray,
    thresholds: np.ndarray,
    rubric_keys: Sequence[str],
    training_info: dict[str, Any],
) -> ScorerModel:
    """学習結果から、保存するのと同じ内容のモデル（重みは float16）を作る。"""
    w16 = quantize(weights)
    if not np.all(np.isfinite(thresholds)):
        raise ValueError("閾値に有限でない値があります")
    blob = w16.tobytes(order="C")
    header: dict[str, Any] = {
        "format": FORMAT_NAME,
        "format_version": MODEL_FORMAT_VERSION,
        "rubric": {
            "version": rubric_version(rubric_keys),
            "keys": list(rubric_keys),
            "score_min": SCORE_MIN,
            "score_max": SCORE_MAX,
        },
        "features": features.spec(),
        "model": {
            "type": "linear_ordinal_all_threshold",
            "predict": "score = score_min + count(w·x > thresholds[k])",
        },
        "thresholds": {key: [float(t) for t in thresholds[a]] for a, key in enumerate(rubric_keys)},
        "weights": {
            "dtype": "float16",
            "byte_order": "little",
            "layout": "axis_major",
            "shape": [len(rubric_keys), features.HASH_DIM],
            "byte_length": len(blob),
            "sha256": hashlib.sha256(blob).hexdigest(),
        },
        "label_source": LABEL_SOURCE_LLM,
        "training": training_info,
    }
    return ScorerModel(
        rubric_keys=list(rubric_keys),
        weights=w16,
        thresholds=np.asarray(thresholds, dtype=np.float64),
        header=header,
    )


def encode(model: ScorerModel) -> bytes:
    """モデルをファイルの中身にする。"""
    head = json.dumps(model.header, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")
    return MAGIC + struct.pack("<I", len(head)) + head + model.weights.astype("<f2").tobytes(order="C")


def save(model: ScorerModel, path: str | Path) -> int:
    """モデルを保存し、ファイルの大きさを返す。MAX_MODEL_BYTES を超えるなら書かずにエラーにする。"""
    data = encode(model)
    if len(data) > MAX_MODEL_BYTES:
        raise ValueError(f"モデルファイルが {len(data)} バイトで、上限 {MAX_MODEL_BYTES} バイトを超えます")
    Path(path).write_bytes(data)
    return len(data)


def decode(data: bytes) -> ScorerModel:
    """ファイルの中身からモデルを読む。形が違う・チェックサムが合わないときは ModelFormatError。"""
    if len(data) < len(MAGIC) + 4 or data[: len(MAGIC)] != MAGIC:
        raise ModelFormatError("採点モデルのファイルではありません（先頭のバイト列が違います）")
    (head_len,) = struct.unpack_from("<I", data, len(MAGIC))
    start = len(MAGIC) + 4
    if head_len > MAX_HEADER_BYTES or start + head_len > len(data):
        raise ModelFormatError("ヘッダの長さが不正です")
    try:
        header = json.loads(data[start : start + head_len].decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ModelFormatError(f"ヘッダを JSON として読めません: {exc}") from exc
    if not isinstance(header, dict) or header.get("format") != FORMAT_NAME:
        raise ModelFormatError("ヘッダの format が違います")
    if header.get("format_version") != MODEL_FORMAT_VERSION:
        raise ModelFormatError(f"モデルの形の版 {header.get('format_version')!r} には対応していません")
    try:
        rubric = header["rubric"]
        keys = list(rubric["keys"])
        wmeta = header["weights"]
        raw_thresholds = [list(header["thresholds"][k]) for k in keys]
        expected_len = int(wmeta["byte_length"])
        expected_sha = str(wmeta["sha256"])
        dim = int(header["features"]["dim"])
    except (KeyError, TypeError, ValueError) as exc:
        raise ModelFormatError(f"ヘッダに必要な項目がありません: {exc}") from exc
    if not keys or (rubric.get("score_min"), rubric.get("score_max")) != (SCORE_MIN, SCORE_MAX):
        raise ModelFormatError("ルーブリックの軸が空か、値域がこの実装と一致しません")
    if any(len(t) != SCORE_MAX - SCORE_MIN for t in raw_thresholds):
        raise ModelFormatError("閾値の数が不正です")
    try:
        thresholds = np.asarray(raw_thresholds, dtype=np.float64)
    except (TypeError, ValueError) as exc:
        raise ModelFormatError(f"閾値の値が不正です: {exc}") from exc
    if header["features"] != features.spec():
        # 特徴量の手順が違うモデルは、同じ入力から別の特徴量を作るので使えない。
        raise ModelFormatError("特徴量の設定が、この実装と一致しません")
    if wmeta.get("dtype") != "float16" or wmeta.get("byte_order") != "little" or wmeta.get("layout") != "axis_major":
        raise ModelFormatError("重みの形式に対応していません")
    blob = data[start + head_len :]
    if expected_len != len(keys) * dim * 2 or len(blob) != expected_len:
        raise ModelFormatError(f"重みの長さが合いません（ヘッダ {expected_len}、実際 {len(blob)}）")
    if hashlib.sha256(blob).hexdigest() != expected_sha:
        raise ModelFormatError("重みのチェックサム（SHA-256）が合いません")
    if not np.all(np.isfinite(thresholds)):
        raise ModelFormatError("閾値の値が不正です")
    weights = np.frombuffer(blob, dtype="<f2").reshape(len(keys), dim).copy()
    if not np.all(np.isfinite(weights)):
        raise ModelFormatError("重みに有限でない値があります")
    return ScorerModel(rubric_keys=keys, weights=weights, thresholds=thresholds, header=header)


def load(path: str | Path) -> ScorerModel:
    """モデルファイルを読む。"""
    return decode(Path(path).read_bytes())


def model_sha256(data: bytes) -> str:
    """ファイル全体の SHA-256（レポートに書き、配った版の取り違えを見分けるのに使う）。"""
    return hashlib.sha256(data).hexdigest()
