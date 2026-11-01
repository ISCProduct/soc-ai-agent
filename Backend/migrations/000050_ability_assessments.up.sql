-- 適性検査 第3段: 能力検査（言語・非言語）（#1679 / SOCAIAGENT-433）
--
-- 性格検査（assessment_*）とはテーブルを分ける。共有して kind 列で分岐すると、
-- 次の2つが「分岐を間違えなければ守られる」程度の保証になる。どちらも漏れたときの
-- 損害が大きいので、スキーマと型で成立させる。
--   1. 能力検査の点数を user_weight_scores へ入れない（10カテゴリへ変換する方法が無い）
--   2. 正解（correct_choice）を受検中のレスポンスへ出さない
-- 分けたことで assessment_* 側に NULL 許容列が増えず、本番稼働中の性格検査に触らずに済む。
--
-- ability_templates:  運営標準のみ（学校独自は作らない。問題の自作コストが大きい）。
--                     time_limit_seconds は受検の制限時間。NULL は無制限。
-- ability_questions:  4択。correct_choice は 0 始まりの添字。section で言語/非言語を分ける。
--                     公開後に問題を変えるときは version を上げて行を足す（既存行は書き換えない）。
-- ability_sessions:   開始時に template_version と deadline_at を凍結する。
--                     締め切りはサーバだけが決める。クライアントのタイマーは表示用。
-- ability_answers:    UNIQUE (session_id, question_id) で自動保存の再送を冪等にする。
-- ability_results:    section 別の正答数と出題数。偏差値や順位は出さない（出し方を誤ると
--                     学生の自己認識を固定する）。

CREATE TABLE IF NOT EXISTS `ability_templates` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `slug` varchar(64) NOT NULL,
  `title` varchar(255) NOT NULL,
  `description` text NOT NULL,
  `version` int unsigned NOT NULL DEFAULT 1,
  `time_limit_seconds` int unsigned DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_ability_templates_slug` (`slug`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ability_questions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `template_id` bigint unsigned NOT NULL,
  `template_version` int unsigned NOT NULL,
  `position` int unsigned NOT NULL,
  `section` varchar(20) NOT NULL,
  `body` text NOT NULL,
  `choices` json NOT NULL,
  `correct_choice` tinyint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_ability_questions_position` (`template_id`, `template_version`, `position`),
  CONSTRAINT `fk_ability_questions_template` FOREIGN KEY (`template_id`) REFERENCES `ability_templates` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ability_sessions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` char(36) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `template_id` bigint unsigned NOT NULL,
  `template_version` int unsigned NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'in_progress',
  `deadline_at` datetime(3) DEFAULT NULL,
  `submitted_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  -- 受検中だけ template_id を持つ。提出済みは NULL なので一意制約から外れる。
  `active_template_id` bigint unsigned GENERATED ALWAYS AS (IF(`status` = 'in_progress', `template_id`, NULL)) STORED,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_ability_sessions_public_id` (`public_id`),
  -- 同じ検査の受検中は1人1件。複数タブ・並行POSTで二重に作られるのを DB で止める。
  UNIQUE KEY `uniq_ability_sessions_active` (`user_id`, `active_template_id`),
  KEY `idx_ability_sessions_user_template` (`user_id`, `template_id`, `status`),
  CONSTRAINT `fk_ability_sessions_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_ability_sessions_template` FOREIGN KEY (`template_id`) REFERENCES `ability_templates` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ability_answers` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `session_id` bigint unsigned NOT NULL,
  `question_id` bigint unsigned NOT NULL,
  `choice` tinyint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_ability_answers_session_question` (`session_id`, `question_id`),
  CONSTRAINT `fk_ability_answers_session` FOREIGN KEY (`session_id`) REFERENCES `ability_sessions` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_ability_answers_question` FOREIGN KEY (`question_id`) REFERENCES `ability_questions` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ability_results` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `session_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  -- section -> {"correct": n, "total": m}。user_weight_scores へは書かない。
  `section_scores` json NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_ability_results_session` (`session_id`),
  KEY `idx_ability_results_user` (`user_id`),
  CONSTRAINT `fk_ability_results_session` FOREIGN KEY (`session_id`) REFERENCES `ability_sessions` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 運営標準の能力検査（v1）。言語6問・非言語6問、制限15分。
-- 問題は自作。SPI・玉手箱等の既存問題は著作権上流用できない（SOCAIAGENT-433）。
-- この12問は動作確認と初期提供のための最小セットで、出題数と難易度の調整は別途行う。
INSERT INTO `ability_templates` (`slug`, `title`, `description`, `version`, `time_limit_seconds`, `created_at`, `updated_at`)
VALUES ('standard-ability-v1', '能力検査（言語・非言語）',
        '言語6問・非言語6問の基礎的な問題です。制限時間は15分で、時間になると自動的に締め切られます。結果は言語・非言語それぞれの正答数で表示し、企業へのおすすめ計算には使いません。',
        1, 900, NOW(3), NOW(3));

INSERT INTO `ability_questions` (`template_id`, `template_version`, `position`, `section`, `body`, `choices`, `correct_choice`, `created_at`)
SELECT t.`id`, 1, q.`position`, q.`section`, q.`body`, q.`choices`, q.`correct_choice`, NOW(3)
FROM `ability_templates` t
JOIN (
  SELECT 1 AS `position`, 'verbal' AS `section`,
         '「審議」の意味として最も適切なものはどれか。' AS `body`,
         JSON_ARRAY('物事をよく調べて話し合うこと', '決まったことを通知すること', '上位者の判断を仰ぐこと', '前例に従って処理すること') AS `choices`,
         0 AS `correct_choice`
  UNION ALL SELECT 2, 'verbal', '「割愛する」の本来の意味はどれか。',
         JSON_ARRAY('不要なものを捨てること', '惜しいと思うものをやむを得ず省くこと', '順序を入れ替えること', '短くまとめ直すこと'), 1
  UNION ALL SELECT 3, 'verbal', '次の文の空欄に入る語として最も適切なものはどれか。「彼の主張は____で、誰が聞いても同じ結論になる。」',
         JSON_ARRAY('曖昧', '難解', '明快', '多義的'), 2
  UNION ALL SELECT 4, 'verbal', '「普遍」の対義語はどれか。',
         JSON_ARRAY('一般', '共通', '特殊', '全体'), 2
  UNION ALL SELECT 5, 'verbal', '「他山の石」の意味として最も適切なものはどれか。',
         JSON_ARRAY('他人の失敗を自分の戒めとすること', '遠くにある価値を見落とすこと', '他人の成功をうらやむこと', '動かせない障害があること'), 0
  UNION ALL SELECT 6, 'verbal', '「すべてのAはBである」「すべてのBはCである」の2つが正しいとき、確実に言えることはどれか。',
         JSON_ARRAY('すべてのCはAである', 'すべてのAはCである', 'AとCには関係がない', 'あるAはCではない'), 1
  UNION ALL SELECT 7, 'numerical', '原価800円の商品に25%の利益を見込んで売価をつけると、売価はいくらか。',
         JSON_ARRAY('825円', '1000円', '1025円', '1200円'), 1
  UNION ALL SELECT 8, 'numerical', '定価2000円の品物を3割引で買うと、支払額はいくらか。',
         JSON_ARRAY('1400円', '1700円', '600円', '1340円'), 0
  UNION ALL SELECT 9, 'numerical', '12人で8日かかる仕事を6人で行うと、何日かかるか。作業の速さは全員同じとする。',
         JSON_ARRAY('4日', '10日', '16日', '24日'), 2
  UNION ALL SELECT 10, 'numerical', '生徒300人のうち42%が女子である。女子の人数は何人か。',
         JSON_ARRAY('116人', '126人', '132人', '142人'), 1
  UNION ALL SELECT 11, 'numerical', '時速60kmで走る車が45分間に進む距離は何kmか。',
         JSON_ARRAY('27km', '40km', '45km', '80km'), 2
  UNION ALL SELECT 12, 'numerical', '5人の中から2人を選ぶ組み合わせは何通りあるか。',
         JSON_ARRAY('10通り', '20通り', '15通り', '25通り'), 0
) q
WHERE t.`slug` = 'standard-ability-v1';
