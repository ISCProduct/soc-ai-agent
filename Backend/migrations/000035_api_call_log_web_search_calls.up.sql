-- web_search ツールの呼び出し回数を記録する。
--
-- このツールはトークンとは別に1コール単位($10/1,000コール)で課金されるが、
-- calculateCost はトークン単価しか持っておらず、検索コストの85%を占める
-- ツール料が api_call_logs に一切現れていなかった。
--
-- モデル名からは判別できない。同じ gpt-4o-mini でも通常のチャットと
-- web_search が混ざるため、生の回数を持つ必要がある。
-- AudioSeconds / Characters と同じで、単価が変わっても後から再計算できる。
--
-- 既存行は 0 のまま。遡って埋めることはできるが(入力トークンが8,000を超える
-- gpt-4o-mini 行が web_search)、推測値を実測と同じ列に混ぜない。
ALTER TABLE `api_call_logs`
  ADD COLUMN `web_search_calls` INT NOT NULL DEFAULT 0 AFTER `cache_hit`;
