#!/bin/bash
# /opt/app/docker-compose.yml に edge nginx を idempotent に追加する。
# ponytail: user_data 再実行なしで既存 staging EC2 を edge 構成へ寄せる。
set -euo pipefail

COMPOSE="${1:-/opt/app/docker-compose.yml}"

python3 - "$COMPOSE" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text()

if "edge:" in text:
    print("edge service already present")
    sys.exit(0)

text = text.replace(
    """  frontend:
    image:""",
    """  frontend:
    image:""",
)

# host 公開 3000 を internal expose に差し替え
text = text.replace(
    """    ports:
      - "3000:3000"
""",
    """    expose:
      - "3000"
""",
    1,
)

edge_block = """
  edge:
    image: nginx:1.27-alpine
    restart: always
    ports:
      - "3000:80"
    volumes:
      - ./nginx/staging-edge.conf:/etc/nginx/conf.d/default.conf:ro
      - ./nginx/static:/usr/share/nginx/html/static:ro
    depends_on:
      - frontend
"""

marker = "  rag-review:"
if marker not in text:
    sys.exit("unexpected docker-compose.yml shape")
text = text.replace(marker, edge_block + marker, 1)
path.write_text(text)
print("patched edge nginx into docker-compose.yml")
PY
