-- staff ロールを学生へ戻す。role は元が 'student' 既定のため、staff を student に戻す。
-- （このバックフィル以前に staff を手で設定していた運用は無い想定。）
UPDATE `users`
SET `role` = 'student', `updated_at` = NOW()
WHERE `role` = 'staff';
