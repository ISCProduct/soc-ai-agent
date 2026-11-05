-- #1942 履歴書レビューに「原文」と「置き換え文」、OCR行の認識スコアを残す。
--
-- quote / rewrite: 画面と注釈PDFが「どの文を、どの文に置き換えるか」を出すため。
--   suggestion は直し方の説明のまま。既存行は NULL（画面は従来の改善案表示に倒す）。
-- rewrite_withheld: 書き換えを出さなかった理由。low_ocr_confidence は誤認識行。
-- confidence: PaddleOCR の行スコア（0〜1）。NULL は #1942 以前の行（信頼度不明）。
--   不明な行まで書き換え禁止にすると、過去に保存した本文から再レビューできなくなる。
ALTER TABLE `resume_review_items`
  ADD COLUMN `quote` text NULL AFTER `message`,
  ADD COLUMN `rewrite` text NULL AFTER `suggestion`,
  ADD COLUMN `rewrite_withheld` varchar(32) NULL AFTER `rewrite`;

ALTER TABLE `resume_text_blocks`
  ADD COLUMN `confidence` double NULL AFTER `b_box`;
