-- 学生の質問（kind = 'question'）の親の行は消さずに閉じる。旧版の ListActiveByStudent は種類で除外しないので、
-- 閉じないと学生の質問が「先生からの案内」としてホームに出る（docs/wiki/consultations.md のロールバック）。
UPDATE teacher_student_guidances SET dismissed_at = CURRENT_TIMESTAMP WHERE kind = 'question' AND dismissed_at IS NULL;
DROP TABLE IF EXISTS guidance_messages;
