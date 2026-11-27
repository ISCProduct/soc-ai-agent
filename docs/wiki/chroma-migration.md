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

## 企業データの再埋め込みと全件再構築（SOCAIAGENT-658）

### 企業単位の再埋め込み（`POST /vector/reembed`）

以前は「削除 → 再取得」の順で、Web 検索が例外・空だと企業データが空のまま残った。いまは doc_type ごとに
**再取得 → 新しく書く → 書けたことを確かめる → 旧文書を消す** の順（`vector_store.replace_documents`）。

- 再取得が例外・空の doc_type は旧データを残す。成功した doc_type だけ置き換える。結果は `results[]`（`replaced` / `unchanged` / `failed`）
- すべての doc_type が失敗したら 502（旧データはそのまま）。一部だけ失敗したら `status: partial`
- 置き換えのときは、同じコレクションの同じ企業（`company` と `company_hash`）の、今回書いた以外の文書を消す（以前の「全 role を消す」と同じ結果。ただし書けた後だけ）
- 同じ企業の再埋め込みは並行させない（2つ目は 409）
- `refresh=false` は今までどおり削除だけ

### 全件再構築（別の世代へ作って切り替える）

コレクションの名前に世代の接尾辞（`RAG_COLLECTION_SUFFIX`、空なら今までの名前）を付けられる。全件の再構築は、
新しい世代へ写してから切り替える。写している間も今の世代で検索できる。埋め込みは写すだけなので OpenAI は呼ばない。

```sh
# 1. 今の世代から g2 へ写す（距離関数は既定で cosine。l2 のままにするなら --space l2）
python scripts/rebuild_collections.py --to-suffix g2
# 2. 件数を確かめ、rag-review の環境変数 RAG_COLLECTION_SUFFIX=g2 にして入れ替える
# 3. 旧世代は 7日 残し（切り戻し用）、そのあと消す（接尾辞なしの世代は ""）
python scripts/rebuild_collections.py --drop-suffix ""
```

今使っている世代は `--drop-suffix` で消せない。学生プロフィール（`student_profiles`）は対象外（下の「距離関数」を参照）。

### 距離関数（cosine。SOCAIAGENT-554）

新しく作るコレクションは、企業データ・学生プロフィールとも距離関数を **cosine**（`hnsw:space`）にする（`vector_store.COLLECTION_METADATA`）。
指定しないと Chroma の既定の **l2**（ユークリッド距離の2乗）になる。

- Chroma はコレクションを作ったあとで距離関数を変えられない。既存の l2 のコレクションに cosine を指定して開いても、
  **指定は黙って無視され l2 のまま**になる（chromadb 0.6.3 で確認）。既存のものは下の手順で新しいコレクションへ写して切り替える
- OpenAI の埋め込みは長さ1なので、l2 と cosine で**検索の並び順は変わらない**（l2 の距離 = 2 × cosine 距離）。
  変わるのは距離の値で、距離で閾値を決めるところ（学生検索の関連度の下限など）は距離関数によらない値に直して使う
- 学生検索の関連度は `1/(1 + 2 × cosine 距離)`（`student_profile_store.relevance_score`）。l2 のコレクションでは以前の `1/(1+d)` と同じ値なので、
  切り替えの前後で関連度の下限（`STUDENT_SEARCH_MIN_SCORE`、既定 0.38）の意味は変わらない
- 今の距離関数と件数は `python scripts/rebuild_collections.py --status` で見る（書き込まない）

企業データは、上の「全件再構築」で新しい世代へ写して切り替える（既定で cosine）。
学生プロフィールは世代の接尾辞を持たないので、名前を入れ替える。

```sh
# 0. 学生プロフィールの書き込みを止める（rag-review を止め、スクリプトは別プロセスで動かす）
# 1. 確認（件数と今の距離関数。書き込まない）
python scripts/rebuild_collections.py --student-profiles
# 2. student_profiles__cos554 に cosine で写し、元を student_profiles__l2old554 に、写しを student_profiles に改名する
python scripts/rebuild_collections.py --student-profiles --apply
# 3. rag-review を起動して学生検索の疎通を確かめる。7日 後に旧いコレクションを消す
#    （旧いコレクションにも退会・同意撤回した学生の埋め込みが残るので、必ず消す）
python scripts/rebuild_collections.py --drop-student-old
```

切り戻し: `student_profiles` を消して（または別名にして）、`student_profiles__l2old554` を `student_profiles` に改名する。
staging で確かめてから本番で行う。
