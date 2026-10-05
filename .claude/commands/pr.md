---
description: 現在の変更をプッシュして Issue と紐付いた PR を作成する
argument-hint: <Issue番号>
---

> **宛先は develop。** main への push は本番(ECS on Fargate)への自動デプロイを引く。
> ブランチフローは `feature/* → develop → release → main` で、develop と release の
> レビューゲートを飛ばしてはいけない。`--base main` にしない。

# ブランチ作成
git checkout -b "feature/issue-$1"
git add .
git commit -m "fix: resolve #$1"
git push origin "feature/issue-$1"

# PR作成 (Closes #Issue番号 を含めることで自動紐付け)
### 3. PRの作成 (`/pr`)

現在の変更をリモートにプッシュし、Issueと紐付けたPull Requestを作成します。

**Usage:** `/pr [Issue番号]`

**Execution Workflow:**

1.  **ブランチの作成とプッシュ**
    * 現在の変更を Issue 番号に基づいたブランチ名で作成・移動する。
    * コマンド例: `git checkout -b feature/issue-$1`
    * コマンド例: `git push origin feature/issue-$1`
2.  **変更内容の解析（AIによる生成）**
    * `git diff origin/develop...HEAD` を参照し、実装した具体的な変更点、追加機能、修正バグを箇条書きで整理する。
3.  **PRの作成**
    * `gh pr create` を使用し、以下の構成でPRを投げる。
    * **Title:** `Resolve #$1: [機能の短い要約]`
    * **Body:** * `Closes #$1` (Issueとの自動紐付け)
        * `## 変更内容` (AIが生成した詳細なリスト)
    * コマンド例:
      ```bash
      gh pr create --title "Resolve #$1 <Issueタイトル>" \
                   --body "Closes #$1

      ## 変更内容
      <変更内容の要約>" \
                   --base develop
      ```

---

**Notes for Claude:**
- `<変更内容の要約>` は、実装したコードの論理的な変更（例：「バリデーションロジックの追加」「APIエンドポイントの型定義の修正」など）を具体的に記述してください。
- PR作成後、生成されたPRのURLをユーザーに提示してください。
