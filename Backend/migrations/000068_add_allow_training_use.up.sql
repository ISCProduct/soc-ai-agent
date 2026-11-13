-- 面接データを採点モデルの学習に使うことへの同意（#2167、親 #1834）。
-- 初期値 0 の opt-in。同意した人の面接だけを学習データの書き出し（#2168）の対象にする。
ALTER TABLE `users`
  ADD COLUMN `allow_training_use` TINYINT(1) NOT NULL DEFAULT 0
    COMMENT '面接データを採点モデルの学習に使うことへの同意（opt-in）'
    AFTER `allow_scout_visibility`;
