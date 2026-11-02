# プロンプト版の記録

AI呼び出しの `api_call_logs` に `prompt_stage`、`prompt_version`、`prompt_variant` を記録する。
質問生成は `ai_generated_questions.prompt_version`、AI生成マッチング理由は
`user_company_matches.match_reason_prompt_version`、回答のLLMハイブリッド評価は
`user_weight_scores.prompt_version`、企業情報抽出は `companies.info_prompt_version` にも
実際に使った本文の版を記録する。
マッチング理由の版は `api_call_logs` と同じ素の版名（例: `v2`）で保持する。

`PROMPT_VERSION_<STAGE>` は工程別の版ラベル、`PROMPT_VERSION` は後方互換の共通ラベルである。
工程別設定が共通設定に優先し、どちらも未設定なら `v2` を記録する。ラベルは英数字で始まり、
英数字・`.`・`_`・`-` のみ、最大64文字を指定できる。不正な工程別値は共通値へ、不正な共通値は
既定値へフォールバックする。

ラベルはデプロイ済みのプロンプト本文を識別する運用メタデータであり、環境変数だけで本文を切り替える
機能ではない。プロンプト本文を変更するときは対応する工程のラベルも同じリリースで更新する。
prod / staging の設定経路は `infra/terraform/environments/` を参照する。

異なる候補本文と本文選択経路が存在しないため、`PROMPT_AB_<STAGE>` によるA/B振り分けは未対応である。
現状の `prompt_variant` は `control` のみを記録する。候補本文の導入時に、本文選択・割当・出力記録の
一貫性をテストしたうえでA/B振り分けを追加する。

質問生成、AI評価スコア、企業情報、マッチング理由は、出力レコードにも版を保持する。
AIを使わないルール評価や選択肢回答は版を記録しない。マッチング再計算で新しいAI理由が
生成されなかった場合は、既存の版記録を空値で上書きしない。`prompt_stage` が付与されない
AI呼び出しは `unknown` として保存し、未計測経路を識別できるようにする。

`prompt_stage` は機能配賦用の `feature` とは異なる分析軸であり、1工程が複数の機能に対応する場合がある。
例: `question_generation` → `chat_question`、`score_evaluation` → `chat_answer_eval`、
`company_extraction` → `company_parse` / `company_search`。未計測経路は `unknown` として記録する。
