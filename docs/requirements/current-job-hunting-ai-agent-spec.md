# 現行仕様書（概要）

このファイルは git に入っていなかった。SOCAIAGENT-677 で、実装との差分が残っていた §17 だけを追跡対象にする。1〜16 節はここには写していない。概要が食い違うときは、OpenAPI・テスト・コード・`docs/wiki/` を正とする。

コードコメントの古い記述は、この課題では直さない。

## 17. 実装状況

未完の箇条書きではなく、実装済みと残る限界を分ける。残作業の正は Backlog の課題。

| 項目 | 実装済み | 残る限界 | Backlog | 根拠 |
| --- | --- | --- | --- | --- |
| ES長文 | 字数上限なしの改善文は `max_tokens` を渡さない。評価の段は半角カナでも再試行1回で収まる計算。字数上限ありの改善文は指定字数から予算を渡し、再試行後も上限なら 422 | 半角カナを実モデルで流した確認は未実施 |  | `rag/services/es_review.py`、`docs/wiki/rag-service.md` |
| スコア較正 | 応募時 `score_snapshot`、日次の `AutoCalibrateIfDue`、較正重みのマッチング反映 | スナップショットに合成後スコア・内訳・版が無く、選考遷移の履歴も無い。合成比率 0.5 は暫定 | [SOCAIAGENT-673](https://iwasaki-ac.backlog.com/view/SOCAIAGENT-673) | `docs/wiki/score-calibration.md`、`docs/wiki/scoring.md` §7 |
| マッチング順位 | #1124 でマッチ度を線形化し、高域への集中は解消 | 同点は残る。副キーは未実装。課金・プランは使わない | [SOCAIAGENT-674](https://iwasaki-ac.backlog.com/view/SOCAIAGENT-674) | `docs/wiki/scoring.md` |
| 契約・課金 | 課金状態をマッチ順位に使わない | Stripe 等の契約・課金は未実装（保留） | [SOCAIAGENT-57](https://iwasaki-ac.backlog.com/view/SOCAIAGENT-57) | 課題の保留 |
| 合成比率 | チャットと適性検査を 0.5 : 0.5 で合成。片方だけのときはその値を使う | 比率自体の較正は未実施 | [SOCAIAGENT-673](https://iwasaki-ac.backlog.com/view/SOCAIAGENT-673) | `assessment.ChatWeight` |
| 業界重み | 教員向け業界適性は企業マッチングと同じ式。未計測軸は中立50のまま | 初期値は仮置き。実績からの較正は未実施 |  | `docs/wiki/scoring.md` §2-2 |
| 組織機能 | 組織・所属・主要テーブルの `organization_id` | 組織切り替え、カタログのテナント化、複数組織所属、JWT の org claim は未実装。文書のずれは別課題 | [SOCAIAGENT-572](https://iwasaki-ac.backlog.com/view/SOCAIAGENT-572) | `docs/wiki/multitenancy.md` Phase 2 |
| 集合知同意 | 記録前に `allow_collective_insight` を見る。既定は参加 | プライバシーポリシーと学生向けの同意画面が無い | [SOCAIAGENT-672](https://iwasaki-ac.backlog.com/view/SOCAIAGENT-672) | `docs/wiki/data-privacy.md` |
| AI根拠照合 | 発話を流用していない根拠は捨てる | 述語だけ流用した捏造は通る |  | Issue #1566、`docs/wiki/scoring.md` §2-3 |
| ローカルAI縮退 | 呼ばれていなかったローカル評価器は削除済み。LLM が使えないときはスコアを書かない | 粗い代替スコアは置かない |  | #831、`docs/wiki/scoring.md` §2-3 |
| 面接動画の保持 | 退会後の削除対象に含まれる | 在学中の自動削除は無い。学校ごとの期間は未実装。`data-privacy.md` の未完チェックは、この課題では変えない | [SOCAIAGENT-671](https://iwasaki-ac.backlog.com/view/SOCAIAGENT-671) | `docs/wiki/data-privacy.md` |
