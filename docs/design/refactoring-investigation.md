# リファクタリング事前調査書

| 項目 | 内容 |
|------|------|
| 文書名 | SOC AI Agent コードベース規模・肥大化調査 |
| 作成日 | 2026-07-25 |
| 対象リポジトリ | `soc-ai-agent-mock` / `ISCProduct/soc-ai-agent` |
| 目的 | 大規模リファクタ実施前に、現状規模・ホットスポット・リスク・分割候補を事実ベースで整理する |
| 関連 | Issue #649（大規模リファクタ追跡） |

---

## 1. 結論（Executive Summary）

本コードベースは **小規模プロトタイプを超え、中規模プロダクト帯** にある。

- 主要コード（Backend / Frontend / RAG、依存除外）はおおよそ **9 万行 / 640 ファイル**
- 層構造（Controller → Service → Repository）は概ね維持されている
- 問題の中心は「全体の行数」より、**単一ファイルへの責務集中（God File）** と、**FE のテスト薄さ**
- 全面リライトは不要。触る頻度が高い巨大ファイルから **段階的分割** が妥当

**推奨スタンス:** いますぐ全域リファクタはしない。調査結果に基づき、優先度付きで「分割 PR」を切る。

---

## 2. 調査範囲と方法

### 2.1 対象

| 領域 | パス | 備考 |
|------|------|------|
| Backend | `Backend/` | Go、migrations 含む |
| Frontend | `frontend/` | `node_modules` / `.next` 除外 |
| RAG | `rag/` | `.venv` 除外 |

### 2.2 非対象

- `node_modules` / 仮想環境 / 生成物
- インフラ Terraform 一式の詳細設計レビュー（別途）
- 個別バグの根因調査

### 2.3 方法

- 拡張子 `.go` / `.ts` / `.tsx` / `.py` の行数集計
- 層別（services / controllers / app pages 等）の内訳
- 800 行超ファイルの抽出と責務密度の簡易指標（メソッド数、`useState` 数など）
- テストファイル数の比較（網羅率の精密計測ではない）

> 行数は 2026-07-25 時点のローカルワークツリー計測。ブランチ差分により前後する。

---

## 3. 規模サマリー

### 3.1 全体

| 領域 | ファイル数 | LOC（概算） | 比率 |
|------|-----------|-------------|------|
| Backend | 415 | 59,339 | 66% |
| Frontend | 206 | 26,997 | 30% |
| RAG | 17 | 3,637 | 4% |
| **合計** | **638** | **89,973** | 100% |

### 3.2 Backend 層別

| 層 | ファイル数 | LOC | 所見 |
|----|-----------|-----|------|
| services | 107 | 24,778 | **最大の肥大層**。業務ロジックと AI/外部連携が集中 |
| controllers | 36 | 7,179 | 一部 500 行超だがサービスほどではない |
| repositories | 43 | 4,187 | 比較的健全（最大でも 300 行弱） |
| models | 45 | 2,489 | seed 系が厚い |

Go service 実装ファイルは約 **53**、Go テストファイルは約 **100**（Backend 全体）。Backend はテスト資産が相対的に厚い。

### 3.3 Frontend 層別

| 層 | ファイル数 | LOC | 所見 |
|----|-----------|-----|------|
| app（App Router） | 132 | 17,655 | **page.tsx に UI・状態・API 呼び出しが集中** |
| components | 32 | 6,153 | チャット・相関図などが大きい |
| lib | 18 | 1,741 | 薄い（分割の受け皿になりうる） |

FE ページは約 **44**。FE の `*.test.ts(x)` は約 **9** と少なく、**規模に対してテストが薄い**。

---

## 4. アーキテクチャ上の現状評価

### 4.1 良い点

- Backend は DDD 風の層分けが文書化・実装の双方で継続されている
- Repository 層は比較的小さく、DB アクセスの境界は読みやすい
- migrations 運用方針（AutoMigrate 禁止）が明確
- FE は App Router + 機能ページ分割の骨格がある

### 4.2 懸念点

1. **Service 層の神クラス化**  
   面接・職務経歴・クロールなどが、セッション管理 / 外部 API / スコア / ファイル処理を同一ファイルに抱える。
2. **FE page の神コンポーネント化**  
   特に面接・結果ページは `useState` が極端に多く、画面状態機械が巨大。
3. **RAG `main.py` の一極集中**  
   ルーティングと処理が同居し、Backend/FE と同型の肥大パターン。
4. **テスト非対称**  
   Backend は厚い一方、FE 巨大ページに対する回帰テストが弱い → 分割時の安全網が不足。

---

## 5. ホットスポット（God File）

### 5.1 800 行超（優先監視）

| LOC | パス | 簡易指標 | 主な責務仮説 |
|-----|------|----------|--------------|
| 1939 | `frontend/app/interview/page.tsx` | useState≈49 / useEffect≈12 | 面接 UI・メディア・同意・レポート・状態遷移 |
| 1575 | `Backend/internal/services/resume_service.go` | methods≈27 | アップロード・PDF 正規化・レビュー・企業検証 |
| 1515 | `rag/main.py` | defs≈50 | HTTP API・RAG 処理の入口集約 |
| 1500 | `Backend/internal/services/interview_service.go` | methods≈31 | セッション・発話・ワーカー・管理一覧 |
| 1358 | `frontend/app/results/page.tsx` | useState≈21 | マッチング結果・応募・相関図導線 |
| 1252 | `Backend/internal/services/crawl_service.go` | methods≈26 | ソース管理・スケジューラ・実行 |
| 1022 | `frontend/components/mui-chat.tsx` | — | チャット UI 本体 |
| 1020 | `Backend/migrations/000001_init_schema.up.sql` | — | 初期スキーマ（分割対象外が基本） |
| 924 | `Backend/internal/services/github_service.go` | — | GitHub 連携 |
| 864 | `Backend/internal/services/chat_question_generator.go` | — | 質問生成 |
| 805 | `Backend/internal/services/answer_evaluator.go` | — | 回答評価 |

### 5.2 次点（500–800 行、早期警戒）

**Backend services:** `auth_service`, `chat_service`, `analysis_scoring_service`, `chat_answer_validator`, `gbizinfo_service` など。

**Frontend pages:** `resume`, `profile`, `company/[id]`, `admin/score-validation`, `schedule`, `es-rewrite` など。

**Frontend components:** `company-results`, `github-skills`, `job-agent-chat`, `Correlation-diagram`, `analysis-sidebar`。

---

## 6. リスク評価

| ID | リスク | 影響 | 発生しやすさ | コメント |
|----|--------|------|--------------|----------|
| R1 | 巨大ファイル変更による回帰 | 高 | 高 | 面接・結果・職務経歴はユーザー導線の中核 |
| R2 | リファクタ中の挙動差分（AI 出力・スコア） | 高 | 中 | 非決定的処理が多く、黄金出力比較が難しい |
| R3 | FE テスト不足による分割事故 | 高 | 高 | page 分割前に最低限のテスト／スモークが必要 |
| R4 | 一度に全域を触る PR | 高 | 中 | レビュー不能・切り戻し困難 |
| R5 | スキーマ／API 契約まで巻き込む | 高 | 低〜中 | 調査時点では **構造分割を優先し契約変更は別トラック** が安全 |

---

## 7. リファクタ方針（推奨・未実施）

### 7.1 原則

1. **挙動変更なし**（pure refactor）を既定とする
2. **1 PR = 1 ホットスポット（または明確に関連する小セット）**
3. 分割前に **特徴づけるテスト or 手動チェックリスト** を用意する
4. 公開 API / DB スキーマ変更は本トラックに混ぜない
5. 行数目標の目安: 新規・分割後ファイルは **おおむね 400–600 行以下**（厳密な上限ではない）

### 7.2 優先順位（案）

| 優先度 | 対象 | 分割の方向性（案） | 理由 |
|--------|------|-------------------|------|
| P0 | `frontend/app/interview/page.tsx` | hooks / メディア / 同意ダイアログ / レポート送信をコンポーネント＆hooks へ | 最大・変更頻度・状態数が多い |
| P0 | `Backend/internal/services/interview_service.go` | セッション生命周期 / 発話永続化 / worker / admin 照会をファイル分割 | FE と対になる中核ドメイン |
| P1 | `frontend/app/results/page.tsx` | データ取得・リスト・応募アクション・ナビを分離 | マッチング価値の中核 UI |
| P1 | `Backend/internal/services/resume_service.go` | Upload / Review / Annotated / Company validation | LOC 最大級のサービス |
| P2 | `crawl_service.go` / `github_service.go` | スケジューラと実行、外部 API クライアント境界 | 運用系・連携系の複雑性 |
| P2 | `rag/main.py` | router / service / store 境界の明確化 | エントリ一点集中の解消 |
| P3 | チャット周辺（`mui-chat`, question generator, evaluator） | 生成と評価の境界整理 | 相互依存が強いため後段でも可 |

### 7.3 明示的に「今はやらない」こと

- フレームワーク載せ替え（Next / Echo / LangChain 等）
- マイクロサービス分割
- ディレクトリ全面再編を伴うビッグバン移動
- スコア計算式やプロンプトの同時改修

---

## 8. 成功指標（リファクタ開始後に測る）

| 指標 | 現状（概算） | 目標イメージ |
|------|--------------|--------------|
| 800 行超のアプリコードファイル数 | 10（SQL 除く） | 段階的に半減 |
| 面接 page の LOC / useState 数 | ~1939 / ~49 | 分割後 page は薄いオーケストレーションに |
| FE ユニットテストファイル数 | ~9 | 分割対象ドメインごとに追加 |
| 分割 PR のレビュー可能行数 | — | 目安 400 行前後の差分を上限意識 |

---

## 9. 次アクション

1. 本調査書を関係者でレビューし、P0 対象（面接 FE / 面接 Service）の合意を取る
2. Issue #649 を親として、P0/P1 を子 Issue に分解する
3. 各子 Issue に「分割計画・テスト計画・非ゴール」を書く
4. **最初の PR は面接 FE か面接 Service のどちらか一方のみ** とする

---

## 10. 付録: 計測コマンド（再計測用）

```bash
# 依存を除いた主要コードの行数再計測（例）
python3 - <<'PY'
from pathlib import Path
skip = {'node_modules','.next','.venv','venv','__pycache__'}
exts = {'.go','.ts','.tsx','.py'}
# ... Path.rglob で集計
PY
```

詳細な対話用ビューは Cursor Canvas「リファクタ事前調査」も参照。
