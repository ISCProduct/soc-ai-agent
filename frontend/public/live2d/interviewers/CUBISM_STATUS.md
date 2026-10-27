# Cubism 制作状況

最終確認日: 2026-10-28

## 完了したもの

- 男性・女性の面接官向け立ち絵（正面、バストアップ、透過 PNG）
- 男女それぞれの通常・まばたき・口開き素材
- 96 px / 120 px での視認性確認用プレビュー
- Cubism 取り込み用 2048 x 2048 PNG
- パーツ分け用の目閉じ・口開き差分
- `ParamAngleY`、`ParamEyeLOpen`、`ParamEyeROpen`、`ParamMouthOpenY` を含むリギング仕様
- Live2D Cubism Editor 5.3.04 FREE のインストールと起動確認
- 男性の 2048 PNG を Cubism のアートメッシュとして取り込み確認
- `~/Desktop/Untitled Model.cmo3` の実ファイルとCubism 5.3.04での再オープンを確認
- 編集用バックアップを `docs/avatar/live2d-interviewers/cubism-wip/` へ保存

## 未完了のもの

- `.cmo3` の保存
- パラメータへのキー割り当てと変形調整
- テクスチャアトラス生成
- `.moc3`、`.model3.json`、`.cdi3.json` の書き出し
- 女性モデルの Cubism 取り込みとリギング
- `npm run check:live2d -- <model-dir>` の合格

Cubism Editor の Java 製保存ダイアログが自動操作からファイル名・保存先の入力を受け付けず、実ファイルを書き出せなかった。`.moc3` は独自バイナリであり、正規の Cubism 書き出しを経ずに代用品を生成していない。

追加確認の結果、男性WIPの「顔/目/口/髪」などは名前付きの空パーツフォルダーで、表示実体は全身画像1枚の `male-neutral-2048` ArtMeshのみだった。独立した目・口のリグには、差分PNGの追加ArtMesh化またはレイヤー分けPSDへの置き換えが必要。

## Cubism で残っている手順

1. `cubism-input/male/male-neutral-2048.png` を取り込んだモデルを `runtime/male/male-interviewer.cmo3` として保存する。
2. 目、口、顔を差分素材に合わせてアートメッシュへ分離する。
3. `rigging-spec.json` のパラメータにキーを追加する。
4. 2048 x 2048 のテクスチャアトラスを作成する。
5. `runtime/male` へ組み込み用ファイル一式を書き出す。
6. 女性も `cubism-input/female/female-neutral-2048.png` で同様に作成する。
7. `frontend` で男女それぞれに `npm run check:live2d -- public/live2d/interviewers/runtime/<gender>` を実行する。

`runtime` には正規書き出しが完了するまで、偽の `.moc3` や検証をすり抜けるだけの JSON を置かない。
