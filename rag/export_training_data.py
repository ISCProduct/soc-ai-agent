#!/usr/bin/env python3
"""
export_training_data.py
簡易トレーニングデータエクスポーター（PIIマスキング対応）

期待される入力（--input 省略時は stdin）:
- JSON 配列 of sessions
  [
    {
      "id": 123,
      "user_id": 10,
      "utterances": [
         {"role":"user", "text":"自己紹介をお願いします"},
         {"role":"ai", "text":"こんにちは。私は..."},
         ...
      ]
    },
    ...
  ]

出力（JSONL）: 2 形式をサポート
- openai_chat: {"messages": [{"role":"user","content":...}, ...], "metadata": {...}}
- openai_prompt: {"prompt": "質問文\n\n###\n\n", "completion": " 回答テキスト"}

用途: 上流から取得したセッション/発話を整形してファインチューニング/評価に使える形式へ変換する。
PII マスキング (--no-mask-pii で無効化可)
"""
from __future__ import annotations

import argparse
import json
import logging
import re
import sys
from typing import Any, Dict, Iterable, List, Optional

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("export_training_data")


def mask_pii(text: str) -> str:
    """簡易 PII マスキング:
    - Email -> <EMAIL>
    - URL -> <URL>
    - Phone -> <PHONE>
    - 氏名/名前フィールドの値 -> <PERSON>
    - 日本語の名前に続く敬称（太郎さん 等）は名前部分を <PERSON> に置換
    - 英語の姓名（Capitalized Name Name）を <PERSON> に置換
    """
    if not text:
        return text

    # Emails
    text = re.sub(r"[\w\.-]+@[\w\.-]+\.[A-Za-z]{2,}", "<EMAIL>", text)

    # URLs
    text = re.sub(r"https?://\S+", "<URL>", text)

    # Phone numbers (simple heuristic: international +81 or sequences of digits with separators)
    text = re.sub(r"\+?\d[\d\-\s()]{6,}\d", "<PHONE>", text)

    # 名前フィールド: 氏名: 山田太郎 -> 氏名: <PERSON>
    text = re.sub(r"((?:氏名|名前|Name|Full ?Name)[:：])\s*[^\n,。;]+", r"\1 <PERSON>", text)

    # 日本語の名前+敬称（例: 山田太郎さん -> <PERSON>さん）
    text = re.sub(r"([\u4E00-\u9FFF]{2,4})(さん|様|君|ちゃん)", r"<PERSON>\2", text)

    # 英語のフルネーム (例: John Doe) -> <PERSON>
    text = re.sub(r"\b[A-Z][a-z]{1,}(?:\s+[A-Z][a-z]{1,})+\b", "<PERSON>", text)

    # 単一の英語名（タイトルあり）: Dr. Smith -> Dr. <PERSON>
    text = re.sub(r"\b(Mr|Mrs|Ms|Dr|Prof)\.\s*[A-Z][a-z]+\b", lambda m: f"{m.group(1)}. <PERSON>", text)

    # simple email-like tokens inside <>
    text = re.sub(r"<mailto:[^>]+>", "<EMAIL>", text)

    return text


def apply_mask_to_session(session: Dict[str, Any]) -> None:
    # mask session-level named fields
    for key in ("name", "full_name", "氏名", "名前", "user_name"):
        if key in session and isinstance(session[key], str):
            session[key] = mask_pii(session[key])

    for u in session.get("utterances", []):
        if isinstance(u, dict) and "text" in u and isinstance(u["text"], str):
            u["text"] = mask_pii(u["text"])


def iter_examples_from_session(session: Dict[str, Any]) -> Iterable[Dict[str, Any]]:
    """セッションから単純な user->ai ペアを生成するジェネレータ。
    session は最小で `utterances` を含むこと。
    """
    utterances = session.get("utterances") or []
    # find pairs: user utterance followed by ai utterance
    for i in range(len(utterances) - 1):
        cur = utterances[i]
        nxt = utterances[i + 1]
        if cur.get("role") == "user" and nxt.get("role") in ("ai", "assistant"):
            yield {
                "prompt": cur.get("text", "").strip(),
                "completion": nxt.get("text", "").strip(),
                "session_id": session.get("id"),
            }


def to_openai_chat(session: Dict[str, Any]) -> Dict[str, Any]:
    messages: List[Dict[str, str]] = []
    for u in session.get("utterances", []):
        role = u.get("role", "user")
        if role == "ai":
            r = "assistant"
        elif role == "user":
            r = "user"
        else:
            r = role
        text = u.get("text", "").strip()
        if not text:
            continue
        messages.append({"role": r, "content": text})
    return {"messages": messages, "metadata": {"session_id": session.get("id")}}


def to_openai_prompt(example: Dict[str, Any]) -> Dict[str, Any]:
    # OpenAI fine-tune prompt-completion style: prompt must not include the completion
    prompt = example.get("prompt", "")
    completion = example.get("completion", "")
    # Ensure completion starts with a space per OpenAI fine-tune recommendations
    if not completion.startswith(" "):
        completion = " " + completion
    return {"prompt": prompt + "\n\n###\n\n", "completion": completion}


def main(argv: Optional[List[str]] = None) -> int:
    p = argparse.ArgumentParser(description="Export training data JSONL from sessions/utterances.")
    p.add_argument("--input", "-i", help="Input JSON file (array of sessions). If omitted reads stdin.")
    p.add_argument("--output", "-o", help="Output JSONL file. If omitted writes to stdout.")
    p.add_argument("--format", "-f", choices=["openai_chat", "openai_prompt"], default="openai_prompt",
                   help="Output format. openai_prompt produces prompt/completion pairs; openai_chat produces messages arrays.")
    p.add_argument("--min-pairs", type=int, default=1, help="最低出力ペア数の閾値（openai_prompt 時）")
    p.add_argument("--no-mask-pii", action="store_true", default=False, help="Disable PII masking (by default masking is enabled)")
    args = p.parse_args(argv)

    mask_enabled = not args.no_mask_pii

    if args.input:
        with open(args.input, "r", encoding="utf-8") as fh:
            data = json.load(fh)
    else:
        data = json.load(sys.stdin)

    if not isinstance(data, list):
        logger.error("input must be a JSON array of sessions")
        return 2

    out_fh = open(args.output, "w", encoding="utf-8") if args.output else sys.stdout

    total = 0
    for session in data:
        if mask_enabled:
            apply_mask_to_session(session)

        if args.format == "openai_chat":
            obj = to_openai_chat(session)
            # skip empty
            if not obj.get("messages"):
                continue
            out_fh.write(json.dumps(obj, ensure_ascii=False) + "\n")
            total += 1
        else:  # openai_prompt
            for ex in iter_examples_from_session(session):
                obj = to_openai_prompt(ex)
                out_fh.write(json.dumps(obj, ensure_ascii=False) + "\n")
                total += 1

    if args.output:
        out_fh.close()

    logger.info("wrote %d examples (mask_enabled=%s)", total, mask_enabled)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
