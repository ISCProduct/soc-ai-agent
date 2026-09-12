-- マッチング結果に (user_id, session_id, company_id) の一意制約を張る（#1166 / SOCAIAGENT-304）
--
-- CreateOrUpdateBatch は既存行を1件ずつ UPDATE していたため、再計算時に公開企業数ぶんの
-- クエリが走っていた。ON DUPLICATE KEY UPDATE による一括 upsert へ寄せるには、
-- 衝突を検出する一意キーが必要になる。重複行の集約は 000022 で済ませてある。
--
-- LOCK=NONE を明示して、COPY アルゴリズムへ黙ってフォールバックした場合に気づけるようにする。
ALTER TABLE `user_company_matches`
  ADD UNIQUE KEY `uniq_user_session_company` (`user_id`, `session_id`, `company_id`),
  ALGORITHM=INPLACE, LOCK=NONE;
