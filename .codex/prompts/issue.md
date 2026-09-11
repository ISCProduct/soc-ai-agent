## GitHub Workflow Commands

### 1. Issueの作成 (`/issue`)

ユーザーの指示から **PRD**・**DesignDoc**・**仕様書** を Notion に作成し、GitHub Issue には各 URL のみを載せる。
フォーマット正本: `docs/notion/prd-designdoc-format.md`（【テンプレート】を優先。新規形式を作らない）

**Usage:** `/issue [実装したい機能や修正したいバグの概要]`

**Notion 宛先:**

| | data_source / テンプレート |
| --- | --- |
| PRD | `collection://ddf4c444-8ea3-4801-9faa-3e08afb6b660` / https://www.notion.so/3d56ce9f823a81fdb742d509445aef29 |
| DesignDoc | `collection://d00a06cd-426e-4b0d-a4d5-efe7671ed1ea` / https://www.notion.so/3d56ce9f823a81bf94b0f0d44c6a8dc7 |
| 仕様書 | `collection://d2d48cc6-2a7d-45e6-a6d2-9ac794778fcb` / https://www.notion.so/3d66ce9f823a812bbfbdd7ec0ffc6e88 |

**Workflow:** 規模決定 → 不足確認 → **ユーザー承認** → GH Issue 作成 → Notion に PRD / DesignDoc / 仕様書の3ページ → Issue に URL 書き戻し。仕様書は #1095 一体型（概要/PRD/DesignDoc/参照/UI）。詳細は分離 DB、仕様書は実装者向け一枚サマリ。
