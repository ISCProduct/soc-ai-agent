"""学習用データの書き出しエンドポイント（/training/export, /training/export_to_file）。

SOCAIAGENT-561 で次を直した。
- 本文を `await request.json()` + isinstance で手で検証していた → Pydantic（セッションの配列）で受ける
- 書き込み先の検証が「..」と先頭「/」を弾くだけで、`data/sub/../../x` 以外の抜け道（シンボリックリンク、
  Windows のドライブ指定など）や既存ファイルの上書きを止められなかった → 決まったディレクトリ
  （RAG_TRAINING_EXPORT_DIR、既定 rag/data）の直下の「ファイル名だけ」を受け、実パスがその中に
  収まることを確かめる。拡張子は .jsonl に限る

認証は main の内部トークンのミドルウェアが掛ける（/health 以外は X-Internal-Token が必要）。
"""
from __future__ import annotations

import json
import os
import re
import tempfile
from typing import Any, Dict, Iterator, List

from fastapi import Body, FastAPI, HTTPException
from fastapi.responses import StreamingResponse
from pydantic import BaseModel

from training.export_training_data import apply_mask_to_session, to_openai_prompt, to_outcome_example

# 書き出し先のファイル名。ディレクトリ区切りや「..」を含められない（パスではなく名前だけを受ける）
_EXPORT_FILENAME = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,99}\.jsonl$")

Session = Dict[str, Any]

# 1回の依頼で受けるセッション数と本文の大きさの上限（SOCAIAGENT-701）。超えたら 413。
# Backend の書き出し（/api/admin/training/export）は件数を limit で絞って渡す。
TRAINING_MAX_SESSIONS = 5000
TRAINING_MAX_BODY_BYTES = 20 * 1024 * 1024


class ExportToFileResult(BaseModel):
    output: str
    count: int


def export_dir() -> str:
    """書き出し先のディレクトリ（実パス）。RAG_TRAINING_EXPORT_DIR、既定は rag/data。"""
    default = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data")
    return os.path.realpath(os.getenv("RAG_TRAINING_EXPORT_DIR", "").strip() or default)


def resolve_export_path(filename: str) -> str:
    """ファイル名を書き出し先ディレクトリの中の実パスにする。外へ出るなら 400。"""
    if not _EXPORT_FILENAME.match(filename) or ".." in filename:
        raise HTTPException(
            status_code=400,
            detail="output_filename はディレクトリを含まない .jsonl のファイル名にしてください",
        )
    base = export_dir()
    path = os.path.realpath(os.path.join(base, filename))
    # シンボリックリンクで外を指している場合もここで止まる
    if os.path.commonpath([base, path]) != base or os.path.dirname(path) != base:
        raise HTTPException(status_code=400, detail="invalid output filename")
    return path


def _training_records(sessions: List[Session], mask_pii: bool) -> Iterator[Dict[str, Any]]:
    """セッションを学習用のレコードにする。教師ラベルは選考結果（application_status）だけを使う。"""
    for session in sessions:
        if mask_pii:
            apply_mask_to_session(session)
        example = to_outcome_example(session)
        if example is None:
            continue
        yield to_openai_prompt(example)


def _check_limits(sessions: List[Session]) -> None:
    """件数と本文の大きさの上限を確かめる。本文の大きさは受け取ったセッションを JSON にした長さで測る。"""
    if len(sessions) > TRAINING_MAX_SESSIONS:
        raise HTTPException(status_code=413, detail=f"セッションは {TRAINING_MAX_SESSIONS} 件以下にしてください")
    size = len(json.dumps(sessions, ensure_ascii=False).encode("utf-8"))
    if size > TRAINING_MAX_BODY_BYTES:
        raise HTTPException(status_code=413, detail="本文が大きすぎます")


def register(app: FastAPI) -> None:
    """Register training-related endpoints on the given FastAPI app."""

    @app.post("/training/export")
    def export_training(
            sessions: List[Session] = Body(..., description="セッションの配列"),
            mask_pii: bool = True,
    ) -> StreamingResponse:
        """セッションの配列を受け取り、ファインチューニング用の JSONL を返す。

        教師ラベルは選考結果(application_status)のみを使用する。チャットのAI発話・
        面接/ESの自動採点などAI生成物は蒸留を避けるため一切使用しない。
        """
        _check_limits(sessions)

        def gen() -> Iterator[str]:
            for record in _training_records(sessions, mask_pii):
                yield json.dumps(record, ensure_ascii=False) + "\n"

        return StreamingResponse(gen(), media_type="application/jsonl")

    @app.post("/training/export_to_file", response_model=ExportToFileResult)
    def export_to_file(
            sessions: List[Session] = Body(..., description="セッションの配列（/training/export と同じ形式）"),
            output_filename: str = "train_export.jsonl",
            limit: int = 0,
            mask_pii: bool = True,
    ) -> ExportToFileResult:
        """セッションの配列を学習用 JSONL として、書き出し先ディレクトリの直下に保存する。

        output_filename はファイル名だけ（ディレクトリは RAG_TRAINING_EXPORT_DIR で決まる）。
        limit は最大出力件数（0 = 制限なし）。
        """
        if limit < 0:
            raise HTTPException(status_code=400, detail="limit は 0 以上にしてください")
        _check_limits(sessions)
        path = resolve_export_path(output_filename)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        written = 0
        # 同じディレクトリの一時ファイルに書いてから置き換える。途中で失敗しても既存のファイルを壊さない。
        # 書く形は /training/export と同じ（to_openai_prompt の出力そのもの。SOCAIAGENT-701）
        fd, tmp_path = tempfile.mkstemp(dir=os.path.dirname(path), prefix=".export-", suffix=".tmp")
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as fh:
                for record in _training_records(sessions, mask_pii):
                    fh.write(json.dumps(record, ensure_ascii=False) + "\n")
                    written += 1
                    if limit and written >= limit:
                        break
            os.replace(tmp_path, path)
        except OSError as exc:
            # 例外の本文（パスなど）は返さない
            raise HTTPException(status_code=500, detail="failed to write file") from exc
        finally:
            if os.path.exists(tmp_path):
                os.remove(tmp_path)
        return ExportToFileResult(output=os.path.basename(path), count=written)
