-- 小数第1位以下は ROUND で整数に丸められ、端数は失われる（元に戻せない情報損失）。
UPDATE user_weight_scores SET score = ROUND(score);
ALTER TABLE user_weight_scores MODIFY COLUMN score BIGINT DEFAULT 0;
