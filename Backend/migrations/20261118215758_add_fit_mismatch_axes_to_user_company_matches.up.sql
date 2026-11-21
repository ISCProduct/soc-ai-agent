-- 推薦の「ずれる軸」（本人の傾向と企業の重視点が大きく離れた軸）の名前を、カンマ区切りで持つ（SOCAIAGENT-670）。
-- 軸ごとの企業間のばらつきを使って計算時に決めるため、表示時に再計算せず、ここから読む。空は「ずれる軸なし」。
ALTER TABLE `user_company_matches`
  ADD COLUMN `fit_mismatch_axes` VARCHAR(120) NOT NULL DEFAULT '' AFTER `matched_axis_count`;
