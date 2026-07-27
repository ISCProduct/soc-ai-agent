FROM python:3.10-slim

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

COPY main.py /app/main.py
COPY models.py /app/models.py
COPY vector_store.py /app/vector_store.py
COPY training_api.py /app/training_api.py
COPY export_training_data.py /app/export_training_data.py
COPY routers/ /app/routers/
COPY services/ /app/services/

EXPOSE 9000

CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "9000"]
