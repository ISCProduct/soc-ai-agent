-- 既存の教員（担当校 admin_school_memberships を持つユーザー）を職員ロールに昇格する（明示化）。
-- 教員の実体はこれまで「担当校を持つ管理者」という派生で表現されていたが、
-- role='staff' を正式な判定基盤にする。ログイン後にチャットを出さず /admin へ送る導線と、
-- 純粋な職員（is_admin=false）の表現に使う。
--
-- is_admin なユーザーもここで role='staff' になるが、release note の audience 判定は
-- is_admin を優先するため表示面は変わらない（audienceForRole）。
UPDATE `users`
SET `role` = 'staff', `updated_at` = NOW()
WHERE `id` IN (SELECT DISTINCT `user_id` FROM `admin_school_memberships`)
  AND `role` <> 'staff';
