-- 企業名の読み仮名（name_reading）の出どころと、出典の URL。
-- 読み仮名の信頼度を区別するため（面接の読み上げ・音声認識の補助語は、公的データ・人が確認したものを優先したい）。
--   corporate_registry : 法人番号公表サイトのフリガナ（公的データ）
--   website            : 公式サイトの会社概要に書かれていた記載（AI で抜き出し、引用文を検証済み）
--   manual             : 人が入力・確認した値
--   空                 : 出どころ不明（この列を足す前から入っていた値）
ALTER TABLE `companies`
  ADD COLUMN `name_reading_source` VARCHAR(24) NOT NULL DEFAULT '' AFTER `name_reading`,
  ADD COLUMN `name_reading_source_url` VARCHAR(500) NOT NULL DEFAULT '' AFTER `name_reading_source`;
