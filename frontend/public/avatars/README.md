# 面接官アバター

AI面接の面接官として表示する3Dモデルを置く場所。

## いま入っているモデルの問題（#1603）

`male-avatar.glb` / `female-avatar.glb` は **Tripo（画像から3Dを生成するAIツール）製の静止メッシュ**で、動かすための要素を1つも持っていない。GLBを直接解析した結果が次のとおり。

```
generator: Tripo
skins: 0                                   骨格が無い
animations: 0
morph targets: 0                           表情ブレンドシェイプが無い
attributes: POSITION, NORMAL, TEXCOORD_0   JOINTS_0 / WEIGHTS_0 が無い
nodes: 1 / meshes: 1
```

**首の関節も頂点ウェイトも無いので、どんなコードを書いてもうなずかない。口も動かない。** `ThreeAvatar.tsx` は口のモーフ検出（多数の命名パターン）と顎ボーンのフォールバックを実装しているが、モデルがどれも提供しないため全部空振りする。

## 置くべきもの

**VRM を推奨する。** 人型ボーンの割り当てと表情が規格化されているので、うなずき・まばたき・口の動きを実装依存なしに動かせる。

- `male-avatar.vrm`
- `female-avatar.vrm`

`.vrm` が無ければ `.glb` を探す（後方互換）。

### VRoid Studio で作る（推奨）

1. [VRoid Studio](https://vroid.com/studio) を入れる（無料）
2. 面接官らしい見た目のアバターを作る
3. **VRM としてエクスポート**する。書き出し設定で表情（`Blink` / `A` など）を含めること
4. `male-avatar.vrm` / `female-avatar.vrm` としてこのディレクトリへ置く

VRoid の出力はボーン名が `J_Bip_C_Head` のような内部名になるが、`vrm-adapter.ts` が VRM の `humanoid.getNormalizedBoneNode('head')` で引くので名前には依存しない。

### Ready Player Me を使う場合

GLB だが Oculus ビセーム付きで書き出せば動く。

```
https://models.readyplayer.me/[YOUR_AVATAR_ID].glb?morphTargets=Oculus+Visemes&compression=draco
```

商用利用の条件は各自で確認すること。

## 置いたモデルが動くかを確かめる

`lib/interview/avatar-capabilities.ts` の `inspectAvatar` が、読み込んだモデルから動かせる部位を洗い出す。足りない部位があれば**ブラウザのコンソールに原因が1行で出る**。

```
[ThreeAvatar] アバターモデルが要件を満たしていません: 骨格（skin）が無い。静止メッシュなので全身が動かせない / 頭・首のボーンが無いのでうなずけない / ...
```

描画は止めない（面接を止めるより、動かないアバターでも面接を続けるほうがよい）。

要件は次の4つ。優先順は「うなずき > 口 > まばたき」で、うなずきは相手が話を聞いていることを示す最小の動作なので、無いと会話として成立しない。

| 要件 | 無いとどうなるか |
|---|---|
| 骨格（skin） | 全身が動かせない |
| 頭または首のボーン | うなずけない |
| 口のモーフ または 顎ボーン | 口が動かない |
| まばたきのモーフ | まばたきしない |

静止メッシュを置いたら検出できることは `tests/lib/interview/avatar-capabilities.test.ts` で固定している。
