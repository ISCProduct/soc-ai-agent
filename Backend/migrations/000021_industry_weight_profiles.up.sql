-- 業界ごとの適性プロファイル（#1027 / SOCAIAGENT-251）
--
-- 生徒の user_weight_scores(10カテゴリ) と突き合わせて「向いている業界」を出す。
-- カラム構成は company_weight_profiles と同じ10軸にしてあるので、
-- マッチ度の計算(matching.CalculateCategoryMatch)をそのまま再利用できる。
--
-- 未設定の業界は行を作らず、アプリ側で中立値50として扱う（PRD 非機能要件）。
-- 初期データの投入自体は本Issueのスコープ外のため、代表業界のみ暫定値を入れる。
--
-- 注: 000019 は #1201/#1197 が競合中、000020 はその決着用に空けてある。

CREATE TABLE IF NOT EXISTS `industry_weight_profiles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `industry_id` bigint unsigned NOT NULL,
  `technical_orientation`  bigint NOT NULL DEFAULT 50,
  `teamwork_orientation`   bigint NOT NULL DEFAULT 50,
  `leadership_orientation` bigint NOT NULL DEFAULT 50,
  `creativity_orientation` bigint NOT NULL DEFAULT 50,
  `stability_orientation`  bigint NOT NULL DEFAULT 50,
  `growth_orientation`     bigint NOT NULL DEFAULT 50,
  `work_life_balance`      bigint NOT NULL DEFAULT 50,
  `challenge_seeking`      bigint NOT NULL DEFAULT 50,
  `detail_orientation`     bigint NOT NULL DEFAULT 50,
  `communication_skill`    bigint NOT NULL DEFAULT 50,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_industry_weight_profiles_industry` (`industry_id`),
  CONSTRAINT `fk_industry_weight_profiles_industry`
    FOREIGN KEY (`industry_id`) REFERENCES `industries` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 代表業界の暫定値。業務知見に基づく本格的な投入は別途行う（PRD スコープ外）。
-- industries.code で引くので、シードの採番に依存しない。
INSERT INTO `industry_weight_profiles`
  (`industry_id`, `technical_orientation`, `teamwork_orientation`, `leadership_orientation`,
   `creativity_orientation`, `stability_orientation`, `growth_orientation`,
   `work_life_balance`, `challenge_seeking`, `detail_orientation`, `communication_skill`,
   `created_at`, `updated_at`)
SELECT i.id, v.tech, v.team, v.ldr, v.crea, v.stab, v.grow, v.wlb, v.chal, v.det, v.comm, NOW(3), NOW(3)
FROM `industries` i
JOIN (
  SELECT 'IT'   AS code, 85 tech, 65 team, 55 ldr, 70 crea, 40 stab, 80 grow, 60 wlb, 75 chal, 60 det, 60 comm
  UNION ALL SELECT 'IT-SW',   90, 70, 55, 70, 35, 85, 60, 75, 70, 55
  UNION ALL SELECT 'IT-WEB',  80, 65, 55, 85, 35, 85, 65, 80, 55, 65
  UNION ALL SELECT 'MFG',     70, 75, 55, 50, 70, 55, 60, 45, 85, 55
  UNION ALL SELECT 'MFG-AUTO',75, 80, 55, 50, 70, 55, 55, 45, 90, 55
  UNION ALL SELECT 'MFG-ELEC',80, 70, 50, 55, 65, 60, 55, 50, 85, 50
  UNION ALL SELECT 'FIN',     55, 65, 60, 40, 85, 60, 55, 40, 90, 70
  UNION ALL SELECT 'FIN-BANK',50, 70, 60, 35, 90, 55, 55, 35, 90, 75
  UNION ALL SELECT 'FIN-INS', 50, 65, 60, 40, 85, 60, 55, 40, 85, 80
  UNION ALL SELECT 'CONS',    65, 70, 80, 70, 35, 85, 40, 85, 70, 90
  UNION ALL SELECT 'EDU',     45, 75, 65, 65, 70, 65, 65, 50, 65, 90
  UNION ALL SELECT 'MED',     55, 85, 55, 40, 80, 60, 50, 45, 90, 85
) v ON v.code = i.code
ON DUPLICATE KEY UPDATE `updated_at` = NOW(3);
