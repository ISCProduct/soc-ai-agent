# Live2D面接官の資産配送

AI面接の面接官アバターを Live2D で描くには、**リポジトリに入っていない実行時資産**が必要です。
このページは「なぜ入れられないのか」と「どこから来るのか」を書きます。

実装の入口は `frontend/app/interview/components/InterviewerAvatar.tsx`。
`NEXT_PUBLIC_LIVE2D_ENABLED=true` のときだけ Live2D を試し、資産が無い・モデルが
面接官の要件（口・まばたき・うなずき）を満たさない場合は `Avatar2D`（外部画像を使わない
SVG/CSS描画）へ自動で切り替わります。**面接自体は止まりません。**

## リポジトリに置けないもの

| 対象 | ライセンス | 置けるか |
|---|---|---|
| Cubism Core (`live2dcubismcore.min.js`) | Live2D Proprietary Software License の **"Redistributable Code"** | **置けない**（このリポジトリは public） |
| Cubism Web Framework (`frontend/vendor/live2d-framework/`) | Live2D Open Software License | 置ける（**コミット済み**） |
| シェーダ (`frontend/public/live2d/shaders/`) | 同上（Framework付属） | 置ける（**コミット済み**） |
| 書き出し済みモデル（`.moc3` / `.model3.json` / `.cdi3.json` / テクスチャ） | 自社制作物 | 置かない（配布条件がリポジトリと別） |

Core はアプリに同梱して配る分は契約の想定内（だからブラウザへ配信してよい）ですが、
public リポジトリへのコミットは SDK 単体の再配布にあたるため行いません。
除外規則は `.gitignore` の `frontend/public/live2d/*` にあります。

> **売上高1000万円以上の事業者は Cubism SDK リリースライセンス（出版許諾契約）への
> 同意が必要です**（SDK同梱の `LICENSE.md`）。本番公開の前に確認してください。

## どこから来るのか

非公開の S3 バケットに置き、デプロイ時にビルドコンテキストへ展開します。

```
s3://soc-build-assets-508897596159/live2d/
  core/live2dcubismcore.min.js
  interviewers/male/male-interviewer.{model3.json,moc3,cdi3.json}
  interviewers/male/male-interviewer.2048/texture_00.png
  interviewers/female/female-interviewer.{model3.json,moc3,cdi3.json}
  interviewers/female/female-interviewer.2048/texture_00.png
```

バケットはパブリックアクセス全ブロック・バージョニング有効。バケット名は
リポジトリ変数 `LIVE2D_ASSETS_BUCKET` で上書きできます（未設定なら上記が既定）。

展開は `automation/ops/fetch-live2d-assets.sh` が行い、`deployment.yml` の
frontend ビルド直前（staging・本番の両方）で同期的に呼ばれます。
`--delete` は付けません（宛先には Git 追跡下のシェーダと素材が既にあるため）。

**資産が揃っていなければスクリプトが非ゼロで終了し、デプロイが失敗します。**
資産の無いイメージを `NEXT_PUBLIC_LIVE2D_ENABLED=true` で作ると、面接のたびに404を
取ってから Avatar2D に落ちるだけなので、黙って進めません。

### 資産を差し替えるとき

```bash
aws s3 sync ./新しい資産/ s3://soc-build-assets-508897596159/live2d/
```

> **注意: S3 を更新しただけでは本番に反映されません。** デプロイの変更検出
> (`Detect changed services`) は `frontend/` 配下のファイル変更しか見ないため、
> 資産だけ入れ替えてもイメージが再ビルドされません。`frontend/` に変更を伴う
> デプロイか、`workflow_dispatch` の手動実行（全サービスをビルドする経路）が必要です。

### 差分ArtMeshのIDは書き出しのたびに変わる

目閉じ・口開きは、本来 Cubism 側で `ParamEyeLOpen` などにキー設定して動かすものですが、
現在の書き出しにはそのキーが入っていません。そのためコード側
（`frontend/lib/interview/live2d-overlay.ts`）が差分 ArtMesh の不透明度と
IsVisible 動的フラグを毎フレーム上書きして動かしています（暫定対応）。

- 男性: `base=ArtMesh` / `eyesClosed=ArtMesh0` / `mouthOpen=ArtMesh1`
- 女性: `base=ArtMesh1` / `eyesClosed=ArtMesh` / `mouthOpen=ArtMesh0`（男性と逆）

**モデルを書き出し直したら ArtMesh の ID を確認し、`live2d-overlay.ts` を直してください。**
IDが変わると口もまばたきも動かなくなります。残っている Cubism 側の作業は
`frontend/public/live2d/interviewers/CUBISM_STATUS.md` にあります。

## ローカルで試す

1. [live2d.com](https://www.live2d.com/) から Cubism SDK for Web を取得して同意し、
   `Core/live2dcubismcore.min.js` を `frontend/public/live2d/core/` に置く
2. モデル一式を S3 から取る
   ```bash
   LIVE2D_ASSETS_BUCKET=soc-build-assets-508897596159 ./automation/ops/fetch-live2d-assets.sh
   ```
3. `frontend/.env.local` に `NEXT_PUBLIC_LIVE2D_ENABLED=true` を書く
4. `npm run check:live2d` でモデルが面接官の要件を満たすか検査する
   （パラメータ構成と LipSync / EyeBlink 登録を見るだけで、動作は保証しません）

単一モデルで試すときは `NEXT_PUBLIC_LIVE2D_MODEL_URL` を指定すると性別によらずそれを使います。
