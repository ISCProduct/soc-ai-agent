-- ES添削の結果を永続化する（#1531）。
--
-- これまで ES添削は RAG のレスポンスをそのままクライアントへ返すだけで、
-- どこにも保存していなかった。そのため画面を閉じると結果が消え、2回目の添削でも
-- 前回と比較できず、user_weight_scores にも反映されないのでマッチングにも
-- 教員向けの傾向分析にも出てこなかった。履歴書（resume_reviews）だけが
-- フライホイールに乗っていて、ES が外れている状態を解消する。
--
-- question は利用者が入力した設問文。種別（question_type）だけだと
-- 「学チカ」「ガクチカ」のように同じ設問が別扱いになり、前回との差分が取れない。
-- 設問文を正規化した question_key で同一設問をまとめる（アプリ側で正規化して入れる）。
--
-- スコアは RAG が返す1〜10の4項目をそのまま持つ。company_fit_score は
-- 企業名が指定されなかったときに NULL になる（0 ではない。0点と未評価は別物）。
CREATE TABLE `es_reviews` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  -- 配賦と越境防止のため、実行時点の所属学園を複製して持つ。
  `organization_id` bigint unsigned NOT NULL,
  `question` varchar(500) NOT NULL,
  -- 設問文を正規化したもの（空白除去・小文字化）。同一設問の突き合わせに使う。
  `question_key` varchar(255) NOT NULL,
  `question_type` varchar(50) NOT NULL,
  `company_name` varchar(255) DEFAULT NULL,
  `es_text` text NOT NULL,
  -- 本文の SHA-256。スコアを二重に積ませないための鍵（#1531）。
  -- 設問や種別は添削結果に影響しない任意入力なので、それらを鍵にすると
  -- 本文を変えずにラベルだけ変えて何度でも加点できてしまう。
  `es_text_hash` char(64) NOT NULL,
  `char_limit` int unsigned DEFAULT NULL,
  `char_limit_mode` varchar(20) DEFAULT NULL,
  `specificity_score` tinyint unsigned NOT NULL,
  `star_score` tinyint unsigned NOT NULL,
  `length_balance_score` tinyint unsigned NOT NULL,
  -- 企業名の指定が無い添削では評価されない。
  `company_fit_score` tinyint unsigned DEFAULT NULL,
  `feedback` text NOT NULL,
  `improved_text` text NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  -- 前回結果の引き当て（本人の同一設問を新しい順に1件）。
  KEY `idx_es_reviews_user_question` (`user_id`,`question_key`,`id`),
  -- 同じ本文を既に評価済みかの判定（本人の中で引く）。
  KEY `idx_es_reviews_user_text` (`user_id`,`es_text_hash`),
  -- 教員向けの履歴一覧（本人の全件を新しい順）。
  -- InnoDB の二次索引は末尾に主キーを含むので実質 (user_id) と等価だが、
  -- 「id の降順で引く」意図を読めるように明示しておく。
  KEY `idx_es_reviews_user_created` (`user_id`,`id`),
  CONSTRAINT `fk_es_reviews_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_es_reviews_organization` FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
