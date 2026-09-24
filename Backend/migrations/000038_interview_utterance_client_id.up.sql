-- 発話保存(POST /utterances)を冪等にする(#1476)。
--
-- クライアントは保存失敗を再試行するが、失敗の中には「サーバーはDBへ書けたが
-- 応答だけ失われた」「8秒のクライアントタイムアウト直前に書き込みが完了した」
-- ケースが含まれる。追記APIのまま再試行すると同じ発言がもう一件入り、
-- 書き起こし・スコア(user_weight_scores)・LoRA学習データへ重複して伝播する。
--
-- クライアントが発話ごとに1つだけ発行する ID を受け取り、
-- (session_id, client_utterance_id) の一意制約で再送を弾く。
-- 結果が不明な失敗を「送り直して良い」ものにできるのはサーバー側だけなので、
-- ここで担保する（クライアントは自分の送信が届いたか判定できない）。
--
-- NULL 許容: 既存行と、ID を送らない経路（管理操作など）は従来どおり追記される。
-- MySQL の UNIQUE は NULL を重複とみなさないため、既存データの移行は不要。
ALTER TABLE `interview_utterances`
  ADD COLUMN `client_utterance_id` VARCHAR(64) DEFAULT NULL AFTER `text`,
  ADD UNIQUE KEY `uniq_interview_utterances_session_client` (`session_id`, `client_utterance_id`);
