# プロンプト版の記録

AI呼び出しの `api_call_logs` に `prompt_stage`、`prompt_version` を記録する。
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

## 宣言済みの版と起動時の検証（SOCAIAGENT-661）

本文の無い版を環境変数に指定すると、本文は今のまま版ラベルだけが変わり、使用量ログの版が実際の本文を
指さなくなる。そこで、環境変数で指定できる版を `prompts.declaredPromptVersions`（工程ごと。今は全工程 `v2`）に
限り、Backend の起動時に `prompts.ValidateSettings()` で確かめる。

- 宣言されていない版・形式が不正な値なら、どの環境変数が不正かをすべて出して起動を止める。形式が不正な値は、
  別の設定（シークレットを含みうる）を誤って入れた可能性があるので、値そのものは出さない
- `PROMPT_VERSION` は、工程別の `PROMPT_VERSION_<STAGE>` が無い工程に効くので、その工程すべてで宣言済みかを確かめる
- A/B の候補の版（`RegisterCandidateBodies`）は `PROMPT_AB_<STAGE>` でだけ使い、`PROMPT_VERSION_<STAGE>` では選べない。
  選ぶと `Body()` は今の本文を返し、ラベルと本文がずれるため
- 本文を変えたら、その工程の版を `declaredPromptVersions` で書き換え（または足し）、同じリリースで環境変数も揃える
- 本番のデプロイは、マイグレーションの前に `/bin/promptcheck`（`cmd/promptcheck`）を新しいイメージとタスク定義の
  `PROMPT_VERSION*` で実行し、不正ならサービスを差し替えない。実行時の版の解決（`ResolvePromptVersion`）は変えていない

## ES 添削の版（SOCAIAGENT-661）

ES 添削のプロンプトは RAG が持ち、版も RAG の本文と一緒に管理する（`rag/services/es_review.py` の `_PROMPT_VERSION`）。
RAG は使用量（`usage.prompt_version`）で版を返し、Backend は `api_call_logs` に `prompt_stage='es_review'` と
その版を記録する。ES 添削（`es_review`）とリライト（`es_rewrite`）は同じプロンプトなので、どちらの記録にも入る。
版を返さない古い RAG や形式に合わない値のときは、版なしで使用量だけを記録する。

## 出力スキーマ（SOCAIAGENT-661）

回答妥当性チェックと回答品質評価の出力は、Go の構造体（`prompts.AnswerValidationOutput` /
`prompts.AnswerQualityOutput`）を正とし、プロンプトの出力形式はそこから作る。出力を読む側
（`chat.LLMAnswerEvaluation`、`parseAnswerValidity`）も同じ構造体を使う。キーを変えるときは構造体のタグを直し、
本文が変わるので工程の版も上げる。プロンプトのキーと構造体のずれ、ベンチ（aibench）の読み方とのずれはテストで検出する。
必須キーが欠けた出力の扱い（ゼロ値で続行）は変えていない。

## A/B 割当（#1976）

工程ごとに候補の版と比率を `PROMPT_AB_<STAGE>` で指定する（例: `PROMPT_AB_SCORE_EVALUATION=v2:50,v3:50`）。
比率は 1〜10000 の整数で、合計に対する割合で割り当てる。

- **割当:** 単位の ID とサーバー側の固定ソルト（`abSalt`）のハッシュで決める。同じ ID は再起動やタスクをまたいでも同じ版になる。ソルトを変えると全員の割当が入れ替わる
- **実験中に設定を変えない:** 割当は比率の累積区間で決めるので、候補の記述順や比率を変えると既存の利用者の群が入れ替わる（`v2:50,v3:50` を `v3:50,v2:50` にすると全員が入れ替わり、50/50 を 70/30 にすると境界をまたぐ利用者が移る）。群が混ざると比較が成り立たないので、実験の途中で `PROMPT_AB_<STAGE>` を書き換えない。比率を変えたいときは、その時点で実験を区切り、集計も分ける
- **単位:** 回答評価は利用者（context の実行主体）で、利用者がいない呼び出しは対象外（今の版）。マッチング理由は対象の利用者（`match.UserID`）で、ワーカーやバッチから実行主体なしで走っても同じ割当になる。企業情報の抽出は企業 ID。企業名だけのプレビュー（`Acquire`）は企業 ID が無いので対象外
- **本文:** 今の版（`PROMPT_VERSION_<STAGE>` で決まるラベル）はコードにある今の本文を使う。候補の版の本文は `prompts.RegisterCandidateBodies` で登録する。1つの工程が複数の本文を使う場合（企業情報の抽出は検索用と事実テキスト用）は、すべてを同じ版で登録する
- **無効になる設定:** 候補が2つ未満、本文を登録していない版、比率の不正、版の重複、本文を差し替えられない工程（質問生成）。警告を1回出し、今の版のまま動く。本文の選択経路がないまま割当だけを入れると、同じ本文同士を比べることになるため
- **記録:** `api_call_logs.prompt_ab_group` に割り当てた版を残す。A/B の対象外の呼び出しは空。`prompt_version` と出力レコードの版（`user_weight_scores.prompt_version` など）にも、割り当てた版が入る

対象の工程は、回答評価（`score_evaluation`）、マッチング理由（`matching_reason`）、企業情報の抽出（`company_extraction`）。
質問生成（`question_generation`）は本文が多数のビルダーに分かれていて差し替える単位が無いので、対象外とする。

現時点で候補の本文は登録していない。候補の本文を足すまで、A/B を設定しても無効になる。

質問生成、AI評価スコア、企業情報、マッチング理由は、出力レコードにも版を保持する。
本番の回答評価で出力レコードに版が残るのは `EvaluateHumanScoringWithContext` の経路である。
`EvaluateWithLLMFallback` と `EvaluateHybrid` は本番から呼ばれておらず、結果に版を返さない。
これらを本番で使う場合は版を通す変更が必要で、それまでの品質比較は `api_call_logs` を正とする。
AIを使わないルール評価や選択肢回答は版を記録しない。マッチング再計算で新しいAI理由が
生成されなかった場合は、理由と一緒に版も空にする（テンプレートの理由に古い版を残さない）。`prompt_stage` が付与されない
AI呼び出しは `unknown` として保存し、未計測経路を識別できるようにする。

`prompt_stage` は機能配賦用の `feature` とは異なる分析軸であり、1工程が複数の機能に対応する場合がある。
例: `question_generation` → `chat_question`、`score_evaluation` → `chat_answer_eval`、
`company_extraction` → `company_parse` / `company_search`。未計測経路は `unknown` として記録する。
