-- 書き出し済みのデータの削除（同意の取り消し・退会。#2168）も、書き出しと同じ監査ログに残す。
-- kind で書き出し（export）と削除（sweep）を分け、削除では消したファイルの数を残す。
-- 個人を特定するものは持たない（件数と出力先だけ）ので、退会時のパージの対象にしないのは同じ。
ALTER TABLE `training_scorer_export_runs`
  ADD COLUMN `kind` varchar(16) NOT NULL DEFAULT 'export' COMMENT 'export / sweep' AFTER `id`,
  ADD COLUMN `deleted_file_count` int NOT NULL DEFAULT 0 AFTER `session_count`;
