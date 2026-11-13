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
  -- DB の時計（NOW(3)）で入れる。API のタスクごとの時計のずれで並びが崩れないよう、アプリの時計は使わない。
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  -- 本人の最後の操作（id の最大）を引く（GET /api/user/training-consent）。
  -- InnoDB の副索引は主キーを含むので、(`user_id`) だけで「user_id ごとの id の最大」を索引から引ける。
  KEY `idx_training_consent_events_user` (`user_id`),
  -- 期間で引く（書き出し・監査で「この期間に取り消した人」を探す）。
  KEY `idx_training_consent_events_created_at` (`created_at`),
  -- 退会のパージ（hardDeleteUserData）では明示的に消す。CASCADE は消し忘れの保険。
  CONSTRAINT `fk_training_consent_events_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 履歴の記録を始める前から同意していた人（allow_training_use = 1）に、1行ずつ履歴を作る。
-- 作らないと、同意済みなのに「いつ・どの文言に同意したか」の記録が1行も無い人が残り、
-- 履歴だけを見て同意の有無を判断する処理（書き出し・監査）で漏れる。
-- いつ・どの文言に同意したかは分からないので、版は 'legacy-before-history'
-- （models.TrainingConsentLegacyTextVersion）、日時はこのマイグレーションの適用時刻にする。
-- 同意していない人（0）は初期値のままで、本人の操作が無いので行を作らない。
INSERT INTO `training_consent_events` (`user_id`, `allow_training_use`, `consent_text_version`, `created_at`)
SELECT `id`, 1, 'legacy-before-history', NOW(3)
FROM `users`
WHERE `allow_training_use` = 1
ORDER BY `id`;
