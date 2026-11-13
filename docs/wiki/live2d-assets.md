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
make live2d-authoring
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

```bash
make live2d-up
```

これ一つで、資産の取得・`frontend/.env.local` のフラグ・モデルの検査まで済みます。
あとは**devサーバーを起動し直して** `/interview` を開いてください。`NEXT_PUBLIC_*` は
ビルド時にバンドルへ焼き込まれるため、起動中の dev サーバーには反映されません。

別のバケットを使うときは `make live2d-up LIVE2D_ASSETS_BUCKET=...`。

### 中で何をしているか

1. `automation/ops/fetch-live2d-assets.sh` で S3 から Core とモデルを取る。
   **Cubism Core も S3 に入っているので、`live2d.com` から SDK を落とす必要はありません**
   （バケットへの読み取り権限がある場合）。権限が無ければここで失敗するので、その場合は
   SDK の `Core/live2dcubismcore.min.js` を `frontend/public/live2d/core/` に、モデルを
   `frontend/public/live2d/interviewers/{male,female}/` に、それぞれ別途受け取って置きます。
2. `frontend/.env.local` に `NEXT_PUBLIC_LIVE2D_ENABLED=true` が無ければ追記する（既にあれば触らない）。
3. 男女のモデルが面接官の要件を満たすか検査する。

```bash
# 検査だけやり直すとき（frontend で実行する。スクリプトは frontend/package.json にしかない）
cd frontend
npm run check:live2d -- public/live2d/interviewers/male
npm run check:live2d -- public/live2d/interviewers/female
```

検査はパラメータ構成と LipSync / EyeBlink 登録を見るだけで、動作は保証しません。
面接官として使うには `ParamAngleY` か `ParamAngleX` のどちらかが必要です
（`live2d-params.ts` の `isUsableAsInterviewer`。両方無いモデルは不適格と判定され
Avatar2D に切り替わります）。

取得した資産は `.gitignore` で除外されているので、`git status` は汚れません。

モデルを**描き直す**場合は `make live2d-authoring` で制作用の資産も取ってください。
アプリを動かすだけなら要りません。

単一モデルで試すときは `NEXT_PUBLIC_LIVE2D_MODEL_URL` を指定すると性別によらずそれを使います。
ただし**差分ArtMeshの対応は `gender` で選ばれたまま**なので、男性モデルを女性として表示すると
口開き差分が出たままになります。試すモデルに合う性別を選んでください。
