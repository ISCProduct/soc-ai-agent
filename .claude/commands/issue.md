## GitHub Workflow Commands

### 1. Issueの作成 (`/issue`)

ユーザーの指示から **PRD（要件）**・**DesignDoc（設計）**・**仕様書（一体型サマリ）** を生成し、**Notion に正本として作成**したうえで GitHub Issue を作る。
フォーマット正本: `docs/notion/prd-designdoc-format.md`

**Usage:** `/issue [実装したい機能や修正したいバグの概要]`

**Notion 宛先（このリポジトリ専用）:**

| | URL / ID |
| --- | --- |
| PRD DB | https://www.notion.so/f1d127bd13f94a3eb93fead33295ceda (`collection://ddf4c444-8ea3-4801-9faa-3e08afb6b660`) |
| DesignDoc DB | https://www.notion.so/3e6d1367035349a89b97f2237d0c04d8 (`collection://d00a06cd-426e-4b0d-a4d5-efe7671ed1ea`) |
| 仕様書 DB | https://www.notion.so/c549dd81990e468bbbd75db7cfa569f5 (`collection://d2d48cc6-2a7d-45e6-a6d2-9ac794778fcb`) |
| PRD テンプレート | https://www.notion.so/3d56ce9f823a81fdb742d509445aef29 |
| DesignDoc テンプレート | https://www.notion.so/3d56ce9f823a81bf94b0f0d44c6a8dc7 |
| 仕様書テンプレート | https://www.notion.so/3d66ce9f823a812bbfbdd7ec0ffc6e88 |

**フォーマット優先順位:** 上記【テンプレート】ページと `docs/notion/prd-designdoc-format.md` を正とする。新規フォーマットを発明しない。仕様書は既存 #1095 一体型（概要 / PRD / DesignDoc / 参照 / UI）を踏襲。

---

**Execution Workflow:**

1. **規模の決定**（Lite / Standard / Full。不明なら Standard）
2. **不足情報の確認**（ロール・成功指標・スコープ外）
3. **確認** … 要約とメタ（タイトル・ラベル・カテゴリ・種別）を提示し、**承認後**に Notion/Issue 作成へ進む
4. **GitHub Issue 作成**（番号採番のため先に作る。本文の PRD/DesignDoc/仕様書は後で URL のみ差し込む）
5. **Notion Markdown** … `notion://docs/enhanced-markdown-spec` を確認してから本文を書く
6. **PRD を PRD一覧へ作成**（テンプレート見出しに従う）
7. **DesignDoc を DesignDoc一覧へ作成**し、PRD と Relation 相互リンク
8. **仕様書を仕様書一覧へ作成**（【テンプレート】仕様書の見出し。PRD/DesignDoc の要約を一体型で載せる。詳細の重複は避け、詳細版 Notion へのリンクを参照に置く）
9. **GitHub Issue 本文の書き戻し**（各セクションは Notion URL のみ）

```bash
gh issue create --title "{{title}}" --body-file {{tmpfile}} --label "{{label}}"
```

```markdown
## 概要
{{description}}

## PRD

{{prd_url}}

## DesignDoc

{{designdoc_url}}

## 仕様書

{{spec_url}}
```

10. **書き戻し** … 3ページとも `Issue番号` / `GitHub` / `Name`（`#N ` 先頭）を更新。`ステータス` がある DB は `レビュー中`（または `実装中`）

**禁止・注意:**
- Notion 作成を飛ばして Issue に全文を載せない
- 仕様書だけ作って PRD/DesignDoc を省略しない（3点セット）
- migrations up/down 必須・AutoMigrate 禁止を DesignDoc/仕様書に明記
- whats-new 非掲載は PRD / 仕様書のロールアウトに書く
