# ブランチ作成
git checkout -b "feature/issue-{{issue_number}}"
git add .
git commit -m "fix: resolve #{{issue_number}}"
git push origin "feature/issue-{{issue_number}}"

# PR作成 (Closes #Issue番号 を含めることで自動紐付け)
### 3. PRの作成 (`/pr`)

現在の変更をリモートにプッシュし、Issueと紐付けたPull Requestを作成します。

**Usage:** `/pr [Issue番号]`

**Execution Workflow:**

1.  **ブランチの作成とプッシュ**
    * 現在の変更を Issue 番号に基づいたブランチ名で作成・移動する。
    * コマンド例: `git checkout -b feature/issue-{{issue_number}}`
    * コマンド例: `git push origin feature/issue-{{issue_number}}`
2.  **変更内容の解析（AIによる生成）**
    * `git diff main...HEAD` を参照し、実装した具体的な変更点、追加機能、修正バグを箇条書きで整理する。
3.  **PRの作成**
    * `gh pr create` を使用し、以下の構成でPRを投げる。
    * **Title:** `Resolve #{{issue_number}}: [機能の短い要約]`
    * **Body:** * `Closes #{{issue_number}}` (Issueとの自動紐付け)
        * `## 変更内容` (AIが生成した詳細なリスト)
    * コマンド例:
      ```bash
      gh pr create --title "Resolve #{{issue_number}} {{title}}" \
                   --body "Closes #{{issue_number}}

      ## 変更内容
      {{ai_generated_summary}}" \
                   --base main
      ```

---

**Notes for Claude:**
- `{{ai_generated_summary}}` は、実装したコードの論理的な変更（例：「バリデーションロジックの追加」「APIエンドポイ ントの型定義の修正」など）を具体的に記述してください。
- PR作成後、生成されたPRのURLをユーザーに提示してください。