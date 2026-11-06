#!/usr/bin/env python3
import argparse
import contextlib
import io
import json
import os
import sys
import traceback

import fitz  # PyMuPDF


def parse_ocr_line(idx, line):
    """PaddleOCR の1行をブロックJSONにする。認識スコアは捨てない（#1942）。

    line は [bbox, (text, score)]。score が無い形式は confidence を付けない。
    """
    bbox, rec = line
    text = ""
    score = None
    if isinstance(rec, (list, tuple)) and len(rec) >= 2:
        text = "" if rec[0] is None else str(rec[0])
        score = rec[1]
    elif rec is not None:
        text = str(rec)
    x_coords = [p[0] for p in bbox]
    y_coords = [p[1] for p in bbox]
    block = {
        "block_index": idx,
        "text": text,
        "bbox": [min(x_coords), min(y_coords), max(x_coords), max(y_coords)],
    }
    if score is not None:
        block["confidence"] = float(score)
    return block


def ocr_page(ocr, pix):
    img_bytes = pix.tobytes("png")
    with contextlib.redirect_stdout(io.StringIO()):
        result = ocr.ocr(img_bytes, cls=True)
    blocks = []
    if not result or not result[0]:
        return blocks
    for idx, line in enumerate(result[0]):
        blocks.append(parse_ocr_line(idx, line))
    return blocks


def process_pdf(path, ocr):
    doc = fitz.open(path)
    pages = []
    for page_index in range(len(doc)):
        page = doc[page_index]
        pix = page.get_pixmap(dpi=150)
        blocks = ocr_page(ocr, pix)
        pages.append({
            "page_number": page_index + 1,
            "width": pix.width,
            "height": pix.height,
            "blocks": blocks,
        })
    return pages


def process_image(path, ocr):
    pix = fitz.Pixmap(path)
    blocks = ocr_page(ocr, pix)
    return [{
        "page_number": 1,
        "width": pix.width,
        "height": pix.height,
        "blocks": blocks,
    }]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True)
    parser.add_argument("--output", required=False)
    args = parser.parse_args()

    if not os.path.exists(args.input):
        print(f"input file not found: {args.input}", file=sys.stderr)
        sys.exit(1)

    try:
        from paddleocr import PaddleOCR
    except Exception as e:
        print(f"PaddleOCR import failed: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        sys.exit(1)

    try:
        with contextlib.redirect_stdout(io.StringIO()):
            ocr = PaddleOCR(use_angle_cls=True, lang="japan", show_log=False)
    except Exception as e:
        print(f"PaddleOCR initialization failed: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        sys.exit(1)

    try:
        ext = os.path.splitext(args.input)[1].lower()
        if ext in [".pdf"]:
            pages = process_pdf(args.input, ocr)
        else:
            pages = process_image(args.input, ocr)
    except Exception as e:
        print(f"OCR processing failed: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        sys.exit(1)

    payload = {"pages": pages}
    try:
        if args.output:
            with open(args.output, "w", encoding="utf-8") as f:
                json.dump(payload, f, ensure_ascii=False)
        else:
            print(json.dumps(payload, ensure_ascii=False))
    except Exception as e:
        print(f"failed to write output: {e}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
