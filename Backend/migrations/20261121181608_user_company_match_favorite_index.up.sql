-- お気に入り一覧は user_id と is_favorited で絞り、診断の新しさ (created_at) で1社にまとめる。
-- updated_at は再計算・閲覧・応募でも進むので、代表行の並びには使わない。
--
-- 1診断あたり 2,500〜4,000 行（000028 と同じ前提）。お気に入りだけに絞ったあと
-- filesort で created_at を並べないようにする。
ALTER TABLE user_company_matches
  ADD INDEX idx_ucm_user_favorited_created (user_id, is_favorited, created_at),
  ALGORITHM=INPLACE, LOCK=NONE;
