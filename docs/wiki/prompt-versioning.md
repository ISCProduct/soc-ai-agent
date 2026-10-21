# プロンプト版の記録

AI 呼び出しの `api_call_logs` に `prompt_stage`、`prompt_version`、`prompt_variant` を記録する。
質問生成は `ai_generated_questions.prompt_version`、AI 生成マッチング理由は
`user_company_matches.match_reason_prompt_version` にも実際に使用した本文の版を記録する。

工程別設定は `PROMPT_VERSION_<STAGE>` で指定でき、従来の `PROMPT_VERSION` は共通設定として
後方互換を維持する。標準設定の例は `.env.example`、開発用の例は `Backend/.env.example`、
staging / production の設定経路は `infra/terraform/environments/` を参照する。

現在実装済みの本文は全工程で `v2` のみである。未登録版は `v2` にフォールバックする。
異なる候補本文が存在しない状態で A/B 割当を行うと、同じ内容を別版として記録して評価を
誤るため、`PROMPT_AB_<STAGE>` は現在無効である。版ごとの本文選択と送信内容が異なることを
確認できる候補実装が追加された後に A/B 割当を有効化する。

質問生成には質問の簡略化も含む。簡略化の AI 呼び出しにも同じ工程・ユーザーによる割当を
付与し、最終的に提示される質問の生成履歴を追跡できるようにする。

`prompt_stage` は機能配賦用の `feature` とは別の分析軸である。質問生成は `chat_question`、
回答評価は `chat_answer_eval`、企業抽出は `company_parse` / `company_search` など複数機能に
対応する場合がある。
