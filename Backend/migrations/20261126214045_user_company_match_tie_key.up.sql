-- おすすめ企業の並び（FindTopMatchesByUserAndSession）を、同点キーまで索引順にする（SOCAIAGENT-674）。
--
-- ORDER BY に CRC32(...) を書くと、idx_ucm_user_session_score
-- (user_id, session_id, match_score) の後ろで filesort になる。
-- 1セッションは本番想定で 2,500〜4,000 行あり、各行が match_reason TEXT を持つ。
-- 生成列にして副キーまで索引に載せ、filesort を避ける。
--
-- 式は Go の matchrank.TieKey と同じ CRC32(CONCAT(user_id, ':', company_id))。
-- 課金・プラン・広告費・掲載優先度は使わない。
-- 旧索引は新しい索引の先頭と同じなので、二重に持たない。
ALTER TABLE user_company_matches
  ADD COLUMN match_tie_key INT UNSIGNED
    GENERATED ALWAYS AS (CRC32(CONCAT(user_id, ':', company_id))) STORED,
  ADD INDEX idx_ucm_user_session_rank (
    user_id, session_id, match_score DESC, match_tie_key, company_id, id
  ),
  DROP INDEX idx_ucm_user_session_score;
