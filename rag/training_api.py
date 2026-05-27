import json
from typing import List, Dict, Any

from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import StreamingResponse

from export_training_data import apply_mask_to_session, to_openai_chat, to_openai_prompt, iter_examples_from_session


def register(app: FastAPI) -> None:
    """Register training-related endpoints on the given FastAPI app."""

    @app.post("/training/export")
    async def export_training(request: Request, format: str = "openai_prompt", mask_pii: bool = True):
        """Accepts a JSON array of sessions and streams JSONL suitable for fine-tuning.

        Query parameters:
        - format: openai_prompt (prompt/completion) or openai_chat (messages array)
        - mask_pii: whether to apply PII masking (default true)
        """
        try:
            data = await request.json()
        except Exception:
            raise HTTPException(status_code=400, detail="invalid json body")

        if not isinstance(data, list):
            raise HTTPException(status_code=400, detail="expected a JSON array of sessions")

        def gen():
            for session in data:
                if mask_pii:
                    apply_mask_to_session(session)

                if format == "openai_chat":
                    obj = to_openai_chat(session)
                    if not obj.get("messages"):
                        continue
                    yield json.dumps(obj, ensure_ascii=False) + "\n"
                else:
                    for ex in iter_examples_from_session(session):
                        obj = to_openai_prompt(ex)
                        yield json.dumps(obj, ensure_ascii=False) + "\n"

        return StreamingResponse(gen(), media_type="application/jsonl")

    @app.post("/training/export_to_file")
    async def export_to_file(request: Request, output_filename: str = "data/train_export.jsonl", limit: int = 0, mask_pii: bool = True):
        """受け取ったセッション配列を学習用 JSONL としてサーバ上に保存するエンドポイント。

        body: JSON array of sessions (same format as /training/export)
        query:
          - output_filename: 保存先（相対パス、デフォルト rag/data/train_export.jsonl の想定）
          - limit: 最大出力件数（0=制限なし）
          - mask_pii: PII マスキング適用の有無

        注意: 本エンドポイントは一時的なデータエクスポート用です。運用では認証・アクセス制御・パス検証を必ず追加してください。
        """
        try:
            data = await request.json()
        except Exception:
            raise HTTPException(status_code=400, detail="invalid json body")

        if not isinstance(data, list):
            raise HTTPException(status_code=400, detail="expected a JSON array of sessions")

        # 安全対策: 出力先は rag ディレクトリ配下に限定する（簡易チェック）
        if ".." in output_filename or output_filename.startswith("/"):
            raise HTTPException(status_code=400, detail="invalid output filename")

        # import here to avoid circular imports at module load
        from export_training_data import apply_mask_to_session, iter_examples_from_session, to_openai_prompt
        from training.export_jsonl import to_training_record
        import os

        os.makedirs(os.path.dirname(output_filename), exist_ok=True)
        written = 0
        try:
            with open(output_filename, "w", encoding="utf-8") as fh:
                for session in data:
                    if mask_pii:
                        apply_mask_to_session(session)
                    for ex in iter_examples_from_session(session):
                        # ex は例 (prompt/completion 等) を示す辞書
                        rec = to_openai_prompt(ex)
                        # sanitize via training utilities
                        rec = to_training_record(rec)
                        fh.write(json.dumps(rec, ensure_ascii=False) + "\n")
                        written += 1
                        if limit and written >= limit:
                            break
                    if limit and written >= limit:
                        break
        except Exception as exc:
            raise HTTPException(status_code=500, detail=f"failed to write file: {exc}")

        return {"output": output_filename, "count": written}

