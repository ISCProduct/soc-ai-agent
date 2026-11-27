"""学生検索の関連度は、コレクションの距離関数（l2 / cosine）によらず同じ値になる（SOCAIAGENT-554）。

cosine のコレクションへ切り替えても、関連度の下限（STUDENT_SEARCH_MIN_SCORE、既定 0.38）の意味が変わらないこと、
l2 のコレクションでは以前の 1/(1+d) と同じ値のままであることを確かめる。
"""
import math
from unittest.mock import patch

import pytest

import student_profile_store as sps
from evaluation.search_quality.memory_store import MemoryClient


def _unit(v):
    n = math.sqrt(sum(x * x for x in v))
    return [x / n for x in v]


STUDENTS = {1: _unit([1.0, 0.0, 0.0]), 2: _unit([0.8, 0.6, 0.0]), 3: _unit([0.1, 0.2, 0.97])}
QUERY = _unit([0.9, 0.3, 0.1])


def _query(metadata):
    client = MemoryClient()
    with patch.object(sps, "get_chroma_client", return_value=client):
        client.get_or_create_collection("student_profiles", metadata=metadata)
        for uid, emb in STUDENTS.items():
            sps.upsert_student(uid, "", emb)
        return sps.query_students(QUERY, 10), client


def test_new_student_collection_is_cosine():
    client = MemoryClient()
    with patch.object(sps, "get_chroma_client", return_value=client):
        sps.upsert_student(1, "", STUDENTS[1])
    assert client.get_collection("student_profiles").metadata == {"hnsw:space": "cosine"}


def test_scores_are_the_same_for_l2_and_cosine_collections():
    l2_hits, _ = _query(None)
    cos_hits, _ = _query({"hnsw:space": "cosine"})

    assert [h["user_id"] for h in l2_hits] == [h["user_id"] for h in cos_hits]
    for a, b in zip(l2_hits, cos_hits):
        assert a["score"] == pytest.approx(b["score"])


def test_l2_scores_match_the_previous_formula():
    # 以前の l2 のコレクションでの値（1/(1+d)、d はユークリッド距離の2乗）から変わらない
    hits, _ = _query(None)
    for hit in hits:
        d = sum((x - y) ** 2 for x, y in zip(QUERY, STUDENTS[hit["user_id"]]))
        assert hit["score"] == pytest.approx(1.0 / (1.0 + d))


@pytest.mark.parametrize("metadata", [None, {"hnsw:space": "cosine"}])
def test_min_score_filters_the_same_students(metadata):
    client = MemoryClient()
    with patch.object(sps, "get_chroma_client", return_value=client):
        client.get_or_create_collection("student_profiles", metadata=metadata)
        for uid, emb in STUDENTS.items():
            sps.upsert_student(uid, "", emb)
        kept = [h["user_id"] for h in sps.query_students(QUERY, 10, min_score=0.5)]
    assert kept == [1, 2]


@pytest.mark.parametrize("space,distance,expected", [("l2", 0.5, 1 / 1.5), ("cosine", 0.25, 1 / 1.5), ("cosine", 0.0, 1.0)])
def test_relevance_score(space, distance, expected):
    assert sps.relevance_score(distance, space) == pytest.approx(expected)
