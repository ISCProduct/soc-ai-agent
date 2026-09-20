-- AI利用量を Provider 非依存の配賦軸つきで記録できるようにする（#1294 / DesignDoc §3.3）
--
-- 現状の api_call_logs は model / トークン / コストしか持たないため、
-- 「どの機能が」「誰の操作で」「どの学校に配賦される」コストなのかが分からない。
-- ローカルAI化の効果（OpenAI費用の減少とローカル処理の増加）を数値で示すには、
-- 機能別・組織別の内訳が要る。
--
-- 既存の provider / via_fallback は #1293 で追加済みのため、ここでは追加しない。
-- DesignDoc は fallback_used という名前で挙げているが、意味は via_fallback と同じで
-- 二重に持つと集計が割れるため既存列を使う。
--
-- 既存行は遡及できないので、feature='unknown' / 主体NULL のまま残す（DesignDoc §3.3）。
-- 「計測されていない経路」は feature='unknown' の件数として観測できる（§8）。
ALTER TABLE `api_call_logs`
  ADD COLUMN `feature` varchar(64) NOT NULL DEFAULT 'unknown'
    COMMENT '#1294 機能名 (interview_stt / interview_llm / interview_tts / es_review / company_search など)',
  -- 実行主体。バッチ経路は主体を持たないため NULL 許容。
  -- 外部キーは張らない。退会や組織削除で利用実績が消えると、過去分の集計が
  -- 後から変わってしまうため（監査ログと同じ扱い）。
  ADD COLUMN `user_id` bigint unsigned NULL
    COMMENT '#1294 実行主体のユーザー (バッチはNULL)',
  ADD COLUMN `organization_id` bigint unsigned NULL
    COMMENT '#1294 配賦先の学校/企業 (不明はNULL)',
  -- STT / TTS はトークンではなく音声の長さが課金単位になる。
  ADD COLUMN `audio_seconds` decimal(10,2) NOT NULL DEFAULT 0
    COMMENT '#1294 STT/TTSの音声秒数',
  -- ローカル化が体験(レイテンシ)に与える影響を見るため。
  ADD COLUMN `latency_ms` int NOT NULL DEFAULT 0
    COMMENT '#1294 AI呼び出しの所要ミリ秒',
  ADD COLUMN `cache_hit` tinyint(1) NOT NULL DEFAULT 0
    COMMENT '#1294 キャッシュヒットで外部呼び出しを回避したか',
  -- 主用途は日次/月次 × provider と、組織別の配賦。
  ADD KEY `idx_api_call_logs_called_at_provider` (`called_at`, `provider`),
  ADD KEY `idx_api_call_logs_org_called_at` (`organization_id`, `called_at`),
  ALGORITHM=INPLACE, LOCK=NONE;
