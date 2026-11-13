-- アプリ内通知（#2033）。締切・選考結果・スカウト・教員の案内など、個人宛のイベントを1か所に集める。
-- dedupe_key は同じイベントの二重通知を防ぐ（例: 予定の前日通知を定期処理が何度走らせても1件）。
-- NULL のときは重複を見ない（MySQL の一意制約は NULL を重複とみなさない）。
CREATE TABLE notifications (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  kind VARCHAR(32) NOT NULL,
  title VARCHAR(200) NOT NULL,
  body VARCHAR(1000) NOT NULL DEFAULT '',
  link VARCHAR(500) NOT NULL DEFAULT '',
  dedupe_key VARCHAR(191) NULL,
  read_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_notifications_user_dedupe (user_id, dedupe_key),
  KEY idx_notifications_user_read (user_id, read_at),
  KEY idx_notifications_user_created (user_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
