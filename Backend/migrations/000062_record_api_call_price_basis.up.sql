-- 単価の根拠（listed / estimated / inherited / unknown）を、記録した時点の値で残す（#2004）。
-- 集計時に今の単価表から判定すると、AI_MODEL_PRICING_JSON を変えて再起動したときに、
-- 過去の行まで新しい根拠で表示されてしまう。空文字は「記録なし」（この列を足す前の行、
-- 音声の単価で計算した行、ローカル推論の行）。
ALTER TABLE api_call_logs
    ADD COLUMN price_basis VARCHAR(16) NOT NULL DEFAULT '';
