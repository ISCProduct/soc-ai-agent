-- 同期したリポジトリが、本人が作成者（owner）または共同編集者（collaborator）のものかを記録する（SOCAIAGENT-698）。
-- 所属組織の他人のリポジトリや、コントリビュートしただけのリポジトリは 0。
-- 既存の行は区別がわからないので 0 とし、次の同期で正しい値に置き換わる（ReplaceRepositories で全件入れ替え）。
ALTER TABLE git_hub_repos
  ADD COLUMN is_owner_or_collaborator TINYINT(1) NOT NULL DEFAULT 0,
  ALGORITHM=INPLACE, LOCK=NONE;
