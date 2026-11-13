-- 学習利用の同意の履歴（#2168 の一部、#2180 のレビュー）。
-- users.allow_training_use は現在値しか持たず、users.updated_at はほかの更新でも上書きされる。
-- 「いつ・どの文言の版に同意し、いつ取り消したか」を後から答えられるよう、操作のたびに1行残す。
-- 行は追記だけで、更新しない（updated_at を持たない）。
CREATE TABLE `training_consent_events` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  -- 操作後の値（1=同意、0=取り消し・同意しない）
  `allow_training_use` tinyint(1) NOT NULL,
  -- 画面に出していた同意の文言の版（models.TrainingConsentTextVersion）
  `consent_text_version` varchar(32) NOT NULL,
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  -- 本人の最後の操作を引く（GET /api/user/training-consent）。
  KEY `idx_training_consent_events_user_created` (`user_id`,`created_at`),
  -- 期間で引く（書き出し・監査で「この期間に取り消した人」を探す）。
  KEY `idx_training_consent_events_created_at` (`created_at`),
  -- 退会のパージ（hardDeleteUserData）では明示的に消す。CASCADE は消し忘れの保険。
  CONSTRAINT `fk_training_consent_events_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
