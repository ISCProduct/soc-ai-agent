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
