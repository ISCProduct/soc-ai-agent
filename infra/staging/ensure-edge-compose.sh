#!/bin/bash
# /opt/app/docker-compose.yml に edge nginx とログサイズ上限を idempotent に追加する。
# ponytail: user_data 再実行なしで既存 staging EC2 を最新構成へ寄せる。
set -euo pipefail

COMPOSE="${1:-/opt/app/docker-compose.yml}"

python3 - "$COMPOSE" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text()
changed = False

if "edge:" not in text:
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
    print("patched edge nginx into docker-compose.yml")
    changed = True
else:
    print("edge service already present")

# ログドライバのデフォルト(json-file/無制限)によるディスク逼迫(#no space left on device)防止。
# 各サービス直下の "restart: always" 行の直後に logging 参照を差し込む。
if "x-logging:" not in text:
    text = (
        "x-logging: &default-logging\n"
        "  driver: json-file\n"
        "  options:\n"
        '    max-size: "10m"\n'
        '    max-file: "3"\n\n'
    ) + text
    text = text.replace(
        "    restart: always\n",
        "    restart: always\n    logging: *default-logging\n",
    )
    print("patched log size limits into docker-compose.yml")
    changed = True
else:
    print("log size limits already present")

if changed:
    path.write_text(text)
else:
    print("no changes needed")
PY
