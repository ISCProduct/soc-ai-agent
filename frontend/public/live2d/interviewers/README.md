# Live2D 面接官モデル制作素材

`soc-ai-agent` の面接官モデル（男性・女性）の Cubism 制作用素材です。
画像は正面バストアップ・透明背景で、96〜120px表示でも口と目の状態が判別できることを確認しています。

## 収録内容

```text
source/
├── male/
│   ├── male-neutral.png
│   ├── male-blink.png
│   ├── male-mouth-open.png
│   ├── parts/            # Cubismで目・口を分離するときの切り出し参考
│   └── previews/         # 96px / 120pxの視認性確認用
└── female/
    └── （同構成）
rigging-spec.json         # 必須パラメータと書き出し設定
```

`neutral`、`blink`、`mouth-open` は同じキャンバス寸法です。差分画像は自動生成されたため、
Cubismへは3枚を単純に重ねるのではなく、目・口の部位を描画オブジェクトとして切り出し、
中間形状を補間してください。

## Cubism Editorでの作業

1. `neutral` を基準に顔、髪、目、口、首、胴体を分離する。
2. `blink` を参照し、左右の目を別ArtMeshにして `ParamEyeLOpen` / `ParamEyeROpen` を設定する。
3. `mouth-open` を参照し、上唇・下唇・口内を分離して `ParamMouthOpenY` を 0〜1 に設定する。
4. 顔・首・髪・胴体のデフォーマを作り、上下方向のうなずきを `ParamAngleY` -30〜30 に設定する。
5. `EyeBlink` と `LipSync` のパラメータグループを設定する。
6. 2048pxアトラス1枚を作成し、MOC3書き出し時に `.cdi3.json` を含める。

必須の最終成果物は、各モデルについて以下です。

```text
<name>.model3.json
<name>.moc3
<name>.cdi3.json
<name>.2048/texture_00.png
<name>.cmo3                 # 社内改修用。ランタイムには不要
```

書き出し後は次を実行します。

```sh
cd frontend
npm run check:live2d -- public/live2d/interviewers/models/<name>
```

`.moc3` / `.cmo3` は Cubism Editor 固有のバイナリであり、画像素材やJSONの手作業では代替できません。

## 生成プロンプトの要点

- 日本の就活支援SaaSで使う、落ち着いた成人の面接官
- 正面・左右対称・バストアップ・透明背景
- ビジネススーツ／オフィスカジュアル、成熟した等身、控えめな配色
- 96〜120pxでも目と口が判別できること
- 文字、ロゴ、透かし、小物、手、誇張したポーズを含めないこと

画像生成にはCodexの組み込みImageGenを使用しました。
