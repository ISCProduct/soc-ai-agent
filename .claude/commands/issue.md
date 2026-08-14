## GitHub Workflow Commands

### 1. Issueの作成 (`/issue`)
ユーザーの指示内容から、PRD（要件）とDesignDoc（設計）をこのコマンド内で生成し、まとめてGitHub Issueを作成します。事前に `/requirements` や `/design` を個別実行する必要はありません。

**Usage:** `/issue [実装したい機能や修正したいバグの概要]`

**Execution Workflow:**

1. **PRD生成**
   - 入力内容から機能名・ターゲットユーザー・背景/目的・既知の制約を整理する。
   - 不足情報があればユーザーに質問してから続行する（自明な場合は妥当な前提を置き、その旨を明記して進めてよい）。
   - 以下を作成する: ユーザーストーリー（As a / I want / So that）、受け入れ条件（Given/When/Then）、非機能要件、リスク・未解決事項。

2. **DesignDoc生成**
   - PRDの内容を前提に、アーキテクチャ概要（主要コンポーネントと役割）を整理する。
   - 必要に応じてMermaidでシーケンス図/データフローを作成する。
   - API/インターフェース仕様（エンドポイント、入出力、エラーケース）を定義する。
   - データモデル/永続化戦略、既存コードへのマッピング（参照ファイル・クラス）、リスクとガードレールを整理する。

3. **確認**
   - PRDとDesignDocの要約をユーザーに提示し、Issue作成前に認識合わせを行う。

4. **タイトル・ラベル推論**
   - 内容から適切なタイトルとラベル（例: `feature`, `bug`, `enhancement`）を推論する。

5. **Issue本文の組み立て**

```markdown
## 概要
{{description}}

## PRD
{{PRD要約: ユーザーストーリー / 受け入れ条件 / 非機能要件 / リスク}}

## DesignDoc
{{DesignDoc要約: アーキテクチャ概要 / API仕様 / データモデル / 既存コードへのマッピング / リスク}}
```

**Command:**
```bash
gh issue create --title "{{title}}" --body "{{body}}" --label "{{label}}"
