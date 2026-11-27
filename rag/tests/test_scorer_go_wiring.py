"""人工データで、書き出し → 学習 → Go の推論まで通るかを確認する（SOCAIAGENT-620）。

精度は見ない。結果は「人工データ」の配線確認である。
Go の NFKC は Unicode 15.0.0 なので、Python 3.12 以外では飛ばす。
"""

import json
import os
import shutil
import subprocess
import sys
import unicodedata
from pathlib import Path

import pytest

from training.scorer import model, train
from tests.test_scorer_training import RUBRIC_KEYS, _synthetic_export


def test_export_train_go_score(tmp_path: Path) -> None:
    if unicodedata.unidata_version != "15.0.0":
        pytest.skip("Go の Unicode 15.0.0 と揃う Python 3.12 で実行する")
    if shutil.which("go") is None:
        pytest.skip("go が無い")

    run_dir = _synthetic_export(tmp_path / "export", n_subjects=3, per_subject=2, seed=1)
    model_out = tmp_path / "scorer.bin"
    report_out = tmp_path / "report.md"
    train.run(run_dir, model_out, report_out, model.TrainConfig(epochs=5))

    result_path = tmp_path / "go-score.json"
    repo = Path(__file__).resolve().parents[2]
    env = os.environ.copy()
    env["SCORER_MODEL_PATH"] = str(model_out)
    env["SCORER_RESULT_PATH"] = str(result_path)
    proc = subprocess.run(
        ["go", "test", "./internal/services/interview/shadowscorer", "-count=1", "-run", "TestScoreTrainedModelFile"],
        cwd=repo / "Backend",
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.returncode != 0:
        sys.stderr.write(proc.stdout + "\n" + proc.stderr)
        raise AssertionError("Go の推論が失敗した")

    body = json.loads(result_path.read_text(encoding="utf-8"))
    assert body["note"].startswith("人工データ")
    assert body["model_version"]
    scores = body["scores"]
    assert set(scores) == set(RUBRIC_KEYS)
    for key in RUBRIC_KEYS:
        assert 0 <= scores[key]["score"] <= 5
