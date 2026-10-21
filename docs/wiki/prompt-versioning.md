# プロンプト版の記録

AI呼び出しの `api_call_logs` に `prompt_stage`、`prompt_version`、`prompt_variant` を記録する。
質問生成は `ai_generated_questions.prompt_version`、AI生成マッチング理由は
`user_company_matches.match_reason_prompt_version`、回答のLLMハイブリッド評価は
`user_weight_scores.prompt_version`、企業情報抽出は `companies.info_prompt_version` にも
実際に使った本文の版を記録する。
マッチング理由の版は `api_call_logs` と同じ素の版名（例: `v2`）で保持する。

現時点で実装済みの各工程の本文は `v2` のみである。未登録の版名は `v2` にフォールバックし、
`PROMPT_VERSION` は従来どおり全工程共通の設定として受け付けるが、登録済みの本文だけが有効になる。
工程別設定 `PROMPT_VERSION_<STAGE>` は共通設定より優先する。

候補版本文が存在しない状態でA/B割当を行うと、同じ本文を異なる版として記録して評価を誤るため、
`PROMPT_AB_<STAGE>` は現在無効である。異なる候補本文を追加し、版ごとの本文選択と評価が検証された後に
A/B割当を再導入する。

質問生成、AI評価スコア、企業情報、マッチング理由は、出力レコードにも版を保持する。
AIを使わないルール評価や選択肢回答は版を記録しない。マッチング再計算で新しいAI理由が
生成されなかった場合は、既存の版記録を空値で上書きしない。`prompt_stage` が付与されない
AI呼び出しは `unknown` として保存し、未計測経路を識別できるようにする。

`prompt_stage` は機能配賦用の `feature` とは異なる分析軸であり、1工程が複数の機能に対応する場合がある。
例: `question_generation` → `chat_question`、`score_evaluation` → `chat_answer_eval`、
`company_extraction` → `company_parse` など。
