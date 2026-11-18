-- 相談の意図（ホームの「キャリア相談」で選んだもの）を、セッションごとに1つ持つ（SOCAIAGENT-668）。
-- 値: find_job / learn_industry / how_to_proceed / organize_strengths / free。未選択は空文字。
-- 自由記述の本文は chat_messages に残るので、ここには複製しない（個人情報を二重に持たない）。
ALTER TABLE `conversation_contexts`
  ADD COLUMN `consult_intent` VARCHAR(30) NOT NULL DEFAULT '' AFTER `current_phase`;
