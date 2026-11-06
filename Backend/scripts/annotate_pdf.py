#!/usr/bin/env python3
import argparse
import io
import json
import os
import sys

import fitz  # PyMuPDF
from PIL import Image, ImageDraw, ImageFont

SEVERITY_COLORS = {
    "critical": {
        "highlight": (1.0, 0.82, 0.82),
        "border": (0.78, 0.15, 0.15),
        "badge": (0.85, 0.15, 0.15),
        "label_fg": (1.0, 1.0, 1.0),
        "ja": "重大",
        "pil_chip": (211, 47, 47),
    },
    "warning": {
        "highlight": (1.0, 0.95, 0.75),
        "border": (0.75, 0.50, 0.05),
        "badge": (0.85, 0.55, 0.05),
        "label_fg": (1.0, 1.0, 1.0),
        "ja": "注意",
        "pil_chip": (237, 108, 2),
    },
    "info": {
        "highlight": (0.82, 0.92, 1.0),
        "border": (0.18, 0.48, 0.78),
        "badge": (0.18, 0.48, 0.78),
        "label_fg": (1.0, 1.0, 1.0),
        "ja": "情報",
        "pil_chip": (2, 136, 209),
    },
}

DEFAULT_COLOR = {
    "highlight": (0.90, 0.90, 0.90),
    "border": (0.40, 0.40, 0.40),
    "badge": (0.40, 0.40, 0.40),
    "label_fg": (1.0, 1.0, 1.0),
    "ja": "",
    "pil_chip": (120, 120, 120),
}


def get_color(severity):
    return SEVERITY_COLORS.get(severity, DEFAULT_COLOR)


def resolve_japanese_font():
    """日本語フォントのパスを解決する。

    優先順位:
    1. ANNOTATION_FONT_PATH 環境変数
    2. 既知のフォントパス候補をフォールバック検索

    Returns:
        str | None: フォントファイルのパス。見つからない場合は None。
    """
    env_path = os.getenv("ANNOTATION_FONT_PATH", "").strip()
    if env_path:
        if os.path.exists(env_path):
            return env_path
        # 環境変数が設定されているがファイルが存在しない場合は明示的に警告
        print(
            f"WARNING: ANNOTATION_FONT_PATH が設定されていますが、ファイルが存在しません: {env_path!r}\n"
            "フォールバック候補を検索します。",
            file=sys.stderr,
        )

    candidates = [
        "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
        "/usr/share/fonts/opentype/noto/NotoSansCJKjp-Regular.otf",
        "/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
        "/usr/share/fonts/truetype/noto/NotoSansCJKjp-Regular.otf",
        "/usr/share/fonts/opentype/noto/NotoSansJP-Regular.otf",
        "/usr/share/fonts/truetype/noto/NotoSansJP-Regular.otf",
        "/usr/share/fonts/truetype/fonts-japanese-gothic.ttf",
        "/System/Library/Fonts/AppleGothic.ttf",
        "/System/Library/Fonts/ヒラギノ角ゴシック W3.ttc",
        "/System/Library/Fonts/ヒラギノ角ゴシック W6.ttc",
        "/System/Library/Fonts/ヒラギノ明朝 ProN W3.otf",
        "/Library/Fonts/Arial Unicode.ttf",
    ]

    for path in candidates:
        if os.path.exists(path):
            print(f"INFO: フォールバックフォントを使用: {path!r}", file=sys.stderr)
            return path

    return None


def wrap_text(draw, text, font, max_width):
    lines = []
    for raw_line in text.split("\n"):
        line = raw_line.strip()
        if not line:
            lines.append("")
            continue

        words = line.split() if " " in line else list(line)
        current = ""

        for word in words:
            test = word if current == "" else (f"{current} {word}" if " " in line else current + word)
            bbox = draw.textbbox((0, 0), test, font=font)
            if bbox[2] <= max_width:
                current = test
            else:
                if current:
                    lines.append(current)
                current = word

        if current:
            lines.append(current)

    return lines


def draw_number_badge(page, rect, number, severity):
    """番号印を本文の外に置く。

    矩形の左上角を中心にすると円の半分が行頭に重なり、指摘された文字が読めない。
    左に余白が無いときは行の上、上も無ければ行の下へ逃がす。
    """
    c = get_color(severity)
    radius = 7
    gap = 2
    cx = rect.x0 - radius - gap
    cy = rect.y0 + radius
    bounds = page.rect
    if cx - radius < bounds.x0:
        cx = min(max(rect.x0 + radius, bounds.x0 + radius), bounds.x1 - radius)
        cy = rect.y0 - radius - gap
        if cy - radius < bounds.y0:
            cy = min(rect.y1 + radius + gap, bounds.y1 - radius)
    page.draw_circle((cx, cy), radius, color=c["badge"], fill=c["badge"])
    label = str(number)
    # insert_text の座標はベースライン。1桁は円の中央付近に載せる。
    half = 2 if len(label) == 1 else 4
    page.insert_text((cx - half, cy + 3), label, fontsize=8, color=c["label_fg"])


def annotate_pages(doc, items):
    for idx, item in enumerate(items, start=1):
        page_number = item.get("page_number", 1)
        bbox = item.get("bbox", [20, 20, 200, 60])
        page_width = item.get("page_width")
        page_height = item.get("page_height")
        severity = item.get("severity", "info")

        page_index = max(0, page_number - 1)
        if page_index >= len(doc):
            continue

        page = doc[page_index]
        rect = fitz.Rect(bbox[0], bbox[1], bbox[2], bbox[3])
        page_rect = page.rect

        if page_width and page_height:
            sx = page_rect.width / float(page_width)
            sy = page_rect.height / float(page_height)
            rect = fitz.Rect(rect.x0 * sx, rect.y0 * sy, rect.x1 * sx, rect.y1 * sy)
        else:
            if 0 <= rect.x0 <= 1.5 and 0 <= rect.y0 <= 1.5 and 0 <= rect.x1 <= 1.5 and 0 <= rect.y1 <= 1.5:
                rect = fitz.Rect(
                    rect.x0 * page_rect.width,
                    rect.y0 * page_rect.height,
                    rect.x1 * page_rect.width,
                    rect.y1 * page_rect.height,
                )

        c = get_color(severity)
        # 既定の不透明塗り（fill_opacity=1, overlay=True）は本文を隠す。
        # 薄い塗りにして、指摘された文字が読めるようにする（#1942）。
        page.draw_rect(
            rect,
            color=c["border"],
            fill=c["highlight"],
            width=1.5,
            fill_opacity=0.28,
        )
        draw_number_badge(page, rect, idx, severity)


def render_review_page(items, page_width_pt, page_height_pt, font_path):
    scale = 2
    w = int(page_width_pt * scale)
    h = int(page_height_pt * scale)
    margin = int(30 * scale)

    image = Image.new("RGB", (w, h), (255, 255, 255))
    draw = ImageDraw.Draw(image)

    def load_font(size):
        if font_path.lower().endswith(".ttc"):
            return ImageFont.truetype(font_path, size, index=0)
        return ImageFont.truetype(font_path, size)

    title_font = load_font(int(16 * scale))
    body_font = load_font(int(9 * scale))
    small_font = load_font(int(8 * scale))

    y = margin

    draw.rectangle([0, 0, w, int(55 * scale)], fill=(38, 64, 115))
    draw.text((margin, int(17 * scale)), "レビュー指摘一覧", font=title_font, fill=(255, 255, 255))
    y = int(65 * scale)

    legend_labels = [("critical", "重大"), ("warning", "注意"), ("info", "情報")]
    lx = margin
    for sev, label in legend_labels:
        chip_color = get_color(sev)["pil_chip"]
        chip_bbox = draw.textbbox((0, 0), label, font=small_font)
        cw = chip_bbox[2] - chip_bbox[0] + int(10 * scale // 2)
        ch = chip_bbox[3] - chip_bbox[1] + int(6 * scale // 2)
        draw.rounded_rectangle([lx, y, lx + cw, y + ch], radius=int(3 * scale // 2), fill=chip_color)
        draw.text((lx + int(5 * scale // 2), y + int(3 * scale // 2)), label, font=small_font, fill=(255, 255, 255))
        lx += cw + int(8 * scale // 2)
    y += int(28 * scale)

    draw.line([(margin, y), (w - margin, y)], fill=(180, 180, 180), width=2)
    y += int(12 * scale)

    body_line_h = int(body_font.size * 1.35)
    small_line_h = int(small_font.size * 1.35)

    for idx, item in enumerate(items, start=1):
        if y > h - margin * 2:
            break

        severity = item.get("severity", "info")
        page_num = item.get("page_number", 1)
        message = item.get("message", "")
        suggestion = item.get("suggestion", "")
        quote = item.get("quote", "") or ""
        rewrite = item.get("rewrite", "") or ""
        withheld = item.get("rewrite_withheld", "") or ""
        c = get_color(severity)
        chip_color = c["pil_chip"]
        label = c["ja"] or severity

        badge_size = int(18 * scale)
        draw.ellipse([margin, y, margin + badge_size, y + badge_size], fill=chip_color)
        num_text = str(idx)
        nb = draw.textbbox((0, 0), num_text, font=small_font)
        draw.text(
            (margin + (badge_size - (nb[2] - nb[0])) // 2, y + (badge_size - (nb[3] - nb[1])) // 2),
            num_text,
            font=small_font,
            fill=(255, 255, 255),
        )

        chip_pad = int(5 * scale // 2)
        tag_x = margin + badge_size + int(8 * scale)
        cb = draw.textbbox((0, 0), label, font=small_font)
        cw = cb[2] - cb[0] + chip_pad * 2
        ch = cb[3] - cb[1] + chip_pad * 2
        draw.rounded_rectangle([tag_x, y, tag_x + cw, y + ch], radius=int(3 * scale // 2), fill=chip_color)
        draw.text((tag_x + chip_pad, y + chip_pad), label, font=small_font, fill=(255, 255, 255))

        page_text = f"  ページ {page_num}"
        draw.text((tag_x + cw + int(8 * scale // 2), y + chip_pad), page_text, font=small_font, fill=(120, 120, 120))
        y += max(badge_size, ch) + int(6 * scale)

        text_x = margin + int(10 * scale)

        def draw_wrapped(text, font, line_h, fill):
            nonlocal y
            for line in wrap_text(draw, text, font, w - text_x - margin):
                if y > h - margin:
                    break
                draw.text((text_x, y), line, font=font, fill=fill)
                y += line_h

        if quote:
            draw_wrapped("原文: " + quote, body_font, body_line_h, (20, 20, 20))
        if withheld == "low_ocr_confidence":
            draw_wrapped(
                "書き換え案: 読み取りの信頼度が低いため出していません。元の書類を見て書き直してください。",
                small_font,
                small_line_h,
                (90, 70, 20),
            )
        elif rewrite:
            draw_wrapped("書き換え案（AIの提案。事実と見比べてください）: " + rewrite, body_font, body_line_h, (20, 20, 20))
        if message:
            draw_wrapped(message, body_font, body_line_h, (20, 20, 20))
        if suggestion and suggestion != rewrite:
            label = "補足: " if (quote or rewrite or withheld) else "改善案: "
            draw_wrapped(label + suggestion, small_font, small_line_h, (60, 100, 60))

        y += int(10 * scale)
        draw.line([(margin, y), (w - margin, y)], fill=(220, 220, 220), width=1)
        y += int(10 * scale)

    buffer = io.BytesIO()
    image.save(buffer, format="PNG")
    return buffer.getvalue()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--items", required=True)
    args = parser.parse_args()

    if not os.path.exists(args.input):
        raise SystemExit(f"input file not found: {args.input}")
    if not os.path.exists(args.items):
        raise SystemExit(f"items file not found: {args.items}")

    with open(args.items, "r", encoding="utf-8") as f:
        items = json.load(f)

    doc = fitz.open(args.input)
    font_path = resolve_japanese_font()

    if not font_path:
        # フォントが見つからない場合: 日本語レビューページはスキップし、
        # 矩形アノテーションのみ付与してPDFを出力する（graceful degradation）
        print(
            "WARNING: 日本語フォントが見つかりません。\n"
            "  - レビューサマリーページは生成されません（文字化け防止のため）。\n"
            "  - アノテーション（矩形・バッジ）のみ付与して出力します。\n"
            "  - 修正方法: ANNOTATION_FONT_PATH 環境変数にフォントパスを設定するか、\n"
            "    fonts-noto-cjk パッケージをインストールしてください。",
            file=sys.stderr,
        )
        if items:
            annotate_pages(doc, items)
        doc.save(args.output, garbage=4, deflate=True)
        doc.close()
        return

    if items:
        annotate_pages(doc, items)
        first_page = doc[0]
        page_width = first_page.rect.width
        page_height = first_page.rect.height
        new_page = doc.new_page(-1, width=page_width, height=page_height)
        review_image = render_review_page(items, page_width, page_height, font_path)
        new_page.insert_image(
            fitz.Rect(0, 0, page_width, page_height),
            stream=review_image,
            keep_proportion=False,
            overlay=True,
        )

    doc.save(args.output, garbage=4, deflate=True)
    doc.close()


if __name__ == "__main__":
    main()
