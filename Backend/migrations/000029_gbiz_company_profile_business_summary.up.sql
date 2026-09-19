-- gBizINFO が返す business_summary(事業概要) を保存できるようにする。
-- APIは以前から返していたが、対応カラムが無く取りこぼしていた。
-- 企業の主要事業(companies.main_business)に直結する情報で、これが埋まると
-- web_search を呼ばずに済む企業が増える(#1124)。
--
-- 「メディア事業\nインターネット広告事業\nゲーム事業」のように複数行で返る
-- ことがあるため TEXT にする。
ALTER TABLE g_biz_company_profiles
  ADD COLUMN business_summary TEXT NULL AFTER company_url;
