-- 集約した重複行と、付け替えた user_application_statuses.match_id は復元できないため戻さない。
-- session_id の NULL も、空文字と区別できないため戻さない（Go 側は常に空文字を書く）。
--
-- ロールバックする場合は先にアプリを旧リビジョンへ戻すこと。
-- 一意キー(000023)が無い状態で新コードを動かすと、ON DUPLICATE KEY UPDATE が衝突せず
-- 再計算ごとに重複行が増え続ける。
SELECT 1;
