-- users.school_id のうち、学校の所属学園が本人の所属学園と違う行を外す（#1675 の後片付け）。
--
-- 背景: users.school_id はプロフィールの学校名（本人の自己申告）から
-- SchoolRepository.FindByName 経由で解決される。この解決が organization_id で絞られて
-- いなかったため、学生が学校名を他学園の校名に書き替えるだけで、その学校の school_id を
-- 自分に付けられた。解決側は同じ修正で塞いだが、塞ぐ前に入った行はデータとして残る。
--
-- この UPDATE が安全な理由:
--   * users.school_id を書くのは auth_profile.go と guest_promotion.go の2箇所だけで、
--     どちらも resolveSchoolID 経由。管理画面から学園をまたいで割り当てる経路は無い。
--     つまり学園をまたいだ school_id は、すべてこの不具合の産物である。
--   * users.organization_id と schools.organization_id はどちらも NOT NULL
--     （000004 で既存ユーザーは organization_id=1 に埋められている）。
--     NULL 比較で取りこぼす・巻き込むことはない。
--   * schools は UNIQUE KEY (organization_id, name) なので同名校が学園をまたいで存在しうる。
--     学校名が同じでも別学園なら別の学校であり、外すのが正しい。
--
-- school_name（自己申告の文字列）は残す。本人が入力した事実であり、
-- 消すと学校名での検索・突き合わせができなくなる。外すのは解決結果の school_id だけ。
--
-- 適用前に件数を確認したいときは、同じ条件で SELECT する:
--   SELECT u.id, u.organization_id, u.school_id, s.organization_id AS school_org
--     FROM users u JOIN schools s ON s.id = u.school_id
--    WHERE u.organization_id <> s.organization_id;
UPDATE `users` u
  JOIN `schools` s ON s.id = u.school_id
   SET u.school_id = NULL
 WHERE u.organization_id <> s.organization_id;
