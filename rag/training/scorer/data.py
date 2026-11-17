"""書き出しジョブ（Backend の scorerexport、#2168）の出力を読む。

入力は書き出し1回ぶんのディレクトリ（S3 の <prefix>/runs/<実行ID>/ を手元に落としたもの）。

    manifest.json              書き出しを終えた回にだけある。これが無い回は読まない
    subjects/<subject>.jsonl   学生1人ぶん。1行が面接1回

読む行の形は scorerexport.Sample（format_version・sample_id・subject・template_version・turns・labels・label_source）。
壊れた行は学習に入れず、件数を数えて理由ごとにログに残す（行の中身は個人の回答なのでログに書かない）。
"""

from __future__ import annotations

import json
import logging
import random
from collections import Counter
from dataclasses import dataclass, field
from pathlib import Path, PurePosixPath
from typing import Any

logger = logging.getLogger(__name__)

# EXPORT_FORMAT_VERSION は読める書き出しの形の版（scorerexport.FormatVersion）。
EXPORT_FORMAT_VERSION = 1

# LABEL_SOURCE_LLM は LLM の採点のラベル。#1834 の大橋さんの回答 5. で、この学習に限って使ってよいとされたもの。
LABEL_SOURCE_LLM = "llm"

# SCORE_MIN / SCORE_MAX はルーブリックのスコアの値域（Go の interview.RubricScoreMin / RubricScoreMax）。
# 軸の一覧はマニフェストの rubric_keys（Go の interview.RubricKeys()）から取り、ここには書かない（#795）。
SCORE_MIN = 0
SCORE_MAX = 5


class ExportFormatError(ValueError):
    """書き出しのディレクトリやマニフェストが読めない・形が違う。学習を始めない。"""


@dataclass(frozen=True)
class Sample:
    """面接1回ぶんの学習データ。"""

    sample_id: str
    subject: str
    turns: tuple[dict[str, str], ...]
    labels: dict[str, int]


@dataclass
class Dataset:
    """書き出し1回ぶんを読んだ結果。"""

    run_id: str
    rubric_keys: list[str]
    samples: list[Sample]
    # skipped は読み飛ばした行の数を理由ごとに数えたもの（レポートに出す）。
    skipped: Counter[str] = field(default_factory=Counter)


def _validate_labels(labels: Any, rubric_keys: list[str]) -> dict[str, int] | None:
    """ラベルが Go の ValidateRubricScores と同じ条件（全軸あり・未知の軸なし・整数・値域内）を満たせば返す。"""
    if not isinstance(labels, dict) or set(labels) != set(rubric_keys):
        return None
    out: dict[str, int] = {}
    for key in rubric_keys:
        v = labels[key]
        # bool は int の派生型なので、true/false を 1/0 として受け付けないよう除く。
        if isinstance(v, bool) or not isinstance(v, int) or not SCORE_MIN <= v <= SCORE_MAX:
            return None
        out[key] = v
    return out


def _validate_turns(turns: Any) -> tuple[dict[str, str], ...] | None:
    if not isinstance(turns, list) or not turns:
        return None
    out: list[dict[str, str]] = []
    for t in turns:
        if not isinstance(t, dict) or not isinstance(t.get("q", ""), str) or not isinstance(t.get("a"), str):
            return None
        out.append({"q": t.get("q", ""), "a": t["a"]})
    return tuple(out)


def parse_line(line: str, rubric_keys: list[str]) -> tuple[Sample | None, str]:
    """JSONL の1行を読む。読めなければ (None, 理由) を返す。"""
    try:
        obj = json.loads(line)
    except json.JSONDecodeError:
        return None, "invalid_json"
    if not isinstance(obj, dict):
        return None, "invalid_json"
    if obj.get("format_version") != EXPORT_FORMAT_VERSION:
        return None, "unsupported_format_version"
    if obj.get("label_source") != LABEL_SOURCE_LLM:
        # 教員の採点（teacher）は評価用で、学習に混ぜない（#1834 の 3.）。
        return None, "unexpected_label_source"
    subject = obj.get("subject")
    sample_id = obj.get("sample_id")
    if not isinstance(subject, str) or not subject or not isinstance(sample_id, str) or not sample_id:
        return None, "missing_id"
    labels = _validate_labels(obj.get("labels"), rubric_keys)
    if labels is None:
        return None, "invalid_labels"
    turns = _validate_turns(obj.get("turns"))
    if turns is None:
        return None, "invalid_turns"
    return Sample(sample_id=sample_id, subject=subject, turns=turns, labels=labels), ""


def _read_manifest(run_dir: Path) -> dict[str, Any]:
    path = run_dir / "manifest.json"
    if not path.is_file():
        raise ExportFormatError(f"{path} がありません（書き出しを終えた回のディレクトリを指定してください）")
    try:
        manifest = json.loads(path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        raise ExportFormatError(f"{path} を JSON として読めません: {exc}") from exc
    if not isinstance(manifest, dict):
        raise ExportFormatError(f"{path} の形が違います")
    if manifest.get("format_version") != EXPORT_FORMAT_VERSION:
        raise ExportFormatError(
            f"書き出しの形の版 {manifest.get('format_version')!r} には対応していません（対応: {EXPORT_FORMAT_VERSION}）"
        )
    if manifest.get("label_source") != LABEL_SOURCE_LLM:
        raise ExportFormatError(f"label_source が {LABEL_SOURCE_LLM!r} ではありません: {manifest.get('label_source')!r}")
    keys = manifest.get("rubric_keys")
    if not isinstance(keys, list) or not keys or not all(isinstance(k, str) and k for k in keys) or len(set(keys)) != len(keys):
        raise ExportFormatError("rubric_keys が空か、形が違います")
    files = manifest.get("files")
    if not isinstance(files, list) or not all(isinstance(f, str) for f in files):
        raise ExportFormatError("files の形が違います")
    # run_id はモデルのヘッダとレポートに書き、モデルを元の書き出しまでたどる唯一の手がかりなので、
    # 欠けた・空・文字列でないマニフェストは読まない（#2326 のレビュー）。
    run_id = manifest.get("run_id")
    if not isinstance(run_id, str) or not run_id.strip():
        raise ExportFormatError("run_id が無いか、空でない文字列ではありません")
    return manifest


def _resolve_file(run_dir: Path, name: str) -> Path:
    """マニフェストの files の1つをパスにする。ディレクトリの外を指すもの（../ や絶対パス）は拒む。"""
    rel = PurePosixPath(name)
    if rel.is_absolute() or ".." in rel.parts or not rel.parts:
        raise ExportFormatError(f"files に不正なパスがあります: {name!r}")
    return run_dir.joinpath(*rel.parts)


def load_export(run_dir: str | Path) -> Dataset:
    """書き出し1回ぶんのディレクトリを読む。

    マニフェストの files に載っているファイルだけを読む（途中で止まった回の残りや、無関係なファイルを混ぜないため）。
    載っているのに無いファイルは、書き出しが壊れているのでエラーにする。
    """
    run_dir = Path(run_dir)
    manifest = _read_manifest(run_dir)
    rubric_keys: list[str] = list(manifest["rubric_keys"])
    ds = Dataset(run_id=manifest["run_id"], rubric_keys=rubric_keys, samples=[])
    seen_ids: set[str] = set()
    for name in manifest["files"]:
        path = _resolve_file(run_dir, name)
        if not path.is_file():
            raise ExportFormatError(f"マニフェストにある {name} がありません")
        # バイトのまま1行ずつ読み、行ごとに厳密に UTF-8 として解釈する。不正なバイトを U+FFFD に置き換えて
        # 読むと、改変された回答や、壊れた subject（同じ人が別人として学習用と評価用に分かれる）を
        # 正しい行として学習してしまうため、壊れた行として数える（#2326 のレビュー）。
        with path.open("rb") as fh:
            for lineno, raw in enumerate(fh, start=1):
                try:
                    line = raw.decode("utf-8")
                except UnicodeDecodeError:
                    line, sample, reason = "", None, "invalid_utf8"
                else:
                    if not line.strip():
                        continue
                    sample, reason = parse_line(line, rubric_keys)
                if sample is not None and sample.sample_id in seen_ids:
                    sample, reason = None, "duplicate_sample_id"
                if sample is None:
                    ds.skipped[reason] += 1
                    logger.warning("行を読み飛ばしました: file=%s line=%d reason=%s", name, lineno, reason)
                    continue
                seen_ids.add(sample.sample_id)
                ds.samples.append(sample)
    logger.info("読み込み: run_id=%s samples=%d skipped=%d", ds.run_id, len(ds.samples), sum(ds.skipped.values()))
    return ds


def split_by_subject(
    samples: list[Sample], eval_fraction: float, seed: int
) -> tuple[list[Sample], list[Sample]]:
    """利用者（subject）単位で学習用と評価用に分ける。同じ人の面接は必ず同じ側に入る（#1834 の 3.）。

    subject を整列してから、種を固定した乱数で並べ替え、先頭の round(人数 × eval_fraction) 人
    （少なくとも1人、多くとも 人数-1 人）を評価用にする。入力の並び順によらず、同じ種なら同じ分け方になる。
    """
    if not 0 < eval_fraction < 1:
        raise ValueError("eval_fraction は 0 より大きく 1 より小さくしてください")
    subjects = sorted({s.subject for s in samples})
    if len(subjects) < 2:
        raise ValueError(f"利用者単位で分けるには2人以上のデータが必要です（{len(subjects)}人）")
    random.Random(seed).shuffle(subjects)
    n_eval = min(max(1, round(len(subjects) * eval_fraction)), len(subjects) - 1)
    eval_subjects = set(subjects[:n_eval])
    train = [s for s in samples if s.subject not in eval_subjects]
    evaluation = [s for s in samples if s.subject in eval_subjects]
    return train, evaluation
