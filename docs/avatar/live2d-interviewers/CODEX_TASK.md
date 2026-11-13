# 作業指示書: Live2D面接官モデルの本リギング（Codex向け、2026-10-30 更新）

## 【2026-10-30追記】コード側で直せる分は直した。残るブロッカーはCubism側のみ

「女性モデルが実画面で一切表示されない」件を調査し、原因が3つ重なっていたことが分かった。
うち2つはコード側の不具合だったので修正済み（`frontend/lib/interview/live2d-framework-model.ts`、
`frontend/lib/interview/live2d-overlay.ts`）。**残り1つは画像データそのものの問題なので、
Cubism Editor側での修正が必要。**

1. **（修正済み・コード側）通常絵（base）のdrawableの不透明度がCore既定で0だった**
   → 差分2枚（目閉じ・口開き）と同様に、毎フレーム1へ強制するよう修正済み。
2. **（修正済み・コード側）通常絵（base）のdrawableの`IsVisible`動的フラグ（bit 0）もCore既定でfalseだった**
   → 不透明度とは別のフラグで、falseだとレンダラーが不透明度を無視してそのdrawable自体を
   描画スキップする（公式Framework `cubismrenderer_webgl.ts` 973行目参照）。これも毎フレーム
   強制的に立てるよう修正済み。**この2つを両方直して初めて女性モデルは画面に映るようになった**
   （男性モデルは元々この2つとも既定値が正しかったため、今回の修正は無害な上書きになっている。
   実機で男女とも再確認し、退行がないことを確認済み）。
3. **（未修正・Cubism Editor側の作業が必要）通常絵（base）のdrawableが、テクスチャアトラス内の
   誤った領域（まばたき差分の画像）を参照している**
   → `frontend/public/live2d/interviewers/female/female-interviewer.2048/texture_00.png` を
   直接確認したところ、アトラスには3枚の絵（左上=まばたき中、右上=口開き中、左下=通常/目を開いた
   状態）が並んでいるが、**baseのUVは左上（まばたき中の絵）を指しており、左下（本来の通常絵）を
   指していない。** 上記1・2を直した結果、女性モデルは画面に映るようにはなったが、
   **常に「目を閉じて微笑んでいる」状態で止まって見える**（`source/female/female-blink.png`と
   見比べると一致する）。これはCore側の不透明度/可視性フラグの問題ではなく、**baseのArtMeshに
   割り当てられたテクスチャ座標（UV）そのものが誤っている**ため、コード側では直せない。
   Cubism Editorで、baseのArtMeshに割り当てる画像（またはUV座標）を、左下の「通常（目を開いた）」
   絵に差し替えて再書き出しすること。

## 【経緯】女性モデルが実画面で一切表示されない（2026-10-29夜の調査、1は上記1・2で解消済み）

2026-10-29夜に実機確認した結果、女性モデル（`frontend/public/live2d/interviewers/female/`）は
**`npm run check:live2d`には合格するが、実際にWebGLで描画すると何も映らない**ことを確認した
（React Strict Modeの二重実行が原因かと疑ったが、Strict Modeを無効化しても再現したため別原因）。

原因を`draw()`に一時的なログを仕込んで特定済み:

```
model.getCanvasWidth()  = 1  （本来は rigging-spec.json の sourceCanvas.width=1289 相当の値になるはず）
model.getCanvasHeight() = 1  （同様に height=1220 相当のはず）
```

`canvasinfo`（CanvasWidth/CanvasHeight、Cubismのunit空間の基準値）が**1×1という異常値**で
書き出されている。これにより描画スケールが実質500倍以上に拡大され、キャラクターが完全に
画面外へ飛んでしまい、キャンバスは常に透明（何も描画されない）。男性モデルでは同じコードで
正常に表示されるため、コード側の不具合ではなく、**女性の`.moc3`書き出し自体の不具合**と判断する。

このため、「男女とも `check:live2d` 合格」は構造上のチェックに過ぎず、女性は実機では
面接官としてまったく機能しない状態。`ParamMouthOpenY`/`ParamEyeLOpen`にキーフォームを設定した
という報告も、この描画不具合のせいで実画面で視覚的に検証できていない。

**対応状況（2026-10-30更新）**: このcanvasinfoの不一致自体は、コード側で回避済み
（`live2d-framework-model.ts`の`measureExtent()`。Core側の申告値ではなく、全drawableの頂点から
実際の絵のサイズを毎回測ってスケール計算に使うよう変更した。男女とも実機で退行なしを確認済み）。
そのためCubism Editor側でキャンバス/アートボードサイズを直すことは**必須ではなくなった**。
ただし、申告値と実寸がずれていること自体はEditor側の設定ミスなので、他の作業（パーツ分離や
デフォーマ追加）のついでに直しておくと良い（`rigging-spec.json`の`sourceCanvas`参照）。
女性が実際には映らなかった直接の原因は、この節の先頭に書いた3つの不具合で、スケールはその
一部に過ぎなかった。

## 前提・ゴール

`soc-ai-agent` のAI面接官アバターをLive2Dに置き換える作業は、別ブランチに移って進んでいる。

- ブランチ: `feature/live2d-interviewer-models`（originにpush済み）
- 作業ディレクトリ: `/Users/oohashikazuyuki/soc-ai-agent-mock-live2d`（専用worktree）
- 最新の状態ドキュメント（正）: `frontend/public/live2d/interviewers/CUBISM_STATUS.md`
  （`docs/avatar/live2d-interviewers/CUBISM_STATUS.md` は10/28時点の古い英語版で、以後更新されていない。読むなら前者を参照すること）

実描画側（Cubism Web Framework を直接使う方式）は実装・検証済みで、モデルファイルさえ揃えば動く。
**このタスクはCubism Editor（GUIアプリ）での作業が中心。**

## 現在の状態（実機確認済み）

- 男女とも正規の `.moc3`/`.model3.json`/`.cdi3.json`/2048pxテクスチャを書き出し済み
  （`frontend/public/live2d/interviewers/{male,female}/`。`xxd`でマジックバイト`MOC3`を確認済み、偽物ではない）
- `npm run check:live2d` は男女とも合格（`LipSync`に`ParamMouthOpenY`、`EyeBlink`に
  `ParamEyeLOpen`/`ParamEyeROpen`を登録済み）
- **ただし、この合格はパラメータとグループの申告を見ているだけで、実際の変形キーフォームは
  男女ともゼロ件。** Cubism Coreでパラメータを0/1に振っても、書き出し済み`.moc3`内のdrawable不透明度・
  頂点位置は変化しない（実機で確認済み）
- 口パク・まばたきは、`frontend/lib/interview/live2d-overlay.ts` が**コード側で差分ArtMeshの
  不透明度を毎フレーム上書きする暫定対応**によって実画面で動作確認済み（2026-10-29）。
  これはCubism側の本リギングが未完了であることの回避策であり、正式な解決ではない。
  再書き出しでArtMeshのID（`OVERLAY_MESH_IDS`に定義）が変わると動かなくなる点に注意。
- うなずき（`ParamAngleY`）だけは、口・目と違って「差分画像の重ね合わせ」で代用できない
  （頭部全体の向きが変わる変形のため）。今のところ無反応。**優先度は低く、後回しでよいと
  ユーザーが明示的に判断済み（2026-10-29）。** コード側の暫定代用（CSS回転等）も今回は行わない。
- **面接官は現在、男性固定でフロント・バックエンドとも女性への切り替えを無効化している**
  （`2839225a`で意図的に変更済み）。理由は、女性モデルがまだコード側の暫定対応
  （`live2d-overlay.ts`）の対象になっておらず、表情が動かない静止画のまま表示されてしまうため。
  **この本リギングが男女とも完了し、`npm run check:live2d`と実機確認の両方を満たしたら、
  男女切り替えを復活させる作業（フロント・バックエンドの再有効化）に進んでよい**
  （これはCodexの範囲外。本リギング完了後にこちら側で対応する）。
- 元絵は男女とも1枚の平面PNG（レイヤー分けされていない）。取り込んだCubismプロジェクトも
  全身1枚のArtMeshのまま、パーツ分離が行われていない。

## やってほしいこと

`rigging-spec.json`（`frontend/public/live2d/interviewers/rigging-spec.json`）の
`requiredParameters`のうち、**`ParamMouthOpenY` / `ParamEyeLOpen` / `ParamEyeROpen`**に
実際の変形キーフォームを設定し、コード側の暫定対応（`live2d-overlay.ts`）が不要になる状態まで
持っていくことを優先する。

`ParamAngleY`（うなずき）は今回のスコープ外。優先度が低いとユーザーが判断済みのため、
無理に対応しなくてよい（パラメータ自体は既にrigging-spec通り定義されている想定なので、
`check:live2d`は通る）。余裕があれば着手してもよいが、必須条件ではない。
`recommendedParameters`（`ParamAngleX`, `ParamBreath`）も同様に余裕があれば。

### 使う編集用ファイル

- 男性: `docs/avatar/live2d-interviewers/cubism-wip/male-interviewer.cmo3`
  （直近の書き出し実績があるファイル。`male-interviewer-WIP.cmo3`は古いバックアップなので使わない）
- 女性: `docs/avatar/live2d-interviewers/cubism-wip/female-interviewer.cmo3`

### 手順

1. 対象の`.cmo3`をCubism Editorで開く。現状は全身1枚のArtMeshなので、まず顔・目・目玉・まゆ毛・
   口・鼻・耳・前髪・横髪・後ろ髪などへパーツ分離する
   （参考画像: `frontend/public/live2d/interviewers/source/{male,female}/` の差分・パーツ切り出し）
2. パーツ階層に対応するワープ/回転デフォーマを作成し、パーツをデフォーマ配下にネストする
3. 各パラメータについて、値を変化させたときの変形をキーフォームとして記録する
   - `ParamMouthOpenY` 0→1: `*-mouth-open.png` を参考に口を開いた形状へ
   - `ParamEyeLOpen` / `ParamEyeROpen` 1→0: `*-blink.png` と `parts/eyes-closed.png` を参考に
     まぶたを閉じた形状へ
   - `ParamAngleY`（うなずき）は今回対応不要（スコープ外）
   - （推奨）`ParamAngleX`, `ParamBreath`
4. `LipSync`/`EyeBlink`グループは既に登録済みなので確認のみでよい
5. テクスチャアトラスを2048px・1枚で再生成する（既存の2048px書き出しを維持）
6. `.cmo3`を同じパスに上書き保存する
7. MOC3形式で書き出す（`.cdi3.json`を含める）。書き出し先: `frontend/public/live2d/interviewers/{male,female}/`
   （既存の書き出し済みファイルを上書きする）

### 検証

```sh
cd frontend
npm run check:live2d -- public/live2d/interviewers/male
npm run check:live2d -- public/live2d/interviewers/female
```

これに加えて、**Cubism Editor上でパラメータスライダーを動かし、実際に絵が変形することを
目視確認する**こと。`check:live2d`の合格だけでは変形の有無を判定できない（今回それで一度
誤解が生まれている）。

実画面での確認:

```sh
npm run dev
# http://localhost:3000/live2d-preview （開発用の確認ページ。PRには含めない）
```

96〜120px相当の小さい表示枠を想定し、口パク・まばたきが実際に動いて見えることを確認する
（うなずきは今回のスコープ外なので確認不要）。

本リギングが完了し動作確認できたら、`frontend/lib/interview/live2d-overlay.ts` の暫定上書きが
不要になる（`model.setOverlayState(...)`の呼び出しを削除してよい）。ただし、この削除は
コード側の変更なので、Cubism側の変形が実際に機能していることを目視確認してから行うこと。

## 絶対にやってはいけないこと

- 偽の`.moc3`/`.model3.json`や、`check:live2d`の判定だけすり抜けるダミーファイルの作成
- 判定ロジック（`frontend/lib/interview/live2d-params.ts`）やテストを、特定のモデルに
  合わせて都合よく書き換えること
- `frontend/public/live2d/core/`（Live2D Proprietary Software Licenseのバイナリ）をコミットすること
  （`.gitignore`で除外されている）
- `frontend/public/live2d/interviewers/CUBISM_STATUS.md` を実測と異なる内容に書き換えること
  （チェックボックスは実際に確認できたものだけ付ける）

## 完了条件

- [ ] 女性: baseのArtMeshが、テクスチャアトラス内の「通常（目を開いた）」絵を指すように修正し、
      再書き出しする（冒頭【2026-10-30追記】の3番目参照。現状は「まばたき」の絵を指している）
- [ ] `ParamMouthOpenY` / `ParamEyeLOpen` / `ParamEyeROpen` の3つに
      Cubism Editor上で実際の変形キーフォームが設定されている（`ParamAngleY`はスコープ外）
- [ ] 男女とも `npm run check:live2d` が合格
- [ ] `/interview` 相当の小さい表示枠で、口パク・まばたきが実際に動いて見えることを目視確認する
- [ ] `frontend/public/live2d/interviewers/CUBISM_STATUS.md` を実測結果に更新する
- [ ] `.cmo3`（編集用）を `docs/avatar/live2d-interviewers/cubism-wip/` に保存する
- [ ] 本リギングで動作することを確認できたら、`live2d-overlay.ts` の暫定対応を外せるか検討し、
      外す場合はその旨をコミットメッセージに明記する
