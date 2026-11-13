-- 教員の案内への返信と、学生からの質問のスレッド（#2028）。
-- 案内（teacher_student_guidances）を1つのスレッドとし、その中の発言をここに積む。
-- 学生からの質問は、種類 question の案内（teacher_user_id = 0）として作り、最初の発言もここに入れる。
-- read_at は「相手側が読んだ日時」。学生の発言なら教員が、教員の発言なら学生が読んだ時刻。
CREATE TABLE guidance_messages (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  guidance_id BIGINT UNSIGNED NOT NULL,
  author_user_id BIGINT UNSIGNED NOT NULL,
  author_role VARCHAR(16) NOT NULL,
  body TEXT NOT NULL,
  read_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_guidance_messages_thread (guidance_id, id),
  KEY idx_guidance_messages_unread (guidance_id, author_role, read_at),
  KEY idx_guidance_messages_author (author_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
