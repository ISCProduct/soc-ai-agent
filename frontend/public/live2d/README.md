# Live2D 面接官アバター（検討中）

面接官アバターを Live2D に替える検討のためのディレクトリ。**このディレクトリの中身（README.md 以外）は追跡しない**（`.gitignore` 参照）。Core はプロプライエタリなバイナリ、モデル一式も配布条件がリポジトリと別なので、各自がここに置く。

## 今の状態（2026-10-27）

- `lib/interview/live2d-params.ts`: モデルが面接官として使えるか（口が動くか・うなずけるか）の判定ロジック。公式サンプル4体（Hiyori/Natori/Mao/Mark）の実データで検証済み
- `lib/interview/live2d-loader.ts`: Cubism Core の読み込みとモデルからの情報抽出
- `lib/interview/live2d-motion.ts`: うなずき・まばたきのタイミング（Avatar2D と同じ単純さに合わせてある）
- `app/interview/components/Live2DAvatar.tsx`: 実際に描画するコンポーネント
- `app/interview/components/InterviewerAvatar.tsx`: `NEXT_PUBLIC_LIVE2D_ENABLED=true` のときだけ Live2D を試し、Core/モデルが無い・要件を満たさない場合は自動で `Avatar2D`（現行の2Dイラスト）へ切り替える

**まだ誰も実モデルで描画確認をしていない。** Core・モデルのどちらも未配置で、`npm run check:live2d` 以外の実行確認はできていない。以下を置けば、このコードで動くはずだが、**置いて描画して初めて分かる不具合（モデルの向き・スケール・パラメータの実際の動き方）は必ずある**（`public/avatars/README.md` の「必ず描画して確認する」と同じ理由）。

## 置くもの

```
frontend/public/live2d/
├── README.md              ← これだけ追跡する
├── core/
│   └── live2dcubismcore.min.js   ← Cubism Core（下記参照）
└── model/
    ├── model.model3.json         ← モデル本体（ファイル名は任意、下記env参照）
    ├── model.moc3
    ├── model.cdi3.json           ← 無いと判定が過小評価になる（loader参照）
    ├── *.png                     ← テクスチャ
    └── model.physics3.json       ← あれば揺れものが動く（任意）
```

### Core の入手

1. https://www.live2d.com/sdk/download/web/ から Cubism SDK for Web をダウンロード（ライセンス同意が必要）
2. 展開した中の `Core/live2dcubismcore.min.js` を `public/live2d/core/` にコピーする

### モデルの入手

公式サンプル（無償提供マテリアル）: https://www.live2d.com/sdk/download/web/ の Samples、または Cubism Editor 同梱の `Hiyori`/`Natori` 等。商用利用の条件は各自確認すること。

### 置いたら確認する

```sh
cd frontend
npm run check:live2d -- public/live2d/model
```

「✓ 面接官として使えます」と出れば、口とうなずきの要件は満たしている。「✗」が出たら理由が1行で表示される。

### 実際に画面で確認する

```sh
# frontend/.env.local に追記
echo 'NEXT_PUBLIC_LIVE2D_ENABLED=true' >> .env.local
npm run build && npm run start
```

面接画面（`/interview`）の面接官アバター枠（96px前後の小さい枠。`styles.avatarPanel` 参照）で確認する。Core/モデルを置いていない、または `NEXT_PUBLIC_LIVE2D_ENABLED` が無い状態では、常に現行の `Avatar2D`（2Dイラスト）が表示され、挙動は変わらない。

ファイル名が `model.model3.json` と違う場合は `NEXT_PUBLIC_LIVE2D_MODEL_URL=/live2d/model/<ファイル名>` で上書きする（`live2d-loader.ts` 参照）。

## 既知の懸念（置く前から分かっていること）

- **パネルが小さい**（96〜120px、アスペクト比4:3）。Live2Dモデルは全身〜バスト想定で作られていることが多く、小さい枠で表情が見えるかは実際に置いて確認が要る
- **`pixi-live2d-display`（0.4.0, 2023-12公開）が `gh-pages`（クリティカルな脆弱性あり）を依存に持つ。** `gh-pages` はこのライブラリ自身のドキュメント公開用ツールで、アプリのコードからは一切importしないため実行時リスクは無いと判断しているが、`npm audit` には出続ける。気になる場合は `pixi-live2d-display` を直接使わない実装（Cubism Framework を自前で組む）に変える必要がある
- まばたき・うなずきのタイミングは Avatar2D と同じ値（まばたき2.5〜6秒間隔・0.45秒のうなずき）を流用しているだけで、Live2Dモデルで見て自然かは未確認
