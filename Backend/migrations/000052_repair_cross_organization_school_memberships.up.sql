-- 学園をまたいだ担当校の割当を外す（#1675 の後片付け。000050 の admin_school_memberships 版）。
--
-- 背景: AddMember に学園一致チェックが無かったため、無制限管理者が他学園のユーザーを
-- 学校の担当に割り当てられた。割り当てられた側は ListSchoolsForAdmin でその学校を
-- 担当校として受け取り、学校とその学校の学生は学園が揃っているので、
-- 対象ユーザー側の学園一致チェックでは弾けない。
--
-- 読み取り側（ListSchoolsForAdmin）でも学園一致を要求するようにしたので、
-- 残った行は効かなくなっている。ただし「担当校一覧に出るのに何も見えない」状態は
-- 運用で紛らわしいため、行そのものを外す。
--
-- 適用前に件数を確認したいときは、同じ条件で SELECT する:
--   SELECT m.user_id, m.school_id, u.organization_id AS user_org, s.organization_id AS school_org
--     FROM admin_school_memberships m
--     JOIN users u   ON u.id = m.user_id
--     JOIN schools s ON s.id = m.school_id
--    WHERE u.organization_id <> s.organization_id;
DELETE m FROM `admin_school_memberships` m
  JOIN `users` u   ON u.id = m.user_id
  JOIN `schools` s ON s.id = m.school_id
 WHERE u.organization_id <> s.organization_id;
