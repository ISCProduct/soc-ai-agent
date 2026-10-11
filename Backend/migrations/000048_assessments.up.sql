-- 適性検査 第1段: 性格検査と標準テンプレート（#1678）
--
-- assessment_templates: school_id が NULL なら運営標準。is_standard=1 のものだけマッチングへ合流させる。
-- assessment_questions: template_version ごとに問題を持つ。公開後に問題を変えるときは version を上げて
--   新しい行を足す（既存行は書き換えない）。受検中・受検済みの採点が揺れないようにするため。
-- assessment_sessions:  受検開始時の template_version を凍結保存する。
-- assessment_answers:   UNIQUE (session_id, question_id) で自動保存の再送を冪等にする。
-- assessment_results:   UNIQUE (session_id) で二重提出を DB でも止める。
-- user_weight_scores.source: chat / assessment。既存行は chat。

CREATE TABLE IF NOT EXISTS `assessment_templates` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `school_id` bigint unsigned DEFAULT NULL,
  `slug` varchar(64) NOT NULL,
  `title` varchar(255) NOT NULL,
  `description` text NOT NULL,
  `version` int unsigned NOT NULL DEFAULT 1,
  `is_standard` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_assessment_templates_slug` (`slug`),
  KEY `idx_assessment_templates_school_id` (`school_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `assessment_questions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `template_id` bigint unsigned NOT NULL,
  `template_version` int unsigned NOT NULL,
  `position` int unsigned NOT NULL,
  `body` text NOT NULL,
  `weight_category` varchar(100) NOT NULL,
  `reverse_scored` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_assessment_questions_position` (`template_id`, `template_version`, `position`),
  CONSTRAINT `fk_assessment_questions_template` FOREIGN KEY (`template_id`) REFERENCES `assessment_templates` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `assessment_sessions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` char(36) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `template_id` bigint unsigned NOT NULL,
  `template_version` int unsigned NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'in_progress',
  `submitted_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  -- 受検中だけ template_id を持つ。提出済みは NULL なので一意制約から外れる。
  `active_template_id` bigint unsigned GENERATED ALWAYS AS (IF(`status` = 'in_progress', `template_id`, NULL)) STORED,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_assessment_sessions_public_id` (`public_id`),
  -- 同じ検査の受検中は1人1件。複数タブ・並行POSTで二重に作られるのを DB で止める。
  UNIQUE KEY `uniq_assessment_sessions_active` (`user_id`, `active_template_id`),
  KEY `idx_assessment_sessions_user_template` (`user_id`, `template_id`, `status`),
  CONSTRAINT `fk_assessment_sessions_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_assessment_sessions_template` FOREIGN KEY (`template_id`) REFERENCES `assessment_templates` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `assessment_answers` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `session_id` bigint unsigned NOT NULL,
  `question_id` bigint unsigned NOT NULL,
  `value` tinyint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_assessment_answers_session_question` (`session_id`, `question_id`),
  CONSTRAINT `fk_assessment_answers_session` FOREIGN KEY (`session_id`) REFERENCES `assessment_sessions` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_assessment_answers_question` FOREIGN KEY (`question_id`) REFERENCES `assessment_questions` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `assessment_results` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `session_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `category_scores` json NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_assessment_results_session` (`session_id`),
  KEY `idx_assessment_results_user` (`user_id`),
  CONSTRAINT `fk_assessment_results_session` FOREIGN KEY (`session_id`) REFERENCES `assessment_sessions` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `user_weight_scores`
  ADD COLUMN `source` varchar(20) NOT NULL DEFAULT 'chat' AFTER `session_id`;

-- 運営標準の性格検査（v1）。10カテゴリ×2問、各カテゴリ1問を逆転項目にして黙従傾向を打ち消す。
INSERT INTO `assessment_templates` (`school_id`, `slug`, `title`, `description`, `version`, `is_standard`, `created_at`, `updated_at`)
VALUES (NULL, 'standard-personality-v1', '性格検査（標準）',
        '仕事の進め方や大切にしたいことについての20問です。正解・不正解はありません。今の自分に近いものを選んでください。',
        1, 1, NOW(3), NOW(3));

SET @tpl := (SELECT `id` FROM `assessment_templates` WHERE `slug` = 'standard-personality-v1');

INSERT INTO `assessment_questions` (`template_id`, `template_version`, `position`, `body`, `weight_category`, `reverse_scored`, `created_at`) VALUES
(@tpl, 1, 1,  '新しい技術や道具を自分で調べて試すのが好きだ', '技術志向', 0, NOW(3)),
(@tpl, 1, 2,  '仕組みがどう動いているかには、あまり興味がわかない', '技術志向', 1, NOW(3)),
(@tpl, 1, 3,  '一人で進めるより、仲間と分担して進めるほうが力を出せる', 'チームワーク志向', 0, NOW(3)),
(@tpl, 1, 4,  '人と協力するより、自分のペースで一人で進めたい', 'チームワーク志向', 1, NOW(3)),
(@tpl, 1, 5,  'グループでは、方向性を決めてまとめる役を引き受けることが多い', 'リーダーシップ志向', 0, NOW(3)),
(@tpl, 1, 6,  '人をまとめる役は、できれば他の人に任せたい', 'リーダーシップ志向', 1, NOW(3)),
(@tpl, 1, 7,  '決まったやり方より、自分なりの工夫やアイデアを出したい', '創造性志向', 0, NOW(3)),
(@tpl, 1, 8,  '新しい案を考えるより、決まった手順どおりに進めるほうが安心だ', '創造性志向', 1, NOW(3)),
(@tpl, 1, 9,  '変化の多い環境より、長く落ち着いて働ける環境を選びたい', '安定志向', 0, NOW(3)),
(@tpl, 1, 10, '環境や仕事内容がよく変わるほうが、わくわくする', '安定志向', 1, NOW(3)),
(@tpl, 1, 11, 'できることを増やすためなら、大変な仕事も引き受けたい', '成長志向', 0, NOW(3)),
(@tpl, 1, 12, '今できることを続けられれば、新しく学ぶ必要はあまり感じない', '成長志向', 1, NOW(3)),
(@tpl, 1, 13, '仕事と同じくらい、自分の時間や生活を大切にしたい', 'ワークライフバランス', 0, NOW(3)),
(@tpl, 1, 14, '成果のためなら、休日や夜の時間を使うのも苦にならない', 'ワークライフバランス', 1, NOW(3)),
(@tpl, 1, 15, '失敗するかもしれなくても、難しいことに挑戦したい', 'チャレンジ志向', 0, NOW(3)),
(@tpl, 1, 16, 'うまくいくか分からないことには、手を出さないほうだ', 'チャレンジ志向', 1, NOW(3)),
(@tpl, 1, 17, '細かい点の確認や見直しを、最後まで丁寧にやるほうだ', '細部志向', 0, NOW(3)),
(@tpl, 1, 18, '細かいところより、全体がだいたい合っていればよいと思う', '細部志向', 1, NOW(3)),
(@tpl, 1, 19, '初めて会う人とも、自分から話しかけて関係をつくれる', 'コミュニケーション力', 0, NOW(3)),
(@tpl, 1, 20, '考えを人に説明したり話し合ったりするのは苦手だ', 'コミュニケーション力', 1, NOW(3));
