# 調査: frontend の API の応答の実行時の型の検証（SOCAIAGENT-548 / #2087）

2026-11-19 時点の develop（`98ca0e5a`）を grep して調べた結果と、検証を入れる順番の案です。

## 結論

- **zod は依存にあるが、どのコードからも使われていない。** `next.config.ts` の `transpilePackages: ['zod']`（ビルドで変換する対象に名前を挙げているだけ）が唯一の参照です。
- API の応答の多くは**実行時に検証されていません**。形は2通りあります。
  - **暗黙の `any`**：`const data = await res.json()` のように型注釈の無い代入が、少なくとも83か所あります。`Response.json()` は `any` を返すので、そのまま利用側へ流れます。
  - **検証しない型付け**：`Promise<型>` の戻り値や、`unwrapArray<T>` のような型の指定です。
  - どちらも実行時には何も確かめないので、Backend の応答の形が崩れると、画面の描画の途中で例外（`undefined` の読み取り、配列でない値への `.map()` など）になるか、黙って空の表示になります。
- **明示的な** `any`（`eslint-disable` で許しているもの）は、結果画面の2ファイル（4か所）だけでした。課題の本文が挙げた `app/results/utils.ts`・`useResultsData.ts` が、その2ファイルです。ただし、問題は暗黙の `any` を含めたこの4か所よりずっと広い範囲にあります。
- いっぺんに全部へ入れるのではなく、**「形が崩れたときの被害が大きい画面」から1画面ずつ**入れるのがよいと考えます（下の「導入の順番の案」）。

## 調べたこと

| 項目 | 数 | 調べ方 |
|---|---|---|
| zod を import しているファイル（`app`・`components`・`lib`） | 0 | `grep -rln "from 'zod'"` |
| `eslint-disable … no-explicit-any`（テストを除く） | 4（2ファイル） | `app/results/hooks/useResultsData.ts`・`app/results/utils.ts` |
| 画面・ライブラリで応答を読む `.json()` の呼び出し | 182（76ファイル） | 下の「数え方」 |
| うち管理画面（`app/admin` 67・`components/admin` 5・`lib/admin` 3） | 75（33ファイル） | 同上をフォルダごとに数えた |
| うち型注釈の無い代入（`const x = await ….json()` / `let x = …`） | 83 以上 | 上の行のうち、正規表現 `(const\|let) [a-zA-Z_]+ = await [a-zA-Z_.]+\.json\(\)` に合うもの。`.then((r) => r.json())` など別の書き方は数えていないので、下限です |

**数え方**：`frontend/` で `grep -rn "\.json()" app components lib --include=*.ts --include=*.tsx` を実行し、次を除いた行の数です。1行に2回以上の呼び出しは無いので、行の数＝呼び出しの数です。

- サーバー側の転送（`app/api/`）
- テスト（`*.test.*`）
- コメントの行（`//`・`*`・`/*` で始まる行）

（初版の 186 はコメントの行の4つを含めた数、管理画面の 34 はファイルの数で、呼び出しの数と単位が混ざっていました。レビューのご指摘で直しました。コメントの行を含めると、管理画面は 77 行です。）

### 今ある手書きの確かめ方

検証が全く無いわけではなく、画面ごとに手書きで形を確かめている箇所があります。

- `lib/api.ts` の `unwrapArray<T>(raw: unknown)`：配列か、`{ data: [...] }` の形かを見て配列を取り出します（要素の形は確かめない）。
- `app/results/hooks/useResultsData.ts`：`Number.isFinite(Number(data?.…))` で数値の項目を1つずつ確かめています（3か所）。推薦の配列は `Array.isArray` で確かめますが、要素は `any` のまま `mapRecommendationToCompany` に渡しています。分析（`/api/chat/analysis`）の応答は検証せずに state に入れていて、`suggested_roles` が配列でなければ、`ResultsListView.tsx` の `.map()` で例外になり得ます。
- `lib/admin/company-fetch.ts` の `parseFetchPrimaryResponse`：管理画面の企業情報の取得の応答を、項目ごとに型を確かめ、配列（`aspects`・`errors`）は文字列の要素だけを残し、入れ子の step も確かめています（`unknown` から組み立て直す形）。**要素まで確かめている唯一の既存の例**です。

このように、確かめ方が画面ごとにばらばらです。要素の形まで確かめているのは、上の `company-fetch.ts` だけでした。

## 導入の順番の案

1. **結果画面（`/api/chat/recommendations`・`/api/chat/analysis`）**
   - 課題で名前が挙がっている、明示的な `any` が残っている唯一の場所です。
   - 学生が最も見る画面で、形が崩れたときの被害が大きいところです。
   - 推薦（`mapRecommendationToCompany` の入力）と、分析の応答全体（`job_suitability_comment`・`suggested_roles`・`score_comment`・`scores`。`mapAnalysisScores` の入力）の両方を zod のスキーマにし、4か所の `eslint-disable` をすべて外します。
   - 形が合わない要素は1件ずつ捨てて数をログに出し、画面全体を落とさない形にします（`safeParse`）。
2. **チャット（`lib/api.ts` の `sendChatMessage`・`getChatHistory`・`getUserScores`）**：`unwrapArray` を、要素も確かめる形に置き換えます。
3. **面接・職務経歴書の結果**：レポートの表示（点数・指摘）を確かめます。
4. **管理画面**：数は多い（75呼び出し・33ファイル）ものの、利用者が限られ、崩れても学生には見えないので最後にします。管理画面の取得の共通部分（`lib/admin/fetch.ts`）にスキーマを渡せる形を足し、画面ごとに移していきます。`company-fetch.ts` の `parseFetchPrimaryResponse` は、今の正規化の挙動（配列から文字列以外を捨てる、など）を変えずにスキーマへ置き換えるか、そのまま残します。検証を二重にしないためです。

### 入れ方の決まり（案）

- スキーマは `lib/schemas/`（新設）に置き、型は `z.infer` で作ります（型の定義とスキーマを二重に書かない）。
- 応答の形が合わないときは `safeParse` で受け、壊れた要素だけを捨てます。全部が壊れていたら、今の「取得に失敗しました」の表示に寄せます。
- 捨てた件数はログ（`console.warn`）に出し、Backend の形の崩れに気づけるようにします。Sentry にも送るかは、1 の導入の PR で決めます。
- zod v4 は ESM が先なので、`next.config.ts` の `transpilePackages: ['zod']` は残します。

## この調査で扱っていないこと

- Backend の OpenAPI からの型の自動生成：今回の目的（実行時の検証）とは別の話なので、扱っていません。
- バンドルの大きさへの影響（zod を画面のバンドルに入れたときの増え方）は、測っていません。1 の導入の PR で `next build` の出力を比べるのがよいと考えます。
