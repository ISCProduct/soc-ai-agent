-- SOCAIAGENT-513: 面接スコア等の小さな寄与が整数丸めで消えないよう score を小数化する。
-- 既存の整数値は 12 -> 12.00 のようにそのまま保たれる（範囲 0〜100 に収まる）。
ALTER TABLE user_weight_scores MODIFY COLUMN score DECIMAL(5,2) DEFAULT 0.00;
