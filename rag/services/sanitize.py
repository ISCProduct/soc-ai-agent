"""入力サニタイズヘルパー。"""
from __future__ import annotations

import re

from fastapi import HTTPException


def _sanitize_company_name_for_query(company_name: str) -> str:
    sanitized = re.sub(r"[^0-9A-Za-zぁ-んァ-ン一-龥ー々〆ヵヶ・\s]", "", company_name)
    sanitized = re.sub(r"\s+", " ", sanitized).strip()
    if not sanitized:
        raise HTTPException(status_code=400, detail="invalid company_name")
    return sanitized


def _sanitize_job_title(job_title: str) -> str:
    """職種名からプロンプトインジェクションに使われうる特殊文字を除去する"""
    sanitized = re.sub(r"[^\w\s\-（）()／/]", "", job_title, flags=re.UNICODE)
    sanitized = re.sub(r"\s+", " ", sanitized).strip()
    return sanitized or "指定なし"
