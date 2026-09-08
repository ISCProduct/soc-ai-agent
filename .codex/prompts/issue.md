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

1. **規模の決定**（Lite / Standard / Full。不明なら Standard）
2. **不足情報の確認**（ロール・成功指標・スコープ外）
3. **PRD を Notion PRD一覧へ作成**（ステータス=下書き）
4. **DesignDoc を Notion DesignDoc一覧へ作成**し、PRD と Relation で相互リンク
5. **要約と Notion URL を提示して確認**
6. **GitHub Issue 作成**（本文は要約＋Notionリンク。正本は Notion）

```bash
gh issue create --title "{{title}}" --body "{{body}}" --label "{{label}}"
```

```markdown
## 概要
{{description}}

## Notion（正本）
- PRD: {{prd_url}}
- DesignDoc: {{designdoc_url}}

## PRD 要約
{{ユーザーストーリー / 受け入れ条件 / 非機能 / リスク}}

## DesignDoc 要約
{{レイヤ / API・データ / 既存コード / リスク}}
```

7. **Issue 番号・URL を両 Notion ページへ書き戻し**（`Name` 先頭に `#N `、ステータスをレビュー中 or 実装中）

詳細セクション構成・プロパティ定義は `docs/notion/prd-designdoc-format.md` に従う。
