FROM python:3.12-slim

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV PYTHONWARNINGS=ignore::FutureWarning
ENV PIP_DISABLE_PIP_VERSION_CHECK=1
ENV PIP_DEFAULT_TIMEOUT=120
ENV PIP_RETRIES=5

COPY requirements.txt /app/requirements.txt
COPY constraints.txt /app/constraints.txt
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --prefer-binary --timeout 120 --retries 5 -r /app/requirements.txt -c /app/constraints.txt
RUN python - <<'PY'
import sys
import openai
from openai import OpenAI
try:
    client = OpenAI(api_key="sk-dummy")
    _ = client.responses
    has_responses = True
except Exception:
    has_responses = False
if not has_responses:
    print("ERROR: OpenAI SDK missing responses API. Installed:", openai.__version__)
    sys.exit(1)
print("OpenAI SDK OK:", openai.__version__)
PY

# ソース一式をまとめて入れる。除外は .dockerignore 側で行う。
# ファイルを個別に列挙すると、モジュール追加時の追記漏れで本番が起動不能になる。
COPY . /app/

# 起動時と同じ import を通してビルドを落とす。COPY漏れ・依存漏れを本番のクラッシュ
# ループではなくビルド失敗として検出する(#1094 で student_profile_store.py が漏れた)。
RUN python -c "import main"

EXPOSE 9000

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "9000"]
