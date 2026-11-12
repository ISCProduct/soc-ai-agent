-- 期限切れの仮登録を日次ジョブで消すため、expires_at にインデックスを張る（#2153）。
--
-- 削除は DELETE FROM pending_registrations WHERE expires_at <= ? ORDER BY expires_at LIMIT ?。
-- インデックスが無いと主キーを全件走査し、REPEATABLE READ では走査した行と隙間すべてにロックが掛かる。
-- その間、仮登録（新規登録・企業招待）の INSERT が待たされる。
-- インデックスがあれば、ロックは消す範囲の行に限られ、LIMIT で走査も早く打ち切れる。
ALTER TABLE `pending_registrations`
  ADD INDEX `idx_pending_registrations_expires_at` (`expires_at`),
  ALGORITHM=INPLACE, LOCK=NONE;
