CREATE TABLE teacher_student_guidances (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  student_user_id BIGINT UNSIGNED NOT NULL,
  teacher_user_id BIGINT UNSIGNED NOT NULL,
  kind VARCHAR(32) NOT NULL,
  message TEXT NOT NULL,
  suggested_industries JSON NULL,
  dismissed_at DATETIME NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_guidances_student_active (student_user_id, dismissed_at),
  KEY idx_guidances_teacher (teacher_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
