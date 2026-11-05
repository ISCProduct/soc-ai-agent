-- #1942 の取り消し。
--
-- 失われるもの: 指摘に紐づく原文・置き換え文・「書き換えを出さなかった理由」、
-- OCR行の認識スコア。down 後は画面が従来の message / suggestion だけになる。
-- 列を戻してもこれらの本文は復元できない。
ALTER TABLE `resume_review_items`
  DROP COLUMN `rewrite_withheld`,
  DROP COLUMN `rewrite`,
  DROP COLUMN `quote`;

ALTER TABLE `resume_text_blocks`
  DROP COLUMN `confidence`;
