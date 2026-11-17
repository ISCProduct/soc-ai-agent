-- リポジトリ要約のキャッシュに、作ったときのプロンプトの版を持たせる（#2185。#2178 のレビュー）。
-- 版が今の repoSummaryPromptVersion と違う行は、force_refresh=false でも作り直し、一覧にも出さない。
-- 既存の行は版が空文字になり、どの版とも一致しないので、すべて作り直しの対象になる
-- （v3 以前のプロンプトでは README・リポジトリ名の注入で虚偽の実績が作られうるため）。
ALTER TABLE `git_hub_repo_summaries`
  ADD COLUMN `prompt_version` VARCHAR(32) NOT NULL DEFAULT ''
    COMMENT '要約を作ったプロンプトの版（空は版の記録より前に作られた行）'
    AFTER `achievement`;
