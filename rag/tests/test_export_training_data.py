"""export_training_data.py の PIIマスキングテスト(#993)。

カタカナ/ひらがな表記の氏名(敬称隣接)を見逃していた問題の回帰防止。
"""
from export_training_data import mask_pii, to_outcome_example, OUTCOME_LABELS


def test_mask_pii_masks_kanji_name_with_honorific():
    assert mask_pii("山田太郎さんは新卒です") == "<PERSON>さんは新卒です"


def test_mask_pii_masks_katakana_name_with_honorific():
    assert mask_pii("タナカタロウさんは新卒です") == "<PERSON>さんは新卒です"


def test_mask_pii_masks_hiragana_name_with_honorific():
    assert mask_pii("たなかたろうさんは新卒です") == "<PERSON>さんは新卒です"


def test_mask_pii_masks_katakana_name_with_long_vowel_mark():
    assert mask_pii("ジョーンズさんが担当です") == "<PERSON>さんが担当です"


def test_mask_pii_masks_email_and_url():
    assert mask_pii("連絡先: test@example.com") == "連絡先: <EMAIL>"
    assert mask_pii("詳細は https://example.com/page を参照") == "詳細は <URL> を参照"


# #993フォローアップ(CodeRabbit指摘): 漢字とひらがなが混在する文字クラスだと
# 「今日は田中さん」の「今日は」まで氏名の一部として誤って消費し、本文が
# 不可逆に失われていた。周辺テキストが保持されることを確認する。
def test_mask_pii_preserves_surrounding_text_before_kanji_name():
    assert mask_pii("今日は田中さんに会いました") == "今日は<PERSON>さんに会いました"


def test_mask_pii_hiragana_name_without_boundary_is_not_masked():
    # ひらがなの氏名は助詞と紛れやすいため、直前に文頭/空白/句読点等の境界が
    # 無い場合は検出しない(本文破壊を避けるためのトレードオフ、既知の制限)。
    assert mask_pii("私はたろうさんに会った") == "私はたろうさんに会った"


# 蒸留禁止(他モデルの出力を教師信号にしないこと)の回帰防止テスト。
# to_outcome_example は選考結果(application_status)のみをlabelにし、
# AI発話("role": "ai")は prompt・label のどちらにも一切含めてはならない。

def _session_with_ai_reply(status: str = "rejected") -> dict:
    return {
        "id": 1,
        "application_status": status,
        "utterances": [
            {"role": "user", "text": "自己紹介をお願いします"},
            {"role": "ai", "text": "こんにちは。AIが生成した応答です"},
        ],
    }


def test_to_outcome_example_excludes_ai_utterances_from_prompt():
    ex = to_outcome_example(_session_with_ai_reply())
    assert ex is not None
    assert "AIが生成した応答です" not in ex["prompt"]


def test_to_outcome_example_label_is_restricted_to_allowed_outcomes():
    ex = to_outcome_example(_session_with_ai_reply(status="rejected"))
    assert ex["completion"] in OUTCOME_LABELS.values()
    # AI生成テキストがlabelに紛れ込んでいないこと
    assert ex["completion"] != "こんにちは。AIが生成した応答です"


def test_to_outcome_example_returns_none_for_unresolved_status():
    # 選考中("applied"等)や辞退("withdrawn")は合否シグナルではないため除外する
    session = _session_with_ai_reply(status="applied")
    assert to_outcome_example(session) is None
