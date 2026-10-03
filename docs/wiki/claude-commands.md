# カスタムコマンド（Claude Code）

`.claude/commands/` に置いた Markdown が、そのままスラッシュコマンドになります。
ファイルはリポジトリにコミットされているので、**clone すれば全員が同じコマンドを使えます**。

| | |
|---|---|
| 置き場所 | `.claude/commands/*.md` |
| コマンド名 | **ファイル名**（`SRE.md` → `/SRE`） |
| 共有範囲 | git 管理下。チーム全員 |
| 関連 | [スキル](#スキル) `.claude/skills/`、[CLAUDE.md](../../CLAUDE.md) |

---

## 目次

1. [一覧](#1-一覧)
2. [開発の流れで使う順番](#2-開発の流れで使う順番)
3. [各コマンド](#3-各コマンド)
4. [書き方のルール](#4-書き方のルール)
5. [既知の不整合](#5-既知の不整合)
6. [新しく追加する](#6-新しく追加する)

---

## 1. 一覧

| コマンド | 役割 | 引数 | 種類 |
|---|---|---|---|
| `/requirements` | 要件抽出とユーザーストーリー生成 | 機能名・ユーザー・目的・制約 | 雛形 |
| `/design` | アーキテクチャとデータフロー設計 | 要件サマリ・既存システム情報 | 雛形 |
| `/issue` | PRD / DesignDoc / 仕様書を Notion に作り Issue を起票 | 機能・バグの概要 | 手順 |
| `/implement` | Issue または PR レビュー指摘を読んで実装 | Issue番号 または PR番号 | 手順 |
| `/selfcheck` | マージ前のセルフチェック | Issue番号（省略可） | 手順 |
| `/pr` | 変更をプッシュして PR を作成 | Issue番号 | 手順 |
| `/code-review` | PR レビュー用のチェックリスト出力 | なし | 雛形 |
| `/code-review-local` | 手元レビュー用のチェックリスト出力 | なし | 雛形 |
| `/SRE` | 信頼性・可観測性の観点で調査/設計/レビュー | モード名 + 対象 | 手順 |

**種類**の違いは実際の挙動に出ます。

- **手順** … 実行手順が書かれている。Claude が `gh` や `git` を動かして作業する
- **雛形** … 見出しと空欄のテンプレート。Claude はそれを埋めた文書を出す（コマンドやファイル変更は走らない）

`/code-review` と `/code-review-local` は**チェックリストの書式そのもの**です。
「レビューを実行する」コマンドではないので、実際に差分を見てほしいときは `/selfcheck` か、後述の外部レビューコマンドを使ってください。

---

## 2. 開発の流れで使う順番

```
 上流                    実装                      マージ前           マージ
  │                       │                          │                 │
  ├─ /requirements        ├─ /implement <番号>       ├─ /selfcheck      ├─ /pr <番号>
  │    要件・AC            │    ブランチを切って実装   │    規約・秘密情報  │    PR作成
  │                       │                          │    デグレ          │
  ├─ /design              │                          │                 │
  │    構成・API・ER       │                          ├─ /code-review    │
  │                       │                          │    チェックリスト  │
  └─ /issue               │                          │                 │
       Notion 3点セット    │                          │                 │
       + Issue            │                          │                 │
                          │
                  （運用・障害時はいつでも）
                          └─ /SRE incident|review|slo|obs|capacity|postmortem|toil
```

全部を通す必要はありません。小さな修正なら `/implement` → `/selfcheck` → `/pr` だけで十分です。

---

## 3. 各コマンド

### `/requirements`

要件を構造化します。出力は固定の7節。

```
/requirements 企業検索のコスト上限機能、管理者向け、月20ドルに抑える、OpenAI web_search 利用
```

1. 入力サマリ ／ 2. 追加で確認したい質問 ／ 3. ユーザーストーリー（As a / I want / So that の表）
4. 受け入れ条件（Given-When-Then）／ 5. 非機能要件 ／ 6. ランディングゾーンと制約
7. リスクと未解決事項 ／ 8. 成功指標

足りない情報は節2に質問として出てきます。**空欄は埋めずに TODO として残る**ので、そこが設計前に決めるべき点です。

### `/design`

`/requirements` の次。Mermaid 図を含む設計文書を出します。

```
/design SOCAIAGENT-334 のマッチング改修。プロファイル未登録企業が最良マッチになる
```

アーキテクチャ概要 → データフロー/状態遷移 → API 仕様 → データモデル → **既存コードへのマッピング** → リスク → テスト/観測性。

「既存コードへのマッピング」節があるので、触るファイルの当たりをつけるのに使えます。

### `/issue`

一番手順が重いコマンドです。**Notion を正本**として PRD・DesignDoc・仕様書の3点セットを作り、そのうえで GitHub Issue を起票します。

```
/issue 企業検索のコストが高いので web_search の呼び出し回数を抑えたい
```

宛先の Notion DB（PRD / DesignDoc / 仕様書）とテンプレートの URL はコマンド本体に書かれています。フォーマットの正本は `docs/notion/prd-designdoc-format.md` とテンプレートページで、**新しいフォーマットを発明しない**のがルールです。

流れ:

1. 規模を決める（Lite / Standard / Full、不明なら Standard）
2. 不足情報を確認
3. 要約とメタ情報を提示 → **承認を取ってから**作成に進む
4. GitHub Issue を先に作る（番号を採番するため）
5. Notion に PRD → DesignDoc（PRD と Relation 相互リンク）→ 仕様書
6. Issue 本文に Notion の URL だけを書き戻す
7. Notion 側に Issue番号 / GitHub / Name / **Backlogキー** を書き戻す

`Backlogキー` は注意が必要です。`github-issue-to-backlog.yml` が Backlog 課題を作って Issue タイトルを `[SOCAIAGENT-N] ...` に書き換えるまで数十秒かかります。`gh issue view <番号> --json title -q .title` を何度か試してプレフィックスが付いたら書き込みます。付かないまま終わる場合は**空欄のまま放置せず理由を伝える**（空欄だと Backlog と突き合わせられません）。

`ステータス` は `承認済み` にします。`レビュー中` にすると誰も動かさず全件止まるため、意図的にこうなっています。

禁止事項として、Notion を飛ばして Issue に全文を載せる、仕様書だけ作って PRD/DesignDoc を省く、migrations の up/down 必須と AutoMigrate 禁止を書き忘れる、が挙がっています。

> **運用方針の確認が必要です。** このコマンドは GitHub Issue を直接起票しますが、現在は Backlog 起点（Backlog 課題を作ると GitHub Issue へ同期される）で運用している場面があります。どちらで起票するかはチームで揃えてから使ってください。

### `/implement`

Issue か PR の内容を読んで実装します。**引数はどちらでも通ります。**

```
/implement 1523      # Issue を読んで実装
/implement 1289      # PR のレビュー指摘を読んで修正
```

- Issue なら `gh issue view`
- PR なら `gh pr view <番号> --comments` で指摘を解析

実装前に修正方針を提示します。レビュー対応のときは指摘を一覧にして、どう直したかを個別に報告します。

既に PR 用ブランチがあればそのブランチ上で作業し、新しく切りません。コミットメッセージは用途で分かれます。

| 状況 | メッセージ |
|---|---|
| 新規実装 | `feat: #<番号> <タイトル>` |
| レビュー修正 | `fix: address review comments for #<番号>` |

破壊的変更が含まれる場合は、書き換える前に承認を取ります。

### `/selfcheck`

マージ前の自己点検。中身は `.github/prompts/selfcheck.prompt.md` を `@` で読み込んでいます。

```
/selfcheck          # 現在のブランチの差分だけ見る
/selfcheck 1523     # Issue の要件と突き合わせる
```

検査する軸:

| 区分 | 内容 |
|---|---|
| A コミットメッセージ | プレフィックス（`feat`/`fix`/`refactor`/`docs`/`chore`/`test`）、意味のある文、Issue番号 |
| B コード規約 | CLAUDE.md 準拠。日本語コメント、Go の `any`/`slices`/`errors.Is`、FE の `any` 禁止、Python の型ヒント |
| C セキュリティ | 秘密情報のハードコード、入力バリデーション、OWASP Top 10、設定の外部化 |
| D 機能・品質 | 要件充足、デグレ、エッジケース、TODO/FIXME の残り |

PR を出す前に通すと、CI で落ちる前に規約違反を拾えます。

### `/pr`

ブランチを切ってプッシュし、PR を作ります。

```
/pr 1523
```

ブランチ名は `feature/issue-<番号>`。本文に `Closes #<番号>` を入れて Issue と自動で紐付けます。タイトルは `Resolve #<番号>: <要約>`。

> **⚠ そのまま実行しないでください。** コマンド本体が `--base main` を指定しています。`main` への push は**本番（ECS on Fargate）への自動デプロイ**を引きます。本来のブランチフローは `feature/* → develop → release → main` なので、宛先は `develop` です。詳細は[既知の不整合](#5-既知の不整合)。

### `/code-review` ／ `/code-review-local`

レビュー用のチェックリストを出します。差分を解析するコマンドではありません。

| | `/code-review` | `/code-review-local` |
|---|---|---|
| 想定 | PR に貼る | 手元の確認 |
| 基本情報 | 概要・種別・関連Issue | 対象ファイル・ブランチ・日付・チケット |
| 指摘欄 | 優先度・内容・根拠・**対応者・期限** | 優先度・**ファイル/行**・内容・対応状況 |
| 完了サイン | なし | あり |

優先度の目安は共通です。

| 記号 | 意味 |
|---|---|
| 🔴 High | リリースブロッカー |
| 🟡 Mid | マージ前に対応 |
| 🟢 Low | 次回以降でも可 |

どちらにも「Lint が通っている（FE `npm run lint` / BE `go vet`）」が明示的に入っています。

### `/SRE`

信頼性の観点で見るコマンド。**第1引数がモード名**で、7つのプレイブックがあります。

```
/SRE incident    チャットが500を返している、15分前から
/SRE review      #1289 のレイテンシ改修
/SRE slo         企業検索API
/SRE postmortem  9/28 の frontend OOM
```

| モード | 用途 | 特徴 |
|---|---|---|
| `incident` | 障害対応 | **原因不明でも止血案を先に出す**。仮説ごとに反証可能な確認クエリを付ける |
| `review` | 設計・変更レビュー | 「どう壊れるか」から書く。Blocker / Should / Nit で重み付け |
| `slo` | SLO・エラーバジェット | 多重ウィンドウのバーンレートアラート（14.4x→ページ 等）。**エラーバジェットポリシー必須** |
| `obs` | 可観測性 | RED / USE / 4 Golden Signals。アラートは症状ベース |
| `capacity` | キャパシティ・性能 | 使用率ではなく**飽和（待ち行列）**で判断 |
| `postmortem` | ポストモーテム | Blameless。TTD / TTM を数値化 |
| `toil` | 労力削減 | 頻度×時間×人数で年間コスト算出。**削除 > セルフサービス化 > 自動化** |

モードを省くと内容から自動判別し、判別根拠を1行述べてから進みます。情報が足りなくても**最大3つしか質問せず**、残りは「仮定: 〜」と明示して進みます。

出力ルールとして、推測には「仮説:」、確認済みには根拠（ファイル名・行・メトリクス名）を添え、不明点は「未確認」と書いて埋めません。

全モードに共通で当たる観点（単一障害点、タイムアウト予算の階層、リトライの掛け算爆発、飽和、時限爆弾、群れ挙動、深夜3時の人間が実行できるか）と、アンチパターン一覧がコマンド本体にあります。

---

## スキル

`.claude/skills/` はコマンドとは別の仕組みです。コマンドが「こちらから呼ぶ」ものに対し、スキルは**関連する作業のときに自動で読み込まれます**。

| スキル | 内容 |
|---|---|
| `school-career-ui-design` | 学生・教員・管理者向け UI/UX の設計規約（約1,200行） |

UI の設計・改修をするときは CLAUDE.md の指示によりこれが適用されます。利用者別の情報優先順位、禁止事項（KPIカードの乱立、紫グラデーション、過剰な角丸など）、画面作成の10ステップが定義されています。

---

## 4. 書き方のルール

### frontmatter

ファイル先頭に YAML を `---` で挟みます。**1行目が `---` でなければ frontmatter として読まれません。**

```markdown
---
description: SRE として信頼性・可観測性・インシデント対応の観点で調査/設計/レビューを行う
argument-hint: [incident|review|slo|obs|capacity|postmortem|toil] <対象・状況>
---

# 本文（プロンプト）
```

| キー | 役割 |
|---|---|
| `description` | `/help` の一覧に出る説明 |
| `argument-hint` | 引数の入力ヒント |

**コマンド名は指定できません。** 常にファイル名が使われます。`name` キーを書いても無視されます。

### 引数

| 記法 | 展開 | 使っているファイル |
|---|---|---|
| `$ARGUMENTS` | 引数全体 | `SRE.md` |
| `$1` `$2` | 位置引数 | なし |
| `{{number}}` | **展開されない** | `implement.md` `pr.md` `issue.md` |

`{{...}}` は置換されず、そのまま本文に残ります。Claude が文脈から読み取って動くため結果的に動作はしますが、**仕様としては `$ARGUMENTS` / `$1` を使ってください**。

### ファイル参照

`@パス` で別ファイルを読み込めます。長いプロンプトを分けたいときに使います。

```markdown
---
description: 現在のブランチの変更をマージ前にセルフチェックする
---

@.github/prompts/selfcheck.prompt.md
```

`selfcheck.md` がこの形です。

---

## 5. 既知の不整合

現状のコマンド群には食い違いがあります。**使う前に把握しておいてください。**

### 🔴 `/pr` の宛先が `main` になっている

`pr.md` が `--base main` と `git diff main...HEAD` を指定しています。`selfcheck.prompt.md` も `git diff main...HEAD` です。

```bash
gh pr create ... --base main      # ← pr.md の記述
```

`main` への push は**本番（ECS on Fargate）への自動デプロイ**を引きます（`deployment.yml`）。ブランチフローは `feature/* → develop → release → main` で、`develop` と `release` のレビューゲートを飛ばすことになります。

テスト自体は `main` 宛ての PR でも走ります（`test.yml` の `pull_request.branches` に `main` が含まれる）ので、CI が無いわけではありません。問題は**宛先**です。

`/pr` を使うときは宛先を `develop` に読み替えるか、`gh pr create --base develop` を自分で実行してください。

### 🟡 `/requirements` と `/design` の frontmatter が効いていない

両ファイルは JSON ブロックで始まっています。

```
{
  "name": "spec-requirements",
  "description": "要件抽出＋ユーザーストーリー生成",
  "argument_hint": "機能名、ターゲットユーザー、目的、制約を入力"
}
---
```

1行目が `---` ではないため、これは frontmatter ではなく**プロンプト本文の一部**として扱われます。結果:

- `description` が `/help` に出ない
- 宣言されている `spec-requirements` / `spec-design` という名前は効かない。実際の名前は **`/requirements` と `/design`**（ファイル名）
- `argument_hint` はキー名も違う（正しくは `argument-hint`）

`SRE.md` / `selfcheck.md` と同じ YAML 形式に直せば解消します。

### 🟡 `{{number}}` が展開されない

`implement.md` `pr.md` `issue.md` が `{{number}}` `{{title}}` などを使っています。Claude Code の置換対象ではないため、文字列のまま残ります。`$1` に置き換えるのが正しい形です。

### 🟡 `/issue` と Backlog 起点運用の衝突

`/issue` は GitHub Issue を直接起票します。一方で `github-issue-to-backlog.yml` による GitHub → Backlog 同期があり、場面によっては Backlog 起点で運用しています。どちらを正とするかはチームで決めてから使ってください。

---

## 6. 新しく追加する

1. `.claude/commands/<名前>.md` を作る。ファイル名がコマンド名になるので、**短くて打ちやすい名前**にする
2. 1行目から `---` で frontmatter を書き、`description` を入れる（`/help` に出る）
3. 引数を取るなら `argument-hint` を書き、本文で `$ARGUMENTS` か `$1` を使う
4. 本文にやってほしいことを書く。手順を実行させるなら番号付きの手順、文書を出させるなら見出しの雛形
5. コミットする。`.claude/commands/` は git 管理下なのでチーム全員に配られる

既存の `SRE.md` が frontmatter・引数・出力ルールを揃えた形になっているので、雛形として参考になります。

### 置き場所の使い分け

| 置き場所 | 共有範囲 | 用途 |
|---|---|---|
| `.claude/commands/` | リポジトリ全員 | チームで共有する定型作業 |
| `~/.claude/commands/` | 自分だけ | 個人的な作業用 |

---

## 外部レビューコマンド

リポジトリ外（プラグイン由来）で使えるレビュー系コマンドもあります。`.claude/commands/` には入っていません。

| コマンド | 内容 |
|---|---|
| `/code-review` | ブランチまたは PR のレビュー。`ultra` を付けるとクラウドで多エージェント実行 |
| `/codex:review` | Codex によるレビュー |
| `/coderabbit:code-review` | CodeRabbit CLI によるレビュー |

`.claude/commands/code-review.md`（チェックリスト雛形）と名前が重なる点に注意してください。

---

**最終更新 2026-10-02** — `.claude/commands/` の9ファイルと `.claude/skills/school-career-ui-design/`、`.github/workflows/{test,deployment}.yml` を読んで作成。
