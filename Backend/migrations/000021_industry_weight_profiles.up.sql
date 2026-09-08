-- 業界ごとの適性プロファイル（#1027 / SOCAIAGENT-251）
--
-- 生徒の user_weight_scores(10カテゴリ) と突き合わせて「向いている業界」を出す。
-- カラム構成は company_weight_profiles と同じ10軸にしてあるので、
-- マッチ度の計算(matching.CalculateCategoryMatch)をそのまま再利用できる。
--
-- 未設定の業界は行を作らず、アプリ側で中立値50として扱う（PRD 非機能要件）。
-- 初期データの投入自体は本Issueのスコープ外のため、代表業界のみ暫定値を入れる。
--
-- 番号について:
-- develop の最新は 000018。000019 は #1201(カテゴリ正典化)、000020 は #1196(企業ユーザー復旧)が使う。
-- golang-migrate は適用済みより小さい番号を二度と実行しないため、
-- この 000021 が先に本番へ入ると 19/20 が永久に未適用になる。
-- **マージ順序は #1201 → #1197 → 本PR を厳守すること。**
-- #1201 に入っている連番チェックが、順序を違えた場合にCIで落とす。

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

-- 初期データはここでは入れない。
-- マイグレーションは SeedData より先に走る(cmd/server/main.go)ため、
-- 新規構築時は industries が空で INSERT...SELECT が 0 行になり、
-- エラーも出ないまま永久に空のテーブルが残る。
-- 投入は seed.go の seedIndustryWeightProfiles で行う(冪等)。
