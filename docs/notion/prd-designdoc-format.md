# SOC AI Agent — PRD / DesignDoc / 仕様書 フォーマット

`/issue` 実行時に Notion へ作成するドキュメントの正本。
**既存の【テンプレート】ページを優先**し、新規フォーマットを発明しない。

## Notion 配置

| 種別 | DB | data_source |
| --- | --- | --- |
| PRD | [PRD一覧](https://www.notion.so/f1d127bd13f94a3eb93fead33295ceda) | `collection://ddf4c444-8ea3-4801-9faa-3e08afb6b660` |
| DesignDoc | [DesignDoc一覧](https://www.notion.so/3e6d1367035349a89b97f2237d0c04d8) | `collection://d00a06cd-426e-4b0d-a4d5-efe7671ed1ea` |
| 仕様書 | [仕様書一覧](https://www.notion.so/c549dd81990e468bbbd75db7cfa569f5) | `collection://d2d48cc6-2a7d-45e6-a6d2-9ac794778fcb` |

テンプレートページ（Notion）:

- PRD: [【テンプレート】PRD](https://www.notion.so/3d56ce9f823a81fdb742d509445aef29)
- DesignDoc: [【テンプレート】DesignDoc](https://www.notion.so/3d56ce9f823a81bf94b0f0d44c6a8dc7)
- 仕様書: [【テンプレート】仕様書](https://www.notion.so/3d66ce9f823a812bbfbdd7ec0ffc6e88)（#1095 一体型を正とする）

役割:

| ドキュメント | 役割 |
| --- | --- |
| PRD | 詳細な What / Why |
| DesignDoc | 詳細な How |
| 仕様書 | 実装者向け一枚サマリ（PRD+DesignDoc の要約。詳細はリンクで辿る） |

## 規模（Lite / Standard / Full）

| 規模 | いつ使う | PRD | DesignDoc | 仕様書 |
| --- | --- | --- | --- | --- |
| **Lite** | バグ、文言、小さな修正 | §1,2,5,7,9 | §1,4,7,10 | 全見出し（短く） |
| **Standard** | 通常機能 | 全セクション | 全セクション（§5 は簡潔で可） | 全見出し |
| **Full** | 横断 dig / アーキ変更 | 全セクション＋判断軸・選択肢 | 全セクションを厚く | 全見出し（表・依存を厚く） |

## DB プロパティ

共通: `Name`, `種別`, `カテゴリ`, `Issue番号`, `GitHub`, `Backlogキー`

- PRD/DesignDoc 追加: `ステータス`, `規模` ほか（詳細は各 DB）
- 仕様書: 上記共通のみ（従来スキーマ）

---

## 仕様書テンプレート（一体型・#1095 準拠）

```markdown
## 概要
{{背景・目的・対象範囲。依存Issueがあれば明記}}

## PRD
### ユーザーストーリー
- **As a** …
- **I want** …
- **So that** …

### 受け入れ条件
- **Given** …, **When** …, **Then** …

### 非機能要件
### リスク・未解決事項

## DesignDoc
### アーキテクチャ概要
### データモデル
### 既存コードへのマッピング
### リスクとガードレール

## 参照
- GitHub Issue / PRD（詳細）/ DesignDoc（詳細）

## UI上の主な変更点
{{画面がある場合のみ。無ければ「該当なし」}}
```

---

## PRD テンプレート（What / Why）

```markdown
## 1. 問題 / 背景
{{誰が困っているか。事実と仮説を分ける}}

## 2. 目的・成功指標
- **目的:** {{1〜2文}}
- **成功指標 (KPI):** {{計測可能}}
- **非目標 (Non-goals):** {{やらないこと}}

## 3. 対象ユーザー / ペルソナ
| ロール | 使う画面 / 体験 | 価値 |
| --- | --- | --- |
| 学生 / 教員 / 学校管理者 / 企業 / システム管理者 | | |

## 4. スコープ
### 対象
### 対象外

## 5. ユーザーストーリー
- **As a** …
- **I want** …
- **So that** …

## 6. 要件
### 機能要件
### 受け入れ条件 (Given / When / Then)
### 非機能要件
- 性能 / セキュリティ・同意・テナンシー / コスト・AI / 可用性

## 7. 依存・制約
- 依存 Issue/PR
- フライホイール（スコア・マッチング）への影響
- 環境変数

## 8. ロールアウト
- 段階公開 / whats-new 掲載可否 / ロールバック

## 9. リスク・未解決事項

## 10. 参照
- GitHub Issue / DesignDoc / docs/wiki
```

プロダクト固有の観点（必ず意識）:

- ロール境界（学生・教員・admin・企業）
- 同意・個人情報（分析・スカウト等）
- フライホイール（`user_weight_scores` × 企業プロファイル）への副作用
- `/whats-new` に載せるかは「ユーザー体験がある変更」かで判断

---

## DesignDoc テンプレート（How）

```markdown
## 1. コンテキスト
- 対応 PRD / 要約 / 前提決定

## 2. 現状アーキテクチャ
（必要なら Mermaid）

## 3. 提案設計
### 3.1 コンポーネント責務（FE / BE / RAG）
### 3.2 API / インターフェース
### 3.3 データモデル / マイグレーション
- `Backend/migrations/` up/down 必須。GORM AutoMigrate 禁止
### 3.4 AI / プロンプト（該当時）

## 4. 既存コードへのマッピング
| 関心事 | ファイル / シンボル | 変更 |

## 5. 代替案と比較

## 6. セキュリティ / マルチテナンシー

## 7. テスト計画

## 8. ロールアウト / 移行 / ロールバック

## 9. 観測性

## 10. リスクとガードレール

## 11. 参照
```

プロダクト固有の観点:

- DDD 層（Controller → Service → Repository）を崩さない
- スキーマ変更は migrations のみ
- RAG は `constraints.txt` / Chroma 影響を明記
- AI 変更はプロンプト版・コスト・JSON 検証を書く

---

## `/issue` との関係

1. 規模を決める（不明なら Standard）
2. **ユーザー承認後**、PRD → DesignDoc → **仕様書** の順で Notion に作成
3. PRD ↔ DesignDoc を Relation で相互リンク。仕様書の参照に両方の URL を載せる
4. GitHub Issue を作成（または既存 Issue を更新）し、`## PRD` / `## DesignDoc` / `## 仕様書` は **URL のみ**
5. 3ページの `Issue番号` / `GitHub` / `Name` を書き戻す
