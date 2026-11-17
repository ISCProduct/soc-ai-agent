"""軽量採点モデルの学習（#2170）のテスト。

データはすべてテスト用に作った架空のもの（実在の人・学校・企業の名前は使わない）。
"""

import json
import random
from collections import Counter
from pathlib import Path

import numpy as np
import pytest

from training.scorer import data, evaluate, features, model, train

RUBRIC_KEYS = ["logic", "specificity", "ownership", "communication", "enthusiasm"]


def _labels(**override):
    labels = {k: 3 for k in RUBRIC_KEYS}
    labels.update(override)
    return labels


def _line(sample_id="s-1", subject="subj-a", labels=None, turns=None, **override) -> dict:
    obj = {
        "format_version": 1,
        "sample_id": sample_id,
        "subject": subject,
        "template_version": "v1",
        "turns": turns if turns is not None else [{"q": "自己紹介をお願いします。", "a": "サークルの代表をしていました。"}],
        "labels": labels if labels is not None else _labels(),
        "label_source": "llm",
    }
    obj.update(override)
    return obj


def _synthetic_answer(labels: dict, rng: random.Random) -> str:
    """ラベルが学習できるよう、軸と点数ごとの目印の語（例：logic3）を回答に入れる。"""
    filler = rng.choice(["私は架空大学で", "アルバイトでは", "ゼミの活動で", "部活動では"])
    marks = " ".join(f"{k}{v}" for k, v in labels.items())
    return f"{filler}取り組みました。{marks}"


def _write_export(run_dir: Path, lines_by_subject: dict, manifest_override=None, raw_lines=None) -> Path:
    """書き出しジョブと同じ形（manifest.json と subjects/<subject>.jsonl）でディレクトリを作る。"""
    (run_dir / "subjects").mkdir(parents=True, exist_ok=True)
    files = []
    for subject, lines in lines_by_subject.items():
        name = f"subjects/{subject}.jsonl"
        body = "".join(json.dumps(obj, ensure_ascii=False) + "\n" for obj in lines)
        body += "".join((raw_lines or {}).get(subject, []))
        (run_dir / name).write_text(body, encoding="utf-8")
        files.append(name)
    manifest = {
        "format_version": 1,
        "run_id": "20261117T000000Z",
        "label_source": "llm",
        "rubric_keys": RUBRIC_KEYS,
        "files": files,
    }
    manifest.update(manifest_override or {})
    (run_dir / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False), encoding="utf-8")
    return run_dir


def _synthetic_export(run_dir: Path, n_subjects=10, per_subject=6, seed=1) -> Path:
    rng = random.Random(seed)
    by_subject = {}
    for si in range(n_subjects):
        subject = f"subj-{si:02d}"
        lines = []
        for j in range(per_subject):
            labels = {k: rng.randint(0, 5) for k in RUBRIC_KEYS}
            turns = [{"q": "学生時代に力を入れたことは？", "a": _synthetic_answer(labels, rng)}]
            lines.append(_line(sample_id=f"{subject}-{j}", subject=subject, labels=labels, turns=turns))
        by_subject[subject] = lines
    return _write_export(run_dir, by_subject)


FAST = model.TrainConfig()


# ── 1行の読み込み ─────────────────────────────────────────────


@pytest.mark.parametrize("value", [0, 5])
def test_parse_line_accepts_boundary_scores(value):
    sample, reason = data.parse_line(json.dumps(_line(labels=_labels(logic=value))), RUBRIC_KEYS)
    assert reason == ""
    assert sample is not None
    assert sample.labels["logic"] == value
    assert sample.subject == "subj-a" and sample.sample_id == "s-1"
    assert sample.turns[0]["a"] == "サークルの代表をしていました。"


def test_parse_line_accepts_empty_question():
    """最初の質問より前の学生の発話は、質問が空で書き出される（scorerexport.buildTurns）。"""
    sample, reason = data.parse_line(json.dumps(_line(turns=[{"q": "", "a": "よろしくお願いします"}])), RUBRIC_KEYS)
    assert reason == "" and sample is not None


@pytest.mark.parametrize(
    "line,reason",
    [
        ("{not json", "invalid_json"),
        ("[1, 2]", "invalid_json"),
        (json.dumps(_line(format_version=2)), "unsupported_format_version"),
        (json.dumps(_line(label_source="teacher")), "unexpected_label_source"),
        (json.dumps(_line(subject="")), "missing_id"),
        (json.dumps(_line(sample_id=None)), "missing_id"),
        (json.dumps(_line(labels=_labels(logic=-1))), "invalid_labels"),
        (json.dumps(_line(labels=_labels(logic=6))), "invalid_labels"),
        (json.dumps(_line(labels=_labels(logic=3.5))), "invalid_labels"),
        (json.dumps(_line(labels=_labels(logic=True))), "invalid_labels"),
        (json.dumps(_line(labels=_labels(logic="3"))), "invalid_labels"),
        (json.dumps(_line(labels={k: 3 for k in RUBRIC_KEYS[:-1]})), "invalid_labels"),
        (json.dumps(_line(labels=_labels(extra=3))), "invalid_labels"),
        (json.dumps(_line(labels=[3, 3, 3, 3, 3])), "invalid_labels"),
        (json.dumps(_line(turns=[])), "invalid_turns"),
        (json.dumps(_line(turns=[{"q": "質問"}])), "invalid_turns"),
        (json.dumps(_line(turns=[{"q": 1, "a": "回答"}])), "invalid_turns"),
    ],
    ids=[
        "not_json", "not_object", "format_version", "teacher_label", "empty_subject", "missing_sample_id",
        "label_below_min", "label_above_max", "label_float", "label_bool", "label_string", "label_missing_axis",
        "label_unknown_axis", "labels_not_object", "empty_turns", "turn_without_answer", "question_not_string",
    ],
)
def test_parse_line_rejects_broken_lines(line, reason):
    sample, got = data.parse_line(line, RUBRIC_KEYS)
    assert sample is None
    assert got == reason


# ── 書き出しのディレクトリの読み込み ──────────────────────────────


def test_load_export_skips_broken_lines_and_counts_reasons(tmp_path):
    lines = {"subj-a": [_line("s-1", "subj-a"), _line("s-2", "subj-a", labels=_labels(logic=9)), _line("s-1", "subj-a")]}
    raw = {"subj-a": ["{broken\n", "\n"]}
    ds = data.load_export(_write_export(tmp_path, lines, raw_lines=raw))
    assert [s.sample_id for s in ds.samples] == ["s-1"]
    assert ds.skipped == Counter({"invalid_labels": 1, "duplicate_sample_id": 1, "invalid_json": 1})
    assert ds.rubric_keys == RUBRIC_KEYS
    assert ds.run_id == "20261117T000000Z"


def test_load_export_reads_only_files_in_manifest(tmp_path):
    run_dir = _write_export(tmp_path, {"subj-a": [_line("s-1", "subj-a")]})
    (run_dir / "subjects" / "stray.jsonl").write_text(json.dumps(_line("s-9", "subj-z")) + "\n", encoding="utf-8")
    ds = data.load_export(run_dir)
    assert [s.sample_id for s in ds.samples] == ["s-1"]


def test_load_export_empty_files_list_returns_no_samples(tmp_path):
    ds = data.load_export(_write_export(tmp_path, {}))
    assert ds.samples == [] and not ds.skipped


def test_load_export_requires_manifest(tmp_path):
    with pytest.raises(data.ExportFormatError, match="manifest.json"):
        data.load_export(tmp_path)


@pytest.mark.parametrize(
    "override,match",
    [
        ({"format_version": 2}, "版"),
        ({"label_source": "teacher"}, "label_source"),
        ({"rubric_keys": []}, "rubric_keys"),
        ({"rubric_keys": ["logic", "logic"]}, "rubric_keys"),
        ({"files": "subjects/a.jsonl"}, "files"),
        ({"files": ["../outside.jsonl"]}, "不正なパス"),
        ({"files": ["/etc/passwd"]}, "不正なパス"),
        ({"files": ["subjects/missing.jsonl"]}, "ありません"),
        ({"run_id": None}, "run_id"),
        ({"run_id": ""}, "run_id"),
        ({"run_id": "  "}, "run_id"),
        ({"run_id": 20261117}, "run_id"),
    ],
    ids=["format_version", "label_source", "empty_rubric", "duplicate_rubric", "files_not_list",
         "parent_path", "absolute_path", "missing_file", "run_id_missing", "run_id_empty", "run_id_blank",
         "run_id_not_string"],
)
def test_load_export_rejects_bad_manifest(tmp_path, override, match):
    run_dir = _write_export(tmp_path, {"subj-a": [_line()]}, manifest_override=override)
    with pytest.raises(data.ExportFormatError, match=match):
        data.load_export(run_dir)


def test_load_export_rejects_manifest_that_is_not_json(tmp_path):
    (tmp_path / "manifest.json").write_text("{", encoding="utf-8")
    with pytest.raises(data.ExportFormatError, match="JSON"):
        data.load_export(tmp_path)


# ── 利用者単位の分割 ────────────────────────────────────────


def _samples(n_subjects, per_subject=3):
    return [
        data.Sample(sample_id=f"{s}-{j}", subject=f"subj-{s}", turns=({"q": "", "a": "x"},), labels=_labels())
        for s in range(n_subjects)
        for j in range(per_subject)
    ]


def test_split_by_subject_keeps_each_subject_on_one_side():
    samples = _samples(10)
    tr, ev = data.split_by_subject(samples, 0.2, seed=7)
    tr_subj, ev_subj = {s.subject for s in tr}, {s.subject for s in ev}
    assert tr_subj.isdisjoint(ev_subj)
    assert len(ev_subj) == 2 and len(tr_subj) == 8
    assert sorted(s.sample_id for s in tr + ev) == sorted(s.sample_id for s in samples)
    # 同じ人の面接は全件が同じ側に入る
    for subj in ev_subj:
        assert sum(s.subject == subj for s in ev) == 3


def test_split_by_subject_is_deterministic_and_order_independent():
    samples = _samples(10)
    a = data.split_by_subject(samples, 0.3, seed=7)
    b = data.split_by_subject(list(reversed(samples)), 0.3, seed=7)
    assert {s.subject for s in a[1]} == {s.subject for s in b[1]}
    other = {frozenset(s.subject for s in data.split_by_subject(samples, 0.3, seed=k)[1]) for k in range(20)}
    assert len(other) > 1  # 種で分け方が変わる（種を無視していない）


def test_split_by_subject_two_subjects_puts_one_on_each_side():
    tr, ev = data.split_by_subject(_samples(2), 0.01, seed=1)
    assert len({s.subject for s in tr}) == 1 and len({s.subject for s in ev}) == 1
    tr, ev = data.split_by_subject(_samples(2), 0.99, seed=1)
    assert len({s.subject for s in tr}) == 1 and len({s.subject for s in ev}) == 1


def test_split_by_subject_requires_two_subjects():
    with pytest.raises(ValueError, match="2人以上"):
        data.split_by_subject(_samples(1), 0.2, seed=1)


@pytest.mark.parametrize("fraction", [0.0, 1.0, -0.1])
def test_split_by_subject_rejects_bad_fraction(fraction):
    with pytest.raises(ValueError, match="eval_fraction"):
        data.split_by_subject(_samples(3), fraction, seed=1)


# ── モデルの予測の規則 ───────────────────────────────────────


def _hand_model(thresholds):
    """「a:x」の重みだけが 1 のモデル。回答「x」の特徴は1つで値 1 なので、w·x = 1 になる。"""
    w = np.zeros((len(RUBRIC_KEYS), features.HASH_DIM))
    w[:, features.feature_index("a", "x")] = 1.0
    th = np.tile(np.asarray(thresholds, dtype=np.float64), (len(RUBRIC_KEYS), 1))
    return model.build_model(w, th, RUBRIC_KEYS, {"note": "hand"})


@pytest.mark.parametrize(
    "thresholds,expected",
    [
        ([-1.0, 0.0, 0.5, 2.0, 3.0], 3),
        ([2.0, 2.0, 2.0, 2.0, 2.0], 0),
        ([-2.0, -2.0, -2.0, -2.0, -2.0], 5),
        ([1.0, 1.0, 1.0, 1.0, 1.0], 0),  # w·x と閾値が等しいときは上回っていない
    ],
)
def test_predict_counts_thresholds_below_decision(thresholds, expected):
    m = _hand_model(thresholds)
    assert m.predict([{"q": "", "a": "x"}]) == {k: expected for k in RUBRIC_KEYS}


def test_predict_on_empty_features_uses_zero_decision():
    m = _hand_model([-1.0, 0.0, 0.5, 2.0, 3.0])
    assert m.predict([{"q": "", "a": ""}]) == {k: 1 for k in RUBRIC_KEYS}  # z = 0 は -1 だけを上回る
    assert m.expected([{"q": "", "a": ""}])["logic"] == pytest.approx(
        sum(1 / (1 + np.exp(-(0 - t))) for t in [-1.0, 0.0, 0.5, 2.0, 3.0])
    )


# ── 保存と読み込み ──────────────────────────────────────────


def _trained_model():
    m = _hand_model([-1.0, 0.0, 0.5, 2.0, 3.0])
    rng = np.random.default_rng(0)
    w = rng.normal(0, 0.5, size=(len(RUBRIC_KEYS), features.HASH_DIM))
    return model.build_model(w, m.thresholds, RUBRIC_KEYS, {"train_samples": 1})


def test_save_and_load_round_trip(tmp_path):
    m = _trained_model()
    path = tmp_path / "scorer.bin"
    size = model.save(m, path)
    assert size == path.stat().st_size
    assert size <= model.MAX_MODEL_BYTES
    loaded = model.load(path)
    assert loaded.rubric_keys == RUBRIC_KEYS
    assert np.array_equal(loaded.weights, m.weights)
    assert np.array_equal(loaded.thresholds, m.thresholds)
    assert loaded.header == m.header
    turns = [{"q": "志望動機は？", "a": "理由は二つあります。"}]
    assert loaded.predict(turns) == m.predict(turns)


def test_saved_file_layout_and_header(tmp_path):
    """ファイルの並び（MAGIC・ヘッダ長・ヘッダ・float16 LE の重み）と、ヘッダの版・チェックサムを確かめる。"""
    import hashlib
    import struct

    m = _trained_model()
    path = tmp_path / "scorer.bin"
    model.save(m, path)
    raw = path.read_bytes()
    assert raw[:8] == b"SOCSCORE"
    (n,) = struct.unpack_from("<I", raw, 8)
    header = json.loads(raw[12 : 12 + n].decode("utf-8"))
    blob = raw[12 + n :]
    assert len(blob) == 5 * 65536 * 2 == header["weights"]["byte_length"]
    assert header["weights"]["sha256"] == hashlib.sha256(blob).hexdigest()
    assert header["format_version"] == model.MODEL_FORMAT_VERSION
    assert header["rubric"]["version"] == model.rubric_version(RUBRIC_KEYS)
    assert header["rubric"]["keys"] == RUBRIC_KEYS
    assert header["label_source"] == "llm"
    assert header["features"]["hash"] == "fnv1a32" and header["features"]["dim"] == 65536
    assert header["features"]["ngram_min"] == 1 and header["features"]["ngram_max"] == 3
    # NFKC の Unicode の版を書く（Go は norm.Version と比べる）。それ以外は features.spec() と同じ。
    import unicodedata

    assert header["features"]["unicode_version"] == unicodedata.unidata_version
    assert {k: v for k, v in header["features"].items() if k != "unicode_version"} == features.spec()
    assert set(header["thresholds"]) == set(RUBRIC_KEYS)
    # 軸の順・添字の順に並んでいる（2軸目の 10 番目の重み）
    w = np.frombuffer(blob, dtype="<f2").reshape(5, 65536)
    assert w[1, 10] == m.weights[1, 10]
    assert struct.unpack_from("<e", blob, (1 * 65536 + 10) * 2)[0] == float(m.weights[1, 10])


def test_rubric_version_changes_with_keys_and_order():
    v = model.rubric_version(RUBRIC_KEYS)
    assert v.startswith("sha256:") and len(v) == len("sha256:") + 16
    assert model.rubric_version(list(reversed(RUBRIC_KEYS))) != v
    assert model.rubric_version(RUBRIC_KEYS[:-1]) != v


def test_load_rejects_checksum_mismatch(tmp_path):
    path = tmp_path / "scorer.bin"
    model.save(_trained_model(), path)
    raw = bytearray(path.read_bytes())
    raw[-1] ^= 0x01  # 重みの最後の1バイトを変える
    path.write_bytes(bytes(raw))
    with pytest.raises(model.ModelFormatError, match="チェックサム"):
        model.load(path)


def _rewrite_header(raw: bytes, mutate) -> bytes:
    import struct

    (n,) = struct.unpack_from("<I", raw, 8)
    header = json.loads(raw[12 : 12 + n])
    mutate(header)
    head = json.dumps(header).encode("utf-8")
    return raw[:8] + struct.pack("<I", len(head)) + head + raw[12 + n :]


@pytest.mark.parametrize(
    "mutate,match",
    [
        (lambda h: h.update(format="other"), "format"),
        (lambda h: h.update(format_version=2), "版"),
        (lambda h: h["features"].update(dim=1024), "特徴量の設定"),
        (lambda h: h["features"].update(ngram_max=4), "特徴量の設定"),
        (lambda h: h["weights"].update(dtype="float32"), "重みの形式"),
        (lambda h: h["weights"].update(byte_length=10), "長さ"),
        (lambda h: h["thresholds"].update(logic=[0.0, 1.0]), "閾値の数"),
        (lambda h: h["thresholds"].update(logic=[0.0, 1.0, "x", 2.0, 3.0]), "閾値の値"),
        (lambda h: h["thresholds"].update(logic=[0.0, 1.0, float("nan"), 2.0, 3.0]), "閾値の値"),
        (lambda h: h.pop("thresholds"), "必要な項目"),
        (lambda h: h["rubric"].update(keys=[]), "軸が空"),
        (lambda h: h["rubric"].update(score_max=6), "値域"),
        (lambda h: h["features"].update(unicode_version="13.0.0" if features.unicode_version() != "13.0.0" else "15.0.0"),
         "Unicode の版"),
        (lambda h: h["features"].pop("unicode_version"), "Unicode の版"),
        (lambda h: h.update(features="x"), "必要な項目"),
    ],
    ids=["format", "format_version", "dim", "ngram", "dtype", "byte_length", "threshold_count",
         "threshold_not_number", "threshold_nan", "no_thresholds", "empty_rubric", "score_range",
         "unicode_version_differs", "unicode_version_missing", "features_not_object"],
)
def test_load_rejects_bad_header(tmp_path, mutate, match):
    path = tmp_path / "scorer.bin"
    model.save(_trained_model(), path)
    path.write_bytes(_rewrite_header(path.read_bytes(), mutate))
    with pytest.raises(model.ModelFormatError, match=match):
        model.load(path)


@pytest.mark.parametrize(
    "raw,match",
    [
        (b"", "先頭"),
        (b"NOTMODEL\x00\x00\x00\x00", "先頭"),
        (b"SOCSCORE\xff\xff\xff\x00{}", "ヘッダの長さ"),
        (b"SOCSCORE\x05\x00\x00\x00{}", "ヘッダの長さ"),
        (b"SOCSCORE\x02\x00\x00\x00{x", "JSON"),
    ],
    ids=["empty", "bad_magic", "huge_header", "header_past_end", "header_not_json"],
)
def test_decode_rejects_broken_files(raw, match):
    with pytest.raises(model.ModelFormatError, match=match):
        model.decode(raw)


def test_load_rejects_truncated_weights(tmp_path):
    path = tmp_path / "scorer.bin"
    model.save(_trained_model(), path)
    path.write_bytes(path.read_bytes()[:-2])
    with pytest.raises(model.ModelFormatError, match="長さ"):
        model.load(path)


def test_save_refuses_file_over_size_limit(tmp_path, monkeypatch):
    m = _trained_model()
    size = len(model.encode(m))
    monkeypatch.setattr(model, "MAX_MODEL_BYTES", size - 1)
    path = tmp_path / "scorer.bin"
    with pytest.raises(ValueError, match="上限"):
        model.save(m, path)
    assert not path.exists()
    monkeypatch.setattr(model, "MAX_MODEL_BYTES", size)
    assert model.save(m, path) == size  # ちょうど上限なら保存できる


def test_size_limit_is_2_7mb():
    assert model.MAX_MODEL_BYTES == 2_700_000


@pytest.mark.parametrize("bad", [np.inf, np.nan, 70000.0, -70000.0])
def test_quantize_rejects_values_outside_float16(bad):
    w = np.zeros((1, 4))
    w[0, 2] = bad
    with pytest.raises(ValueError, match="float16"):
        model.quantize(w)


def test_quantize_accepts_float16_max():
    w = np.asarray([[65504.0, -65504.0, 0.5]])
    assert model.quantize(w).dtype == np.dtype("<f2")


# ── 評価の指標とレポート ──────────────────────────────────────


def test_mae_and_qwk_values():
    assert evaluate.mae([0, 2, 5], [1, 2, 3]) == pytest.approx(1.0)
    assert evaluate.qwk([0, 1, 2, 3, 4, 5], [0, 1, 2, 3, 4, 5]) == pytest.approx(1.0)
    # 完全に逆順なら -1
    assert evaluate.qwk([0, 5], [5, 0]) == pytest.approx(-1.0)
    # 手計算: 正解 [0,0,5]・予測 [0,5,5]。0 と 5 の間の重みは (0-5)^2/25 = 1。
    # 観測の不一致は (0,5) の1件 = 1。期待は 正解の周辺 {0:2, 5:1} と予測の周辺 {0:1, 5:2} から
    # E(0,5) = 2×2/3、E(5,0) = 1×1/3 で、不一致の合計 5/3。QWK = 1 - 1 / (5/3) = 0.4。
    assert evaluate.qwk([0, 0, 5], [0, 5, 5]) == pytest.approx(0.4)


def test_qwk_undefined_when_only_one_category():
    assert evaluate.qwk([3, 3, 3], [3, 3, 3]) is None


def test_metrics_reject_empty_input():
    with pytest.raises(ValueError):
        evaluate.mae([], [])
    with pytest.raises(ValueError):
        evaluate.qwk([], [])


# ── 学習 → 保存 → 読み込み → 予測（CLI を含む一周） ─────────────────


def test_run_end_to_end_on_synthetic_export(tmp_path):
    run_dir = _synthetic_export(tmp_path / "export")
    model_out, report_out = tmp_path / "scorer.bin", tmp_path / "report.md"
    result = train.run(run_dir, model_out, report_out, FAST)

    # 2.7MB 以内のモデルファイルができ、ヘッダに版とチェックサムが入る（Issue の受け入れ条件）
    assert model_out.stat().st_size == result.model_bytes <= model.MAX_MODEL_BYTES
    loaded = model.load(model_out)
    assert loaded.header["format_version"] == model.MODEL_FORMAT_VERSION
    assert loaded.header["rubric"]["version"] == model.rubric_version(RUBRIC_KEYS)
    assert len(loaded.header["weights"]["sha256"]) == 64
    info = loaded.header["training"]
    assert info["train_subjects"] == 8 and info["eval_subjects"] == 2
    assert info["train_samples"] == 48 and info["eval_samples"] == 12
    assert info["seed"] == FAST.seed and info["export_run_id"] == "20261117T000000Z"

    # 目印の語からラベルを学習できている（配線が通っていれば、評価用の分割でも大きく外さない）
    assert [m.key for m in result.metrics] == RUBRIC_KEYS
    for m in result.metrics:
        assert m.mae < 0.6, m
        assert m.qwk is not None and m.qwk > 0.8, m
    labels = {"logic": 5, "specificity": 0, "ownership": 2, "communication": 4, "enthusiasm": 1}
    rng = random.Random(99)
    pred = loaded.predict([{"q": "学生時代に力を入れたことは？", "a": _synthetic_answer(labels, rng)}])
    assert set(pred) == set(RUBRIC_KEYS)
    for key in RUBRIC_KEYS:
        assert abs(pred[key] - labels[key]) <= 1, (key, pred, labels)

    report = report_out.read_text(encoding="utf-8")
    head = "\n".join(report.splitlines()[:4])
    assert "LLM ラベルで学習" in head
    assert "LLM ラベルとの一致であり、教員の評価セットでの精度ではありません" in head
    for key in RUBRIC_KEYS:
        assert f"| {key} |" in report
    assert result.model_sha256 in report


def test_run_evaluates_only_eval_subjects_with_saved_model(tmp_path, monkeypatch):
    """評価は、評価用の分割（学習に入れていない利用者）だけで、保存したファイルを読み直したモデルで行う。"""
    run_dir = _synthetic_export(tmp_path / "export", n_subjects=5, per_subject=2)
    seen = {}
    real = evaluate.evaluate

    def spy(m, samples):
        seen["model"], seen["samples"] = m, list(samples)
        return real(m, samples)

    monkeypatch.setattr(evaluate, "evaluate", spy)
    cfg = model.TrainConfig(epochs=10)
    train.run(run_dir, tmp_path / "m.bin", tmp_path / "r.md", cfg)

    all_samples = data.load_export(run_dir).samples
    _, expected_eval = data.split_by_subject(all_samples, train.DEFAULT_EVAL_FRACTION, cfg.seed)
    assert [s.sample_id for s in seen["samples"]] == [s.sample_id for s in expected_eval]
    assert seen["model"].header == model.load(tmp_path / "m.bin").header


def test_run_is_deterministic(tmp_path):
    run_dir = _synthetic_export(tmp_path / "export", n_subjects=4, per_subject=3)
    cfg = model.TrainConfig(epochs=30)
    train.run(run_dir, tmp_path / "a.bin", tmp_path / "a.md", cfg)
    train.run(run_dir, tmp_path / "b.bin", tmp_path / "b.md", cfg)
    assert (tmp_path / "a.bin").read_bytes() == (tmp_path / "b.bin").read_bytes()


def test_run_reports_skipped_lines(tmp_path):
    run_dir = _synthetic_export(tmp_path / "export", n_subjects=3, per_subject=2)
    with (run_dir / "subjects" / "subj-00.jsonl").open("a", encoding="utf-8") as fh:
        fh.write("{broken\n")
        fh.write(json.dumps(_line("bad-1", "subj-00", labels=_labels(logic=7))) + "\n")
    train.run(run_dir, tmp_path / "m.bin", tmp_path / "r.md", model.TrainConfig(epochs=10))
    report = (tmp_path / "r.md").read_text(encoding="utf-8")
    assert "読み飛ばした行: 2 件（invalid_json 1 件、invalid_labels 1 件）" in report
    assert model.load(tmp_path / "m.bin").header["training"]["skipped_lines"] == 2


def test_run_rejects_export_without_usable_lines(tmp_path):
    run_dir = _write_export(tmp_path / "export", {"subj-a": []}, raw_lines={"subj-a": ["{broken\n"]})
    with pytest.raises(ValueError, match="学習に使える行がありません"):
        train.run(run_dir, tmp_path / "m.bin", tmp_path / "r.md", FAST)
    assert not (tmp_path / "m.bin").exists()


def test_main_success_writes_model_and_report(tmp_path):
    run_dir = _synthetic_export(tmp_path / "export", n_subjects=3, per_subject=2)
    rc = train.main([
        "--input", str(run_dir), "--model-out", str(tmp_path / "m.bin"), "--report-out", str(tmp_path / "r.md"),
        "--epochs", "10", "--seed", "3",
    ])
    assert rc == 0
    header = model.load(tmp_path / "m.bin").header
    assert header["training"]["seed"] == 3 and header["training"]["epochs"] == 10
    assert (tmp_path / "r.md").read_text(encoding="utf-8").startswith("# 軽量採点モデルの学習レポート")


@pytest.mark.parametrize(
    "setup,extra",
    [
        ("empty", []),
        ("no_manifest", []),
        ("one_subject", []),
        ("ok", ["--epochs", "0"]),
        ("ok", ["--learning-rate", "0"]),
        ("ok", ["--l2", "-1"]),
        ("ok", ["--eval-fraction", "1"]),
    ],
    ids=["empty_export", "no_manifest", "one_subject", "zero_epochs", "zero_lr", "negative_l2", "eval_fraction_1"],
)
def test_main_returns_2_on_bad_input(tmp_path, setup, extra):
    run_dir = tmp_path / "export"
    if setup == "empty":
        _write_export(run_dir, {})
    elif setup == "no_manifest":
        run_dir.mkdir()
    elif setup == "one_subject":
        _synthetic_export(run_dir, n_subjects=1, per_subject=3)
    else:
        _synthetic_export(run_dir, n_subjects=3, per_subject=2)
    rc = train.main(
        ["--input", str(run_dir), "--model-out", str(tmp_path / "m.bin"), "--report-out", str(tmp_path / "r.md")]
        + extra
    )
    assert rc == 2
    assert not (tmp_path / "m.bin").exists()


# ── #2326 のレビューを受けた追加 ─────────────────────────────


def test_load_export_skips_invalid_utf8_lines_without_replacing(tmp_path):
    """不正な UTF-8 の行は U+FFFD に置き換えて読まず、壊れた行として数える。同じファイルのほかの行は読む。"""
    run_dir = _write_export(tmp_path, {"subj-a": [_line("s-1", "subj-a")]})
    path = run_dir / "subjects" / "subj-a.jsonl"
    broken = json.dumps(_line("s-2", "subj-a")).encode("utf-8").replace(b"subj-a", b"subj-\xff", 1)
    assert b"\xff" in broken
    path.write_bytes(path.read_bytes() + broken + b"\n" + json.dumps(_line("s-3", "subj-a")).encode("utf-8") + b"\n")
    ds = data.load_export(run_dir)
    assert [s.sample_id for s in ds.samples] == ["s-1", "s-3"]
    assert ds.skipped == Counter({"invalid_utf8": 1})
    assert all("\ufffd" not in s.subject for s in ds.samples)


@pytest.mark.parametrize("same", ["identical", "via_dotdot"])
def test_run_rejects_same_model_and_report_path(tmp_path, same):
    """--model-out と --report-out が同じファイルなら、何も書かずにエラーにする（モデルをレポートで上書きしない）。"""
    run_dir = _synthetic_export(tmp_path / "export", n_subjects=3, per_subject=2)
    out = tmp_path / "out.bin"
    other = out if same == "identical" else tmp_path / "export" / ".." / "out.bin"
    with pytest.raises(ValueError, match="同じパス"):
        train.run(run_dir, out, other, FAST)
    assert not out.exists()


def test_main_returns_2_when_model_and_report_are_same(tmp_path):
    run_dir = _synthetic_export(tmp_path / "export", n_subjects=3, per_subject=2)
    out = str(tmp_path / "out.bin")
    assert train.main(["--input", str(run_dir), "--model-out", out, "--report-out", out]) == 2
    assert not (tmp_path / "out.bin").exists()
