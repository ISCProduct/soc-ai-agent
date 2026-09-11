# Chroma データ移行（Persistent → Http）#585

旧 RAG コンテナ内の `/app/chroma_db`（PersistentClient）から、独立 `soc-ai-chroma`（HttpClient）へ移行する手順です。

## 前提

- `compose.yml` の `chroma` は volume `chroma_data` で永続化される（再作成しても消えない）
- 旧 RAG は volume 未マウントだったため、**再作成前に必ず退避**する

## 1. 退避（移行前バックアップ）

```sh
# 稼働中の旧 rag-review から
docker cp soc-ai-rag-review:/app/chroma_db ./backup/chroma_db_$(date +%Y%m%d)

# または検証時の退避先
# /tmp/soc-ai-chroma-backup/chroma_db
```

## 2. 新構成の起動

```sh
make rag-up
# chroma :8000 / rag-review :9000
curl -s http://localhost:9000/health
# → vector_store.ok=true, detail に chromadb http://chroma:8000
```

## 3. ドライラン → 本番移行

```sh
cd rag
# venv がある場合
.venv/bin/python scripts/migrate_persistent_to_http.py \
  --source /tmp/soc-ai-chroma-backup/chroma_db \
  --host 127.0.0.1 --port 8000 --dry-run

.venv/bin/python scripts/migrate_persistent_to_http.py \
  --source /tmp/soc-ai-chroma-backup/chroma_db \
  --host 127.0.0.1 --port 8000
```

旧コレクション名（`hints_______39722` 等）はメタデータ付きで

- `interview_hints` / `es_review` / `company_context`

へ集約されます。

## 4. 確認

```sh
curl -s http://localhost:9000/vector/status
# total_documents が増えていること

# 企業フィルタ（社名は RAG の sanitize 後キー。旧 ID 由来は legacyid39722 形式）
curl -sG 'http://localhost:9000/vector/status' --data-urlencode 'company=legacyid39722'
```

旧コレクション名から推測した企業キーは `legacy_id_39722` → sanitize 後 `legacyid39722` になります（`/vector/status` のクエリと同じ規則）。

## 5. ロールバック

Http 側が壊れた・移行失敗時:

1. `rag-review` を止め、`CHROMA_HOST` を空にする（Persistent フォールバック）
2. 退避した `chroma_db` を `RAG_CHROMA_DATA_DIR`（既定 `/app/chroma_db`）にマウントして再起動

```yaml
# 一時的な compose override 例
services:
  rag-review:
    environment:
      - CHROMA_HOST=
      - RAG_CHROMA_DATA_DIR=/data/chroma_db
    volumes:
      - ./backup/chroma_db_YYYYMMDD:/data/chroma_db
```

## 関連

- Issue #585
- `make rag-up` / #589
