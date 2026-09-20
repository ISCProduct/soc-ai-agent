-- TTS の課金単位（文字数）を記録できるようにする（#1294 / DesignDoc §3.4）
--
-- STT は音声の長さ、TTS は文字数が課金単位で、どちらもトークンではない。
-- 000031 で audio_seconds を入れたが、文字数の置き場が無かった。
-- prompt_tokens へ入れるとトークン合計が実態と食い違うため、独立した列にする。
ALTER TABLE `api_call_logs`
  ADD COLUMN `characters` int NOT NULL DEFAULT 0
    COMMENT '#1294 TTSの入力文字数(rune数)。トークン課金でない経路の課金単位',
  ALGORITHM=INPLACE, LOCK=NONE;
