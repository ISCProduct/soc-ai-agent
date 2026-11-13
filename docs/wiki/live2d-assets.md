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
| 制作用の元絵・Cubismプロジェクト（`source/` / `cubism-input/` / `*.cmo3`） | 自社制作物 | 置かない（**容量**。下の「制作用の資産」参照） |

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

**`live2d/` の下には実行時に要るものだけを置いてください。** このプレフィックスは
まるごと `frontend/public/live2d/` へ同期され、`public/` はブラウザへ配信されます。
制作用の元絵をここに混ぜると、全ビルドのイメージに入って公開されます。制作用は
`live2d-authoring/` に分けてあります（次節）。

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

## 制作用の資産（モデルを描き直す人だけ）

元絵（`source/`）、Cubism 取り込み用の 2048px PNG（`cubism-input/`）、Cubism Editor の
プロジェクト（`cubism-wip/*.cmo3`）は、合わせて 25MB あります。アプリを動かすのに
は要らず、モデルを描き直す人しか開きません。`.cmo3` は書き出すたびに数MBの新しい
blob が履歴へ積み上がるため、リポジトリから外して S3 へ置いています。

```
s3://soc-build-assets-508897596159/live2d-authoring/
  docs/avatar/live2d-interviewers/cubism-wip/{male,female}-interviewer.cmo3
  frontend/public/live2d/interviewers/source/{male,female}/...
  frontend/public/live2d/interviewers/cubism-input/{male,female}/...
```

キーはリポジトリ相対のパスです。ルートへ sync すれば各ファイルが本来の場所へ戻ります。

### 取ってくる

```bash
LIVE2D_ASSETS_BUCKET=soc-build-assets-508897596159 ./automation/ops/fetch-live2d-authoring.sh
```

`.gitignore` で除外してあるので、展開しても `git status` は汚れません。

### 描き直したあと

Cubism プロジェクトと、そこから書き出した実行時モデルは、置き場所が違います。
**両方を上げてください。**

```bash
# 1. プロジェクトと元絵（制作用）
aws s3 sync ./docs/avatar/live2d-interviewers/cubism-wip/ \
  s3://soc-build-assets-508897596159/live2d-authoring/docs/avatar/live2d-interviewers/cubism-wip/

# 2. 書き出した実行時モデル（これが本番で配られる）
aws s3 sync ./frontend/public/live2d/interviewers/female/ \
  s3://soc-build-assets-508897596159/live2d/interviewers/female/
```

バケットはバージョニング有効なので、上書きしても前の版は残ります。

> **注意: `.cmo3` と S3 上の `.moc3` がずれていないか確認してください。** 2026-11-13 に、
> リポジトリの `female-interviewer.cmo3` が 10-28 の平面1枚版のまま取り残され、S3 の
> 女性モデル（3 ArtMesh 版）の書き出し元がローカルの作業コピーにしか無い状態に
> なっていました。書き出したら必ず 1. も上げてください。

## ローカルで試す

1. [live2d.com](https://www.live2d.com/) から Cubism SDK for Web を取得して同意し、
   `Core/live2dcubismcore.min.js` を `frontend/public/live2d/core/` に置く
2. 資産を S3 から取る（`core/` も含めて同期するので、手順1を済ませていれば上書きされるだけ）
   ```bash
   LIVE2D_ASSETS_BUCKET=soc-build-assets-508897596159 ./automation/ops/fetch-live2d-assets.sh
   ```
   バケットへの読み取り権限が無い場合はここで失敗します。その場合はモデルを
   別途受け取って `frontend/public/live2d/interviewers/{male,female}/` に置いてください。
3. `frontend/.env.local` に `NEXT_PUBLIC_LIVE2D_ENABLED=true` を書く
4. モデルが面接官の要件を満たすか検査する（`frontend` で実行する。
   スクリプトは `frontend/package.json` にしかない）
   ```bash
   cd frontend
   npm run check:live2d -- public/live2d/interviewers/male
   npm run check:live2d -- public/live2d/interviewers/female
   ```
   パラメータ構成と LipSync / EyeBlink 登録を見るだけで、動作は保証しません。
   面接官として使うには `ParamAngleY` か `ParamAngleX` のどちらかが必要です
   （`live2d-params.ts` の `isUsableAsInterviewer`。両方無いモデルは不適格と
   判定され Avatar2D に切り替わります）。

モデルを**描き直す**場合は、上の「制作用の資産」も取ってください。アプリを動かすだけなら要りません。

単一モデルで試すときは `NEXT_PUBLIC_LIVE2D_MODEL_URL` を指定すると性別によらずそれを使います。
ただし**差分ArtMeshの対応は `gender` で選ばれたまま**なので、男性モデルを女性として表示すると
口開き差分が出たままになります。試すモデルに合う性別を選んでください。
