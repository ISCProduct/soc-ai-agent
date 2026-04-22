## GitHub Workflow Commands

### 1. Issueの作成 (`/issue`)
ユーザーの指示内容をもとに、適切なタイトルとラベルを推論してGitHub Issueを作成します。

**Usage:** `/issue [実装したい機能や修正したいバグの概要]`

**Command:**
```bash
gh issue create --title "{{title}}" --body "{{description}}" --label "{{label}}"
