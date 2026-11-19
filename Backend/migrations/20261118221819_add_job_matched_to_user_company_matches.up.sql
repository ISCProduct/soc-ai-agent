-- 本人が選んだ職種の募集がある企業か（SOCAIAGENT-670）。計算時に決め、推薦の「選んだ職種の募集あり」の表示に使う。
-- 加点だけで、除外はしない。0 は「該当なし、または職種を選んでいない」。
ALTER TABLE `user_company_matches`
  ADD COLUMN `job_matched` TINYINT(1) NOT NULL DEFAULT 0 AFTER `fit_mismatch_axes`;
