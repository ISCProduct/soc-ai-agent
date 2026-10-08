-- 面接レポートのスコア反映が二重に走らないようにするための印（#1512）。
--
-- レポート生成のフォールバック経路（Redis 障害時の in-process channel）は
-- プロセス内の map でしか重複排除しておらず、本番は backend が最大2タスクまで
-- スケールする。Redis 障害中に同じセッションのレポート生成が2タスクで走ると、
-- user_weight_scores の移動平均へスコアが二重に反映される。
--
-- NULL = まだ反映していない。反映する側が条件付き UPDATE で NULL から
-- 奪い合う形にして、勝った1つだけがスコアを書く。
-- 反映に失敗したら NULL へ戻すので、asynq のリトライは従来どおり動く。
ALTER TABLE interview_reports
  ADD COLUMN scores_applied_at DATETIME NULL DEFAULT NULL
  COMMENT '面接スコアをuser_weight_scoresへ反映した時刻。NULLなら未反映(#1512)';
