#!/usr/bin/env python3
"""Markdown を Notion ページとして作成する（標準ライブラリのみ）。"""

from __future__ import annotations

import json
import os
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path

NOTION_VERSION = "2022-06-28"
MAX_BLOCKS_PER_REQUEST = 100
RICH_TEXT_LIMIT = 2000


def rich_text(content: str) -> list[dict]:
    if not content:
        return [{"type": "text", "text": {"content": " "}}]
    chunks: list[dict] = []
    while content:
        chunk = content[:RICH_TEXT_LIMIT]
        content = content[RICH_TEXT_LIMIT:]
        chunks.append({"type": "text", "text": {"content": chunk}})
    return chunks


def paragraph(text: str) -> dict:
    return {"object": "block", "type": "paragraph", "paragraph": {"rich_text": rich_text(text)}}


def heading(level: int, text: str) -> dict:
    key = f"heading_{level}"
    return {"object": "block", "type": key, key: {"rich_text": rich_text(text)}}


def code_block(text: str, language: str = "plain text") -> dict:
    return {
        "object": "block",
        "type": "code",
        "code": {"rich_text": rich_text(text), "language": language or "plain text"},
    }


def bulleted_item(text: str) -> dict:
    return {
        "object": "block",
        "type": "bulleted_list_item",
        "bulleted_list_item": {"rich_text": rich_text(text)},
    }


def todo_item(text: str, checked: bool) -> dict:
    return {
        "object": "block",
        "type": "to_do",
        "to_do": {"rich_text": rich_text(text), "checked": checked},
    }


def divider() -> dict:
    return {"object": "block", "type": "divider", "divider": {}}


def markdown_to_blocks(markdown: str) -> list[dict]:
    lines = markdown.splitlines()
    blocks: list[dict] = []
    i = 0

    while i < len(lines):
        line = lines[i]

        if line.strip() == "---":
            blocks.append(divider())
            i += 1
            continue

        if line.startswith("```"):
            lang = line[3:].strip() or "plain text"
            code_lines: list[str] = []
            i += 1
            while i < len(lines) and not lines[i].startswith("```"):
                code_lines.append(lines[i])
                i += 1
            blocks.append(code_block("\n".join(code_lines), lang))
            i += 1
            continue

        if re.match(r"^#{1,3}\s+", line):
            level = len(line) - len(line.lstrip("#"))
            text = line[level:].strip()
            blocks.append(heading(min(level, 3), text))
            i += 1
            continue

        todo_match = re.match(r"^-\s+\[([ xX])\]\s+(.*)$", line)
        if todo_match:
            checked = todo_match.group(1).lower() == "x"
            blocks.append(todo_item(todo_match.group(2), checked))
            i += 1
            continue

        if line.startswith("- "):
            blocks.append(bulleted_item(line[2:].strip()))
            i += 1
            continue

        if line.startswith("|") and i + 1 < len(lines) and re.match(r"^\|[-: |]+\|$", lines[i + 1]):
            table_lines = [line]
            i += 2
            while i < len(lines) and lines[i].startswith("|"):
                table_lines.append(lines[i])
                i += 1
            blocks.append(code_block("\n".join(table_lines), "markdown"))
            continue

        if line.startswith(">"):
            blocks.append(paragraph(line.lstrip("> ").strip()))
            i += 1
            continue

        if not line.strip():
            i += 1
            continue

        blocks.append(paragraph(line.strip()))
        i += 1

    return blocks


def notion_request(method: str, path: str, token: str, payload: dict | None = None) -> dict:
    data = None if payload is None else json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        f"https://api.notion.com/v1{path}",
        data=data,
        method=method,
        headers={
            "Authorization": f"Bearer {token}",
            "Notion-Version": NOTION_VERSION,
            "Content-Type": "application/json",
        },
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode("utf-8"))


def append_blocks(page_id: str, token: str, blocks: list[dict]) -> None:
    for start in range(0, len(blocks), MAX_BLOCKS_PER_REQUEST):
        chunk = blocks[start : start + MAX_BLOCKS_PER_REQUEST]
        notion_request("PATCH", f"/blocks/{page_id}/children", token, {"children": chunk})


def create_page(parent_page_id: str, token: str, title: str, blocks: list[dict]) -> dict:
    first_chunk = blocks[:MAX_BLOCKS_PER_REQUEST]
    rest = blocks[MAX_BLOCKS_PER_REQUEST:]
    page = notion_request(
        "POST",
        "/pages",
        token,
        {
            "parent": {"type": "page_id", "page_id": parent_page_id},
            "properties": {
                "title": {"title": rich_text(title)},
            },
            "children": first_chunk,
        },
    )
    if rest:
        append_blocks(page["id"], token, rest)
    return page


def main() -> int:
    if len(sys.argv) < 3:
        print(
            "Usage: NOTION_API_KEY=... NOTION_PARENT_PAGE_ID=... "
            "python3 scripts/notion/publish_page.py <markdown-file> <page-title>",
            file=sys.stderr,
        )
        return 1

    token = os.environ.get("NOTION_API_KEY") or os.environ.get("NOTION_TOKEN")
    parent_id = os.environ.get("NOTION_PARENT_PAGE_ID")
    if not token or not parent_id:
        print("Error: NOTION_API_KEY (or NOTION_TOKEN) and NOTION_PARENT_PAGE_ID are required.", file=sys.stderr)
        return 1

    md_path = Path(sys.argv[1])
    title = sys.argv[2]
    markdown = md_path.read_text(encoding="utf-8")
    blocks = markdown_to_blocks(markdown)

    try:
        page = create_page(parent_id, token, title, blocks)
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        print(f"Notion API error ({exc.code}): {body}", file=sys.stderr)
        return 1

    print(page.get("url") or page.get("id"))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
