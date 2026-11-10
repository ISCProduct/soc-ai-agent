"""注釈PDFが本文を隠さないことと、OCRスコアを残すことの確認（#1942）。"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import fitz

from annotate_pdf import annotate_pages
from ocr_extract import parse_ocr_line


def test_parse_ocr_line_keeps_confidence():
    block = parse_ocr_line(0, ([[0, 0], [10, 0], [10, 4], [0, 4]], ("売上", 0.42)))
    assert block["text"] == "売上"
    assert block["confidence"] == 0.42
    assert block["bbox"] == [0, 0, 10, 4]


def test_parse_ocr_line_without_score_omits_confidence():
    block = parse_ocr_line(1, ([[0, 0], [8, 0], [8, 4], [0, 4]], "本文だけ"))
    assert block["text"] == "本文だけ"
    assert "confidence" not in block


def test_highlight_is_translucent_and_badge_stays_off_the_line():
    doc = fitz.open()
    page = doc.new_page(width=300, height=400)
    text_rect = fitz.Rect(72, 88, 220, 108)
    annotate_pages(doc, [{
        "page_number": 1,
        "bbox": [text_rect.x0, text_rect.y0, text_rect.x1, text_rect.y1],
        "severity": "warning",
        "message": "具体性",
        "quote": "原文が見える",
        "rewrite": "売上を前年比120%に伸ばしました",
    }])

    fills = []
    badges = []
    for drawing in page.get_drawings():
        kind = drawing["items"][0][0] if drawing["items"] else ""
        if kind == "re":
            fills.append(drawing)
        elif kind == "c":
            badges.append(drawing)

    assert fills, "ハイライト矩形が無い"
    assert all(drawing.get("fill_opacity", 1) < 0.5 for drawing in fills), fills
    assert badges, "番号印が無い"
    # 円の中心は矩形の中に入らない（行頭を隠さない）。
    for drawing in badges:
        rect = drawing["rect"]
        cx = (rect.x0 + rect.x1) / 2
        cy = (rect.y0 + rect.y1) / 2
        assert not text_rect.contains(fitz.Point(cx, cy)), rect
