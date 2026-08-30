-- withdrawn へ寄せた declined は判別できないため戻さない
UPDATE `user_application_statuses`
SET `status` = 'interview'
WHERE `status` = 'interview_in_progress';
