"""export_training_data.py の PIIマスキングテスト(#993)。

カタカナ/ひらがな表記の氏名(敬称隣接)を見逃していた問題の回帰防止。
"""
from export_training_data import mask_pii


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
