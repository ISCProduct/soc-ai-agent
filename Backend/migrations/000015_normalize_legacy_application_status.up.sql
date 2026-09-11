-- 旧選考ステータスを現行コードへ寄せる（#1084）
UPDATE `user_application_statuses`
SET `status` = 'interview_in_progress'
WHERE `status` = 'interview';

UPDATE `user_application_statuses`
SET `status` = 'withdrawn'
WHERE `status` = 'declined';
