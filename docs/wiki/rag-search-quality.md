# 検索品質の固定セット評価（#1773）

学生検索（企業向けスカウトのセマンティック検索）と企業リサーチ（キャッシュ済み文書の検索）の品質を、
固定のクエリと正解のセットで測る。埋め込みモデル・Chroma の設定・キャッシュの効かせ方を変えるときに、
変更の前後を同じ物差しで比べるために使う。検索アルゴリズムの改善そのものは対象外。

## 何を測るか

| 対象 | 通す経路（本番と同じ関数） | 固定セット |
|---|---|---|
| 学生検索 | `student_profile_store.upsert_student` → `query_students` | `rag/evaluation/search_quality/data/student_search.json`（学生20人・クエリ11件） |
| 企業リサーチ | `vector_store.set_cached_documents` → `get_cached_documents`（企業［＋企業ハッシュ］・職種の絞り込み、職種が無ければ同じ企業へのフォールバック、鮮度判定、上位5件） | `rag/evaluation/search_quality/data/company_research.json`（架空の5社・文書21件・クエリ12件） |

指標（クエリごとに計算してマクロ平均）:

- **Precision@k**: 上位 k 件のうち正解の割合。返った件数が k 未満でも分母は k
- **Recall@k**: 正解のうち上位 k 件に入った割合
- **MRR**: 最初の正解の順位の逆数
- **nDCG@k**: 2値の関連度での nDCG
- 企業リサーチの不変条件（**基準値と関係なく、0 でなければ失敗**）:
  - 他社の文書が混ざった件数（企業名またはサニタイズ前の企業名が違う文書）
  - 期限切れの文書が返った件数（固定セットの `stale: true` の文書は、取得時刻を期限切れにして保存する）
  - 上限（5件）を超えて返ったクエリの数（みなと商事の営業は6件置いてある）

k は学生検索が 1・3・5・10、企業リサーチが 1・3・5（本番が上位5件だけを使うため）。
企業リサーチは1社あたりの文書が少ないので、Precision@3・@5 は構造上低く出る。比較には Recall・MRR・nDCG を使う。

## 固定セットの作り方

- **匿名の合成データだけで作る。** 実在の学生・企業のデータは入れない。メールアドレス・電話番号・郵便番号らしき文字列があると読み込みで失敗する
- 学生プロフィールは本番の `ScoutProfileText`（`取得資格:` `希望業界:` `自己PR:` などの行）と同じ形にする
- 正解は「そのクエリの条件を満たす文書」を全部挙げる。複数条件のクエリ（例: リーダー経験があって React ができる）は、すべてを満たす文書だけを正解にする
- 企業リサーチは結果が本文で返るので、本文の重複は禁止（本文から文書 ID を引く）
- 表記違いで同じ名前にサニタイズされる企業は `company_original`（サニタイズ前の名前）で分ける。本番と同じく企業ハッシュ付きのキーで保存・検索する
- 期限切れの文書（`stale: true`）は正解にできない
- 個人情報の検査は本文だけでなく、ID・企業名・説明など全ての文字列にかかる

## 実行方法

```sh
cd rag

# CI と同じ（費用なし）。決定的なハッシュ埋め込みとメモリ上の Chroma 互換ストア
python scripts/eval_search_quality.py

# 本番と同じ埋め込み（OPENAI_API_KEY が必要・費用がかかる）と本物の Chroma（メモリ上）
python scripts/eval_search_quality.py --embedder openai --store chroma --output /tmp/before.json

# 設定を変えてもう一度測り、前の結果と比べる（下がった指標があれば終了コード 1）
OPENAI_EMBEDDING_MODEL=text-embedding-3-large \
  python scripts/eval_search_quality.py --embedder openai --store chroma --baseline /tmp/before.json
```

`--tolerance 0.02` のように許容する下げ幅も指定できる。`--output` にはクエリごとの順位も残るので、
どのクエリで順位が変わったかを確認できる。

## CI での扱い

`rag/tests/test_search_quality.py` が、ハッシュ埋め込みとメモリ上のストアで両方の経路を通し、
同梱の基準値（`data/baseline_hash_memory.json`）と一致することを確認する。結果は決定的なので許容幅は 0。

- **ハッシュ埋め込みは品質の絶対値の目安にはならない**（文字 bigram の一致を見るだけ）。CI で見ているのは、
  絞り込み・フォールバック・鮮度・件数・順位付けといった**検索経路のコードの回帰**である
- 検索経路を意図して変えて値が変わった場合は、変化を確認したうえで基準値を更新する:
  `python scripts/eval_search_quality.py --update-baseline`
- OpenAI の埋め込みでの評価は費用と API キーが要るので CI では行わず、設定を変える PR の作成者が手元で実行し、
  前後の表を PR に貼る

メモリ上のストアは Chroma の既定の距離（l2）に合わせている。本物の Chroma（`--store chroma`）と比べると、
距離がほぼ同じ文書の下位の並びが入れ替わることがある（Chroma は近似近傍探索と float32 を使うため）。
同梱のセットでは指標の平均は一致している。

## 関連

- [RAG サービス詳細](./rag-service.md)
- コスト削減で軽い経路に落とすとき（SOCAIAGENT-328 など）は、変更の前後でこの評価を実行する
