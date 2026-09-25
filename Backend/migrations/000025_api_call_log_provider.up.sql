-- AIコールログに「どの推論先が処理したか」を記録する（#1293 / SOCAIAGENT-316）
--
-- api_call_logs は推論先に関係なく記録されるため、そのままでは
-- 「OpenAI への課金額」として使えなかった。実測で2つの壊れ方が確認された。
--
-- 1) ローカル推論（無料）の行が混ざる。しかも未知モデル名は gpt-4o 単価に
--    フォールバックするため、無料の推論が架空コストとして予算を食い潰す。
-- 2) 企業検索などの通常の OpenAI 利用と同じ財布になる。実データでは
--    2026-08 の合計が $57.99 で、既定の月次上限 $20 を恒久的に超過していた。
--    つまりローカル障害時にフォールバックが一度も発動しない。
--
-- provider で課金対象を切り分け、via_fallback でフォールバック専用予算を分離する。
-- 既存行は provider 不明・非フォールバックとして扱う（既定値のまま）。

ALTER TABLE `api_call_logs`
  ADD COLUMN `provider` varchar(32) NOT NULL DEFAULT ''
    COMMENT '#1293 推論先 (openai / local / 空=不明)',
  ADD COLUMN `via_fallback` tinyint(1) NOT NULL DEFAULT 0
    COMMENT '#1293 ローカル障害時のOpenAIフォールバックで処理されたか',
  ADD KEY `idx_api_call_logs_fallback_called_at` (`via_fallback`, `called_at`),
  ALGORITHM=INPLACE, LOCK=NONE;
