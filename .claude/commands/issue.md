## GitHub Workflow Commands

### 1. Issueの作成 (`/issue`)

ユーザーの指示から **PRD（要件）** と **DesignDoc（設計）** を生成し、**Notion に正本として作成**したうえで GitHub Issue を作る。
フォーマット正本: `docs/notion/prd-designdoc-format.md`

**Usage:** `/issue [実装したい機能や修正したいバグの概要]`

**Notion 宛先（このリポジトリ専用）:**

| | URL / ID |
| --- | --- |
| PRD DB | https://www.notion.so/f1d127bd13f94a3eb93fead33295ceda (`collection://ddf4c444-8ea3-4801-9faa-3e08afb6b660`) |
| DesignDoc DB | https://www.notion.so/3e6d1367035349a89b97f2237d0c04d8 (`collection://d00a06cd-426e-4b0d-a4d5-efe7671ed1ea`) |
| PRD テンプレート | https://www.notion.so/3d56ce9f823a81fdb742d509445aef29 |
| DesignDoc テンプレート | https://www.notion.so/3d56ce9f823a81bf94b0f0d44c6a8dc7 |

---

**Execution Workflow:**

1. **規模の決定**
   - `Lite` … バグ・文言・局所修正（PRD §1,2,5,7,9 / DesignDoc §1,4,7,10）
   - `Standard` … 通常機能（全セクション。DesignDoc §5 は簡潔で可）
   - `Full` … 横断・アーキ選択・コスト判断（全セクションを厚く）
   - 不明なら Standard。ユーザーが規模を指定していればそれに従う。

2. **不足情報の確認**
   - ターゲットロール（学生/教員/学校管理者/企業/システム管理者）、成功指標、スコープ外が曖昧なら質問する。
   - 自明なら前提を置き、PRD「リスク・未解決事項」に明記して進めてよい。

3. **PRD 生成（What / Why）**
   - `docs/notion/prd-designdoc-format.md` の PRD テンプレートに従う。
   - Notion MCP `notion-create-pages` で PRD一覧へ作成。
   - properties 例: `Name`=`（仮）{題名}`, `種別`, `カテゴリ`, `ステータス`=`下書き`, `規模`, `対象ユーザー`（JSON配列文字列可）。
   - Issue 番号は未採番のため空のまま。

4. **DesignDoc 生成（How）**
   - 同フォーマットの DesignDoc テンプレートに従う。PRDの再記述はしない。
   - DesignDoc一覧へ作成。`対象レイヤ`（Frontend/Backend/RAG/Infra/CI/DB）を入れる。
   - 作成後、PRD ↔ DesignDoc を Relation（`DesignDoc` / `PRD`）で相互リンクする。

5. **確認**
   - 要約（目的 / 受け入れ条件の要点 / 主要設計 / リスク）と Notion URL をユーザーに提示し、承認を得てから Issue 作成に進む。

6. **GitHub Issue 作成**

```bash
gh issue create --title "{{title}}" --body "{{body}}" --label "{{label}}"
```

本文フォーマット:

```markdown
## 概要
{{description}}

## Notion（正本）
- PRD: {{prd_url}}
- DesignDoc: {{designdoc_url}}

## PRD 要約
- ユーザーストーリー: …
- 受け入れ条件: …
- 非機能 / リスク: …

## DesignDoc 要約
- 変更レイヤ: …
- API / データ: …
- 既存コード: …
- リスク: …
```

7. **書き戻し**
   - 採番された Issue 番号で両 Notion ページの `Issue番号` / `GitHub` / `Name`（先頭に `#N `）を更新する。
   - `ステータス` を `レビュー中` にする（ユーザーが即実装と言う場合は `実装中`）。

**ラベル推論:** `feature` / `bug` / `enhancement` 等。種別プロパティ（feat/fix/perf/ops/調査）と揃える。

**禁止・注意:**
- Notion 作成を飛ばして Issue 本文だけに PRD/Design 全文を載せない（正本は Notion）。
- スキーマ変更を DesignDoc に書く場合は migrations up/down 必須・AutoMigrate 禁止を明記する。
- 開発者向けのみの変更は whats-new 非掲載を PRD §8 に書く。
