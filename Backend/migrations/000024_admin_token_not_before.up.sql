-- 管理者トークンの個別失効ポイント（#1155 / SOCAIAGENT-297）
--
-- 管理者トークンは userID:email の静的な HMAC で、有効期限も発行時刻も持たず、
-- 失効手段が ADMIN_SECRET のローテーション（＝全管理者を巻き込む）しか無かった。
-- トークン側に発行時刻を持たせた上で、管理者1名単位の失効点をここに持つ。
--
-- 特定の管理者のトークンだけを失効させる手順:
--   UPDATE users SET admin_token_not_before = UTC_TIMESTAMP(6) WHERE email = '<対象の管理者>';
-- これより前に発行されたトークンは拒否され、対象の管理者が再ログイン
-- （または GET /api/auth/user による再同期）すると新しいトークンが発行される。
-- 他の管理者のトークンには影響しない。

ALTER TABLE `users`
  ADD COLUMN `admin_token_not_before` datetime(3) DEFAULT NULL
  COMMENT '#1155 これより前に発行された管理者トークンを失効させる（NULLは失効なし）';
