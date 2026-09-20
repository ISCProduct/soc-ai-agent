-- おすすめ企業の取得 (FindTopMatchesByUserAndSession) から filesort を外す。
--
-- クエリ:
--   SELECT * FROM user_company_matches
--   WHERE user_id = ? AND session_id = ? ORDER BY match_score DESC LIMIT 10
--
-- 既存の idx_user_session / uniq_user_session_company は (user_id, session_id) までしか
-- 並びを持たないため、絞り込んだ後の全行を filesort していた。1セッションの行数は
-- 公開企業数と同じ（本番想定 2,500〜4,000 行）で、各行が match_reason TEXT を含む。
-- EXPLAIN の Extra が "Using filesort" から NULL になることを確認済み。
--
-- 冗長になる idx_user_session はここでは消さない。GORM のモデルタグ
-- (internal/models/company.go) が宣言しており、そちらと併せて直す必要があるため。
ALTER TABLE user_company_matches
  ADD INDEX idx_ucm_user_session_score (user_id, session_id, match_score DESC),
  ALGORITHM=INPLACE, LOCK=NONE;
