# 工程ごとのモデルの二層化（#1526）

LLM を使う工程を「学生に見える判断」と「内部の定型処理」に分け、層ごとにモデルを差し替えられるようにしている。
品質が体験に直結する工程に上位のモデルを、定型処理に軽量のモデルを割り当て、**全体の単価を上げずに配分を変える**のが狙い。

## 工程の分類

| 層 | 工程 | 工程ごとの環境変数 | 今の既定 |
|---|---|---|---|
| 上位（`AI_MODEL_PREMIUM`） | 面接の応答 | `OPENAI_INTERVIEW_MODEL` | gpt-4o-mini |
| 上位 | 面接レポート | `INTERVIEW_REPORT_MODEL` | アプリの既定（`AI_TEXT_MODEL` → `OPENAI_MODEL` → gpt-4o-mini） |
| 上位 | チャット診断の回答の採点 | `AI_MODEL_SCORE_EVALUATION` | アプリの既定 |
| 上位 | マッチング理由 | `AI_MODEL_MATCHING_REASON` | アプリの既定 |
| 上位 | ES添削（RAG） | `OPENAI_ES_REVIEW_MODEL` | `OPENAI_CHAT_MODEL` → gpt-4o |
| 上位 | 履歴書レビューのレポート（RAG） | `OPENAI_REVIEW_MODEL` | gpt-4o-mini |
| 軽量（`AI_MODEL_LIGHT`） | 企業情報の抽出 | `OPENAI_COMPANY_EXTRACT_MODEL` | gpt-4o-mini |
| 軽量 | 企業情報の parse | `OPENAI_COMPANY_PARSE_MODEL` | gpt-4o-mini |
| 軽量 | チャットの要約 | `AI_MODEL_CHAT_SUMMARY` | アプリの既定 |
| 軽量 | 検索結果の要約・ヒント（RAG） | （なし） | `OPENAI_CHAT_MODEL` → gpt-4o-mini |
| 軽量 | ヒントの parse（RAG） | `OPENAI_HINTS_PARSE_MODEL` | gpt-4o-mini |

工程表は Go が `Backend/internal/aimodel`、RAG が `rag/services/model_tiers.py`。分類を変えるときは両方のテストも直す。

## モデルを決める順番

1. 工程ごとの環境変数（既存の名前をそのまま使う）
2. 層ごとの環境変数（`AI_MODEL_PREMIUM` / `AI_MODEL_LIGHT`）
3. 今までの既定

**何も設定しなければ今までと同じモデル**になる。導入しただけではコストは変わらない。
prod は工程ごとの環境変数をいくつか明示している（例: RAG の `OPENAI_HINTS_PARSE_MODEL`）。明示している工程は層の設定より優先されるので、
層で一括に切り替えたいときはその値も見直す。

Terraform では変数 `ai_model_premium` / `ai_model_light`（既定は空）で prod の Backend・rag-review と staging の `.env` に渡す。
staging は新規インスタンスの user_data にしか届かないので、稼働中のインスタンスは `/opt/app/.env` も手で更新する。

## 切り替える前に測る

層を切り替えるときは、[AI出力品質の評価ハーネス](../research/ai-eval/README.md)（aibench）で品質とコストの差を測ってから決める。
aibench の各評価対象は、本番と同じ工程表でモデルを決める（`-model` で上書きできる）。

```bash
cd Backend
# 今の既定で測る
go run ./cmd/aibench -target chat-answer -manifest ../docs/research/ai-eval/chat-answer.jsonl -out /tmp/before.json
# 上位の層を変えた場合を測る
AI_MODEL_PREMIUM=gpt-4o go run ./cmd/aibench -target chat-answer -manifest ../docs/research/ai-eval/chat-answer.jsonl -out /tmp/after.json
go run ./cmd/aibench -diff /tmp/before.json,/tmp/after.json
```

採否の基準:

- 上位の層を上げるなら、品質（弁別力・破損率・指示遵守率）が改善していること
- 総コストが現状より増えないこと。上位を上げる分は、軽量の層を下げる（SLM・より安いモデル）ことで相殺する。
  費用は `api_call_logs` の `feature` 別集計（運用手順書の 3.6）で前後を比べる

現時点では層の値はどちらも未設定（既定のまま）で、品質とコストの実測による採否の判断は未実施。
