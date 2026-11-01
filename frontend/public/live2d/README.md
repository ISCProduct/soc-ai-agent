# Live2D 面接官アバター（検討中）

面接官アバターを Live2D に替える検討のためのディレクトリ。**このディレクトリの中身のうち `README.md` と `shaders/` 以外は追跡しない**（`.gitignore` 参照）。Core はプロプライエタリなバイナリ、モデル一式も配布条件がリポジトリと別なので、各自がここに置く。Shaders は Framework 付属物で再配布可のため、唯一ここだけ最初からリポジトリに入っている。

## 今の状態（2026-10-28）

公式サンプルモデル（Hiyori）で**実際に描画確認済み**。

- `vendor/live2d-framework/`: Live2D公式 Cubism Web Framework（SDKの `Framework/src` をそのままベンダリング。LICENSE.md 同梱）
- `lib/interview/live2d-params.ts`: モデルが面接官として使えるか（口が動くか・うなずけるか）の判定ロジック。公式サンプル4体（Hiyori/Natori/Mao/Mark）の実データで検証済み
- `lib/interview/live2d-capabilities-info.ts`: 上記に渡す情報を組み立てる純粋関数（Core/Framework非依存。テスト用に独立させてある）
- `lib/interview/live2d-loader.ts`: Cubism Core の `<script>` 読み込み
- `lib/interview/live2d-framework-model.ts`: Frameworkを直接使ったモデルのロード・パラメータ設定・描画
- `lib/interview/live2d-motion.ts`: うなずき・まばたきのタイミング（Avatar2D と同じ単純さに合わせてある）
- `app/interview/components/Live2DAvatar.tsx`: 実際に描画するコンポーネント（素のcanvas+WebGL）
- `app/interview/components/InterviewerAvatar.tsx`: `NEXT_PUBLIC_LIVE2D_ENABLED=true` のときだけ Live2D を試し、Core/モデルが無い・要件を満たさない場合は自動で `Avatar2D`（現行の2Dイラスト）へ切り替える

### `pixi-live2d-display` ではなく公式Frameworkを直接使っている理由

最初は `pixi-live2d-display`（2023年公開、以後更新なし）で実装したが、現行配布のCore（Core version 06.00.0001, 2026年以降の「Cubism 5 SDK for Web」同梱）とAPIが非互換で、パッチしても一部しか描画されなかった（`drawables.renderOrders`→`drawOrders`の改名はパッチで直ったが、座標系まわりの非互換が他にも残っていた）。副産物として依存していた `gh-pages`（クリティアルな脆弱性）も無くなった（`npm audit` 0件）。

## 置くもの

```
frontend/public/live2d/
├── README.md                     ← 追跡する
├── shaders/                      ← 追跡する（Framework付属・再配布可、SDKからコピー済み）
├── core/
│   └── live2dcubismcore.min.js   ← Cubism Core（下記参照、各自配置）
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

## 既知の懸念・未確認事項

- **商用利用には別途ライセンス登録が要る。** Framework（`vendor/live2d-framework/LICENSE.md`）は「直近会計年度の売上高が1000万円以上の事業者」に Cubism SDK Release License（出版許諾契約、無料だが登録が必要）への同意を求めている。本番投入前に要確認・要登録（登録先: https://www.live2d.com/ja/download/cubism-sdk/release-license/ ）
- **パネルが小さい**（96〜120px、アスペクト比4:3）。今回の確認は300x400pxの大きめの枠で、全身が綺麗に収まることは確認できた。96px前後でも表情が見えるかは未確認
- まばたき・うなずきのタイミングは Avatar2D と同じ値（まばたき2.5〜6秒間隔・0.45秒のうなずき）を流用しているだけで、Live2Dモデルで見て自然かは未確認
- 物理演算（`physics3.json`、揺れもの）はロード・評価はするが、見た目の確認はまだ
