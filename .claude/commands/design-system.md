---
description: デザインシステム（トークン・部品・学生3画面の型・モック規約）を読み込む
argument-hint: 作る画面・モックの概要（任意）
---
# /design-system

誰が画面設計・モックを作っても、同じ規約・トークン・部品に沿わせるための入口。
**画面設計・モック作成・Notion 資料化の前に読む。** 引数があれば、その画面について末尾「出力」の形で返す。

このコマンドは要約。詳細は SKILL.md とコードを読む。ここに無いことは発明せず、コードで確認する。

## 1. 正本の優先順位

1. **コード上のトークン**（`frontend/lib/design-tokens.ts` / `student-theme.ts` / `admin-theme.ts` / `company-theme.ts` / `lib/design/tokens.ts`）
2. **このコマンド**
3. `.claude/skills/school-career-ui-design/SKILL.md`（約1,200行。UI/UX の原則と画面別ルール）

食い違いを見つけたらコードを優先し、食い違いを報告に書く。勝手にどちらかを書き換えない。

## 2. 基準と範囲（決定済み）

- 基準は **MUI 7**（`@mui/material` ^7）。新規画面は MUI で作る。
- `components/ui/*`（button/card/dialog/input/progress/table/tabs/badge。radix + tailwind + cva）は **既存の利用箇所のみ**。新規で使わない。
- MUI と shadcn 系の混在解消は Issue #2105。このコマンドでは統一しない。
- ロール別にテーマが分かれる（`components/MuiProvider.tsx`）。**学生の画面を作るなら学生テーマ**。
  - 学生（既定）: `createStudentMuiTheme(mode)`。`mode` は `comfortable`（既定）/ `high-contrast`
  - 管理: `createAdminMuiTheme()`（`/admin` 配下）／企業ポータル: `createCompanyMuiTheme()`（`/company-portal` 配下）
  - 管理・企業は意図的に別 identity。学生の画面に持ち込まない

## 3. トークン（学生。値は 2026-11 時点のコード）

### 色
値は `STUDENT_SURFACES`（`lib/student-theme.ts`）。画面では `UI.<key>`（`lib/design/tokens.ts`）で参照する。
`UI.*` は CSS 変数 `--ui-<kebab>`（例 `--ui-ink-soft`）で、高コントラストにも追従する。

| 用途 | キー（comfortable） | 値 |
|---|---|---|
| 地 / 面 / 薄い面 | `paper` / `field` / `tint` | #f4f7fb / #ffffff / #f1f5f9 |
| 文字 / 補助 / 弱 | `ink` / `inkSoft` / `inkFaint` | #0f172a / #475569 / #64748b |
| 罫線 / 薄罫線 | `rule` / `ruleSoft` | #d0d7de / #e2e8f0 |
| 主操作・強調 | `mark`（= primary） | #0072B2 |
| 要注意の旗 | `flag` | #B44A00 |
| 面接の暗い画面 | `nightBg` `nightStage` `nightPanel` `nightInk` `nightSoft` `nightMuted` | #221610 #0f131d #2d2e31 #e8eaed #bdc1c6 #9aa0a6 |

MUI palette（comfortable）: primary #0072B2 / success #009E73 / warning・secondary #E69F00 / error #000000 / background.default #f4f7fb / text.primary #111111・secondary #333333。
error が黒なのは色覚セーフ配色のため。**色だけで状態を伝えない**（SKILL §19）。必ず文字か字形（● ✓ ◐ ○）を添える。
「Wong 配色（色覚セーフ）は意匠ではなく要件」（`lib/design/tokens.ts`）。色を足さない・変えない。

### タイポ
- 書体: `FONT_STACK_PROPORTIONAL`（BIZ UDPGothic → Noto Sans JP → システム体。webfont は配信していない）
- 本文 16px（`BODY_FONT_SIZE`）。body1/body2 = 1rem・行間 1.6、caption = 0.875rem
- 見出し: h4 1.75rem / h5 1.375rem / h6 1.125rem（いずれも 700）。h1〜h3 は MUI 既定値に固定
- 見出しを大きくしすぎない。階層は 4 段（ページ / セクション / 主情報 / 補助）まで

### 余白・角丸・影・幅
- 余白: MUI の `spacing`（1 = 8px）。`sx={{ p: 2 }}` のようにテーマ単位で書き、px 直書きを増やさない
- 角丸: `shape.borderRadius` = 12（高コントラストは 4）。独自の角丸値を足さない
- 影: 使わない。構造は罫線（`UI.rule`）で表す。MuiCard は 1px 罫線（高コントラストは 2px の primary）
- 本文幅: `CONTENT_MAX_WIDTH` = 1200（`lib/design/tokens.ts`）。面接履歴は `INTERVIEW_HISTORY_CONTENT_MAX_WIDTH` = 1100
- ボタン: 高さ 44px 以上・`textTransform: none`・太字（テーマ設定済み）
- focus: `*:focus-visible` に 2px（高コントラスト 3px）の primary 枠（テーマ設定済み）。消さない

## 4. 部品の使い分け

| 用途 | 使うもの（実在） |
|---|---|
| 空表示 | `components/common/EmptyState`（title / description / action） |
| 失敗表示（主要部分が描けない） | `components/common/ErrorState`（`onRetry` は再試行で直る時だけ） |
| 1行のエラー | `components/common/ErrorAlert` |
| 一覧の読み込み | `components/common/ListSkeleton`（表は `TableSkeletonRows`） |
| ページ全体の読み込み | `components/common/PageLoading` / `RouteLoading`（`loading.tsx` 用） |
| 接続不能の全画面 | `components/ServiceErrorView` |
| 下部ナビ（学生） | `components/StudentBottomNav`（項目は `lib/sidebar-nav.ts` の `STUDENT_BOTTOM_NAV_ITEMS`） |
| 基本部品 | MUI: Button / Stack / Typography / TextField / Chip / Tabs / Dialog / Alert / Skeleton / Tooltip |

- 既存コンポーネントがあれば再利用し、似たものを新規に作らない（SKILL §30）。実装前に `frontend/components` と `frontend/app` を見る
- アイコンは `@mui/icons-material` が主（面接・ナビなど既存画面の大半）。`lucide-react` は `components/ui/dialog.tsx` など shadcn 系の内部のみ。新規は `@mui/icons-material`
- アイコンだけのボタンには `aria-label` と Tooltip を付ける
- Dialog は削除確認・短い編集・補助情報だけ。長い入力は独立ページ
- カードを乱用しない。クリック単位・まとまりが必要な時だけ

### 禁止（SKILL §27 の要約）
大量の KPI カード / 紫系グラデ / Glassmorphism / 過剰な影・角丸 / 意味のないアニメーション・アイコン / 大きすぎる見出し / LP のような業務画面 / 装飾目的のグラフ。
新しい UI ライブラリを足さない。直書きの色（#hex）を新規に足さない（トークンを使う）。

## 5. 学生3画面の型

代表画面。構成と状態を揃えて作る。コードのパスは実在するもの。

### 5.1 チャット（診断）— `/`（`app/page-content.tsx` → `components/mui-chat/`）
- 構成: ChatHeader / ChatMessageList（`role="log"`・`aria-live="polite"`）/ ChatInputBar（選択肢は `role="group"`、送信は `aria-label` 付き、入力と音声ボタンは 44px）
- Loading: メッセージ領域の Skeleton。エージェント入力中は TypingIndicator（`role="status"`）
- Empty: 初回は `EmptyState`（`dense`）で「まだ回答がありません」+ 次に何が起きるかの説明 + 候補チップ（action 枠）。挨拶は Empty には出さず、履歴0件の取得時にメッセージ（ログ）として入る（`hooks/useMuiChat.ts`・`utils.ts`）
- Error: 履歴の読み込み失敗は `historyLoadError` で入力を無効化する。文言は次の操作つきにし、ログは消さない
- モバイル: 入力バーと `StudentBottomNav` を重ねない。下余白は、チャットは `BottomNavSpacer` を使わず `app/page.module.css` の `.mainContent` の `padding-bottom: 56px`（`min-width: 900px` で 0）で確保する。他の画面（結果・履歴など）は `components/common/BottomNavSpacer`

### 5.2 面接 — `/interview`（`app/interview/page-content.tsx`）
- 状態遷移（`InterviewStatus`）: `selection` → `lobby` → `connecting` → `connected` → `finished`（途中 `error`）
- 画面: SelectionScreen（企業・形式を選ぶ）/ LobbyScreen（カメラ・マイク確認）/ SessionScreen（面接中。暗い面 `night*`）/ ReportScreen（レポート）
- 面接中は暗い面（`UI.night*`）。主操作の色は `UI.mark`。面接中は `StudentBottomNav` を隠す
- Loading: `loading.tsx`（RouteLoading）。レポート生成中は文言付きの進捗（「レポートを生成中です...」。`ReportScreen`）
- Empty: 面接履歴（`/interview/history`）が空の時は `EmptyState`（次の操作 = 面接を始める）
- Error: 権限拒否は LobbyScreen にブラウザ許可の手順つきで出す（`lobbyPermissionError`）。保存失敗は SessionScreen、レポート失敗は ReportScreen で `role="alert"`。原因と次の操作を添える
- 注意: 1応答に 5〜7 秒かかる（運用メモ）。待ち時間は進行表示で埋め、無反応に見せない

### 5.3 結果（おすすめ企業）— `/results`（`app/results/`）
- 構成: ヘッダー帯（見出し・件数）→ 企業の並び（順位・企業名・適合度・理由の先頭2文）→ 企業詳細
- 適合度は各行に数字（`matchScore`）で出す。候補全体の幅は `scaleBand` で求め、幅が15以下に収まるときだけ「順位の差はわずか」の注記を出す（`positionInBand` は定義のみで一覧では未使用）。理由文は `leadSentences`
- Loading（現状）: `ResultsLoadingView`。`maxWidth: 960` の列に `ListSkeleton`（4行。各行は細い3本の線）を出す。確定後の一覧は `maxWidth: 1200`（`CONTENT_MAX_WIDTH`）で、行は罫線区切り（順位・企業名・適合度・理由・技術タグ・ボタン）のため、**確定後と同じ形ではない**（幅も行の構成もずれる）。標準形は「確定後と同じ幅・行構成の Skeleton」。モックでは標準形で描き、現状とのずれは「食い違い」に書く
- Empty（現状の経路）: 0件のとき `useResultsData` が `error` に理由文（`buildEmptyRecommendationsMessage`）を入れるため、`page-content.tsx` では **`ResultsErrorView`** が出る（文言・再試行・チャットに戻る・やり直す）。`ResultsNoMatchView`（「適合する企業が見つかりませんでした」）は企業0件かつ error 無しの稀な経路だけ。`ResultsEmptyView` は `page-content.tsx` から使われておらず（テストのみ）、現状は画面に出ない。標準形は「診断を進める導線 + 更新」で、**「データがありません」で終わらせない**
- Error: `ResultsErrorView`。何が起きたか + 再試行（ページ更新）・チャットに戻る・最初からやり直す
- モバイル: 余白・文字サイズ・ボタン幅を `xs` / `sm` で切り替える（`ResultsListView`）。表は使わず縦に並べる。下余白は `BottomNavSpacer`

## 6. 状態・レスポンシブ・アクセシビリティの最低線
- **Loading**: 画面を真っ白にしない。Skeleton（一覧）か文言付きの進捗。読み上げに「読み込み中」を1回だけ
- **Empty**: 何が無いか → 次にすること → 操作ボタン（具体名。「OK」「実行」は不可）
- **Error**: 何が起きたか + 次にすること。謝らない。ステータスコード・例外名を出さない
- **幅**: 375 / 768 / 1024 / 1440px で確認。学生はスマホ優先。横スクロールの表を避け、タップ領域は 44px 以上
- **アクセシビリティ**: キーボード操作・focus 表示・label / aria-label・コントラスト 4.5:1 以上・見出し構造・色だけで状態を伝えない・適切な HTML 要素
- **高コントラスト**: `UI.*` を使えば追従する。直書きの色は追従しない

## 7. モック作成のルール
- **単体 HTML 1 枚**。外部の CSS・フォント・アイコン・スクリプト・画像に依存しない（Notion の埋め込みは外部をブロックする）
- 書体は `font-family` を §3 の順で指定するだけ。webfont を読まない。アイコンは文字（● ✓ ◐ ○）かインライン SVG
- トークンは `:root` の CSS 変数に写す。値は §3 の表から（コードが正。ずれたらコードに合わせる）

```html
<style>
  :root {
    --ui-paper:#f4f7fb; --ui-field:#fff; --ui-tint:#f1f5f9;
    --ui-ink:#0f172a; --ui-ink-soft:#475569; --ui-ink-faint:#64748b;
    --ui-rule:#d0d7de; --ui-rule-soft:#e2e8f0;
    --ui-mark:#0072B2; --ui-flag:#B44A00;
    --radius:12px; --space:8px; --btn-h:44px; --content-max:1200px;
  }
  body { margin:0; background:var(--ui-paper); color:var(--ui-ink); font:16px/1.6
    'BIZ UDPGothic','Noto Sans JP',-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif; }
  :focus-visible { outline:2px solid var(--ui-mark); outline-offset:2px; }
  button { min-height:var(--btn-h); border-radius:var(--radius); font-weight:700; }
</style>
```

- 想定幅は 375（スマホ）と 1440（PC）の 2 枚。必要なら 768 を足す。状態（Loading / Empty / Error）は画面ごとに並べて見せる
- **実在しない画面・部品を「既存」と書かない。** 新規提案は「新規」と明記し、既存の画面は実在パスで示す
- **個人情報・本番データを載せない。** 氏名・学校名・メール・実在企業の非公開情報は架空のダミーにする
- 学生の画面に管理（`ADMIN_COLORS`）・企業（`COMPANY_COLORS`）の色を混ぜない
- 実装に近い見た目の確認は `cd frontend && npm run dev`（3000）で既存画面と並べる

## 8. Notion 資料化への引き継ぎ
- 配置・書式は `docs/notion/prd-designdoc-format.md` と `docs/notion/README.md` に従う。**HTML の埋め込みは使わない**（Notion が外部アイコン等をブロックする）。**PNG にして貼る**
- PNG 化（macOS の Chrome headless。幅ごとに 1 枚）:

```bash
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --disable-gpu \
  --hide-scrollbars --window-size=1440,1000 --screenshot=mock-pc.png file://$PWD/mock.html
# スマホ幅は --window-size=375,812
```

- 載せるときの注意: 画像に個人情報が写っていないか / 「新規提案」と「既存」を区別して書く / 参照したトークンとパスを本文に残す / 画像の代替テキストを付ける
- 注意: `/issue` コマンド（`.claude/commands/issue.md`）に Chrome headless の手順は無い。上記は本コマンドの手順

## 9. 出力（引数があるとき）
1. 対象の利用者・画面の目的・最重要操作
2. 使うトークン・部品（実在パス付き）と、新規に必要なもの（新規と明記）
3. 状態（Loading / Empty / Error）と 375 / 1440px の見え方
4. 食い違い・未確認があれば列挙

## 10. 参考にした情報
- コード: `frontend/lib/design-tokens.ts` / `student-theme.ts` / `admin-theme.ts` / `company-theme.ts` / `lib/design/tokens.ts` / `components/MuiProvider.tsx` / `components/common/*` / `components/mui-chat/` / `app/interview/` / `app/results/` / `frontend/tests/lib/design-tokens.test.ts`
- ドキュメント: `CLAUDE.md` / `.claude/skills/school-career-ui-design/SKILL.md` / `docs/notion/README.md` / `docs/notion/prd-designdoc-format.md` / `docs/wiki/claude-commands.md`
- Issue: #2105（MUI と shadcn 系の混在解消）/ #851（ページ間のデザイン一貫性。3系統が混在という指摘）
