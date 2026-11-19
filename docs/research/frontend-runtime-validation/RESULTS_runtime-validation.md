# 調査: frontend の API の応答の実行時の型の検証（SOCAIAGENT-548 / #2087）

2026-11-19 時点の develop（`98ca0e5a`）を grep して調べた結果と、検証を入れる順番の案です。

## 結論

- **zod は依存にあるが、どのコードからも使われていない。** `next.config.ts` の `transpilePackages: ['zod']`（ビルドで変換する対象に名前を挙げているだけ）が唯一の参照です。
- API の応答は、ほとんどが `Promise<型>` と書いて**検証せずに型を付けている**だけです。型の注釈は実行時には何も確かめないので、Backend の応答の形が崩れると、画面の描画の途中で例外（`undefined` の読み取りなど）になるか、黙って空の表示になります。
- `any` を `eslint-disable` で許しているのは、結果画面の2ファイル（4か所）だけでした。課題の本文が挙げた `app/results/utils.ts`・`useResultsData.ts` が、その2ファイルです。
- いっぺんに全部へ入れるのではなく、**「形が崩れたときの被害が大きい画面」から1画面ずつ**入れるのがよいと考えます（下の「導入の順番の案」）。

## 調べたこと

| 項目 | 数 | 調べ方 |
|---|---|---|
| zod を import しているファイル（`app`・`components`・`lib`） | 0 | `grep -rln "from 'zod'"` |
| `eslint-disable … no-explicit-any`（テストを除く） | 4（2ファイル） | `app/results/hooks/useResultsData.ts`・`app/results/utils.ts` |
| 画面・ライブラリで応答を読む `.json()`（`app/api/` の転送とテストを除く） | 186 | `grep -rn "\.json()"` |
| うち管理画面（`app/admin`・`components/admin`・`lib/admin`） | 34 | 同上をフォルダごとに数えた |

### 今ある手書きの確かめ方

検証が全く無いわけではなく、画面ごとに手書きで形を確かめている箇所があります。

- `lib/api.ts` の `unwrapArray<T>(raw: unknown)`：配列か、`{ data: [...] }` の形かを見て配列を取り出します（要素の形は確かめない）。
- `app/results/hooks/useResultsData.ts`：`Number.isFinite(Number(data?.…))` で数値の項目を1つずつ確かめています（3か所）。推薦の配列は `Array.isArray` で確かめますが、要素は `any` のまま `mapRecommendationToCompany` に渡しています。

このように確かめ方が画面ごとにばらばらで、要素の形まで確かめているところはありません。

## 導入の順番の案

1. **結果画面（`/api/chat/recommendations`・`/api/chat/analysis`）**
   - 課題で名前が挙がっている、`any` が残っている唯一の場所です。
   - 学生が最も見る画面で、形が崩れたときの被害が大きいところです。
   - `mapRecommendationToCompany` の入力を zod のスキーマにし、`eslint-disable` を外します。
   - 形が合わない要素は1件ずつ捨てて数をログに出し、画面全体を落とさない形にします（`safeParse`）。
2. **チャット（`lib/api.ts` の `sendChatMessage`・`getChatHistory`・`getUserScores`）**：`unwrapArray` を、要素も確かめる形に置き換えます。
3. **面接・職務経歴書の結果**：レポートの表示（点数・指摘）を確かめます。
4. **管理画面**：数は多い（34）ものの、利用者が限られ、崩れても学生には見えないので最後にします。管理画面の取得の共通部分（`lib/admin/fetch.ts`）にスキーマを渡せる形を足し、画面ごとに移していきます。

### 入れ方の決まり（案）

- スキーマは `lib/schemas/`（新設）に置き、型は `z.infer` で作ります（型の定義とスキーマを二重に書かない）。
- 応答の形が合わないときは `safeParse` で受け、壊れた要素だけを捨てます。全部が壊れていたら、今の「取得に失敗しました」の表示に寄せます。
- 捨てた件数はログ（`console.warn`）に出し、Backend の形の崩れに気づけるようにします。Sentry にも送るかは、1 の導入の PR で決めます。
- zod v4 は ESM が先なので、`next.config.ts` の `transpilePackages: ['zod']` は残します。

## この調査で扱っていないこと

- Backend の OpenAPI からの型の自動生成：今回の目的（実行時の検証）とは別の話なので、扱っていません。
- バンドルの大きさへの影響（zod を画面のバンドルに入れたときの増え方）は、測っていません。1 の導入の PR で `next build` の出力を比べるのがよいと考えます。
