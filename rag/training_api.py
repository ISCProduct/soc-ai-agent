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
