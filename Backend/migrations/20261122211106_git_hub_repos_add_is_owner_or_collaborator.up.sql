-- 同期したリポジトリが、本人が作成者（owner）または共同編集者（collaborator）のものかを記録する（SOCAIAGENT-698）。
-- 所属組織の他人のリポジトリや、コントリビュートしただけのリポジトリは 0。
ALTER TABLE git_hub_repos
  ADD COLUMN is_owner_or_collaborator TINYINT(1) NOT NULL DEFAULT 0,
  ALGORITHM=INPLACE, LOCK=NONE;

-- 既存の行は本人のものとして扱う（オーナー決定 2026-11-22）。
-- 0 のままだと、既存の利用者は同期し直すまで要約を作れず、保存済みの要約も一覧から隠れる。
-- 次の同期で ReplaceRepositories が全件入れ替え、正しい値に置き換わる。
-- down は列ごと消すので、この UPDATE を戻す処理は要らない。
UPDATE git_hub_repos SET is_owner_or_collaborator = 1;
