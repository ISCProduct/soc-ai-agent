"""埋め込み前の切り詰めでエンコーダを使い回す（SOCAIAGENT-569）。"""
from unittest.mock import patch

import tiktoken

from services import embed


def test_encoder_is_created_once_per_model():
    """複数の文書を切り詰めても、エンコーダはモデルごとに1度だけ作る。"""
    embed._encoder_for.cache_clear()
    real = tiktoken.encoding_for_model
    with patch.object(embed.tiktoken, "encoding_for_model", side_effect=real) as created:
        for text in ["一つ目の文書", "二つ目の文書", "三つ目の文書"]:
            embed._truncate_text(text, "text-embedding-3-small")
    assert created.call_count == 1
    embed._encoder_for.cache_clear()


def test_unknown_model_falls_back_to_cl100k():
    embed._encoder_for.cache_clear()
    assert embed._encoder_for("no-such-model").name == "cl100k_base"
    embed._encoder_for.cache_clear()


def test_truncate_still_cuts_long_text(monkeypatch):
    """使い回しても、上限を超える文書は今までどおり切り詰める。"""
    import main

    monkeypatch.setattr(main, "MAX_EMBED_TOKENS", 5)
    out = embed._truncate_text("あいうえお" * 20, "text-embedding-3-small")
    assert len(embed._encoder_for("text-embedding-3-small").encode(out)) <= 5
