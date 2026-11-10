"""検索品質の固定セット評価（#1773）のテスト。

CI では chromadb がスタブなので、メモリ上の Chroma 互換クライアントと決定的なハッシュ埋め込みで
本番と同じ検索経路を通す。同梱の基準値より下がったら、検索経路（絞り込み・鮮度・件数・順位）の
変更が品質を落としたことになる。
"""
import json

import pytest

from evaluation.search_quality.dataset import DatasetError, default_dataset_path, load_dataset
from evaluation.search_quality.embedders import hash_embedder
from evaluation.search_quality.memory_store import MemoryClient
from evaluation.search_quality.metrics import (
    evaluate_query,
    mean_scores,
    ndcg_at_k,
    precision_at_k,
    recall_at_k,
    reciprocal_rank,
)
from evaluation.search_quality.runner import (
    compare_to_baseline,
    evaluate_company_research,
    evaluate_student_search,
)
from scripts.eval_search_quality import HASH_BASELINE, run, strip_details


# ── 指標 ──────────────────────────────────────────


def test_precision_and_recall():
    ranked = ["a", "x", "b", "y"]
    assert precision_at_k(ranked, ["a", "b"], 2) == 0.5
    assert recall_at_k(ranked, ["a", "b"], 2) == 0.5
    assert recall_at_k(ranked, ["a", "b"], 3) == 1.0
    # 返った件数が k 未満でも分母は k（件数を絞っただけで良く見えないように）
    assert precision_at_k(["a"], ["a"], 5) == 0.2


def test_reciprocal_rank():
    assert reciprocal_rank(["x", "a"], ["a"]) == 0.5
    assert reciprocal_rank(["x", "y"], ["a"]) == 0.0


def test_ndcg():
    assert ndcg_at_k(["a", "b"], ["a", "b"], 2) == pytest.approx(1.0)
    assert ndcg_at_k(["x", "a"], ["a"], 2) == pytest.approx(1 / 1.5849625007, rel=1e-6)
    assert ndcg_at_k(["x", "y"], ["a"], 2) == 0.0


def test_invalid_arguments():
    with pytest.raises(ValueError):
        precision_at_k(["a"], ["a"], 0)
    with pytest.raises(ValueError):
        recall_at_k(["a"], [], 1)


def test_mean_scores():
    rows = [evaluate_query(["a"], ["a"], [1]), evaluate_query(["x"], ["a"], [1])]
    assert mean_scores(rows)["recall@1"] == 0.5


# ── 評価セット ─────────────────────────────────────


@pytest.mark.parametrize("name", ["student_search", "company_research"])
def test_bundled_datasets_are_valid(name):
    dataset = load_dataset(default_dataset_path(name))
    assert len(dataset.documents) >= 10
    assert len(dataset.queries) >= 10


def _write(tmp_path, data):
    path = tmp_path / "dataset.json"
    path.write_text(json.dumps(data, ensure_ascii=False), encoding="utf-8")
    return path


@pytest.mark.parametrize(
    "data",
    [
        # 正解が文書に無い
        {"documents": [{"id": "d1", "text": "本文"}], "queries": [{"id": "q1", "query": "問い", "relevant": ["d9"]}]},
        # 文書 ID の重複
        {
            "documents": [{"id": "d1", "text": "本文1"}, {"id": "d1", "text": "本文2"}],
            "queries": [{"id": "q1", "query": "問い", "relevant": ["d1"]}],
        },
        # 本文の重複（企業リサーチは本文から ID を引くので禁止）
        {
            "documents": [{"id": "d1", "text": "同じ"}, {"id": "d2", "text": "同じ"}],
            "queries": [{"id": "q1", "query": "問い", "relevant": ["d1"]}],
        },
        # 個人情報らしき文字列
        {"documents": [{"id": "d1", "text": "連絡先 taro@example.com"}], "queries": [{"id": "q1", "query": "問い", "relevant": ["d1"]}]},
        {"documents": [{"id": "d1", "text": "電話 090-1234-5678"}], "queries": [{"id": "q1", "query": "問い", "relevant": ["d1"]}]},
        # 正解が空
        {"documents": [{"id": "d1", "text": "本文"}], "queries": [{"id": "q1", "query": "問い", "relevant": []}]},
    ],
)
def test_invalid_datasets_are_rejected(tmp_path, data):
    with pytest.raises(DatasetError):
        load_dataset(_write(tmp_path, data))


# ── 検索経路 ───────────────────────────────────────


def test_student_search_runs_through_production_path():
    dataset = load_dataset(default_dataset_path("student_search"))
    summary = evaluate_student_search(dataset, hash_embedder(), MemoryClient())
    assert summary["queries"] == len(dataset.queries)
    for row in summary["per_query"]:
        # 全員が登録されているので、上位10件は必ず埋まる
        assert len(row["ranked"]) == 10
    for value in summary["mean"].values():
        assert 0.0 <= value <= 1.0


def test_company_research_never_returns_other_companies():
    dataset = load_dataset(default_dataset_path("company_research"))
    summary = evaluate_company_research(dataset, hash_embedder(), MemoryClient())
    assert summary["foreign_documents"] == 0
    by_id = {doc.id: doc for doc in dataset.documents}
    for query, row in zip(dataset.queries, summary["per_query"]):
        assert row["ranked"], f"{query.id}: 同じ企業の文書が1件も返らない"
        assert {by_id[doc_id].company for doc_id in row["ranked"]} == {query.company}


def test_company_research_falls_back_to_same_company_when_role_is_missing():
    dataset = load_dataset(default_dataset_path("company_research"))
    summary = evaluate_company_research(dataset, hash_embedder(), MemoryClient())
    rows = {row["id"]: row for row in summary["per_query"]}
    # cq09 はサクラ製作所の営業。営業の文書は無いので、同じ企業のエンジニアの文書が返る
    assert rows["cq09"]["ranked"] and all(doc_id.startswith("c-eng-") for doc_id in rows["cq09"]["ranked"])


def test_hash_memory_results_match_bundled_baseline():
    """同梱の基準値（hash/memory）より下がらないこと。結果は決定的なので許容幅は 0。

    検索経路を意図して変えて値が変わった場合は、変化を確認したうえで
    `python scripts/eval_search_quality.py --update-baseline` で基準値を更新する。
    """
    report = run("hash", "memory")
    baseline = json.loads(HASH_BASELINE.read_text(encoding="utf-8"))
    assert compare_to_baseline(report, baseline) == []
    assert strip_details(report) == baseline, "基準値と結果がずれている。意図した変更なら基準値を更新する"


def test_compare_to_baseline_reports_regressions():
    report = {"x": {"mean": {"recall@1": 0.4}, "foreign_documents": 1}}
    baseline = {"x": {"mean": {"recall@1": 0.5}, "foreign_documents": 0}}
    regressions = compare_to_baseline(report, baseline)
    assert len(regressions) == 2
    assert compare_to_baseline(report, baseline, tolerance=0.2)[0].startswith("x 他社")
