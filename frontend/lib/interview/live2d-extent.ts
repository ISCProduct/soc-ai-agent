/**
 * 書き出し済みモデルの「実際の絵の大きさ」を頂点から測る。
 *
 * `live2d-framework-model.ts` に置かないのは、あのモジュールが公式Framework経由で
 * `Live2DCubismCore` のグローバルを読み込み時に要求するため。素のまま import できないと
 * テストから呼べないので、Core非依存の計算だけここに分けてある
 * （`live2d-capabilities-info.ts` と同じ方針）。
 */

/**
 * drawable の頂点配列（`[x0, y0, x1, y1, ...]` の並び）をまとめて囲む矩形の幅・高さを返す。
 *
 * Cubism Editor のキャンバス設定（`getCanvasWidth()/getCanvasHeight()`）は書き出しに
 * よって実際の絵のサイズと食い違う（女性モデルで確認: 申告値が実寸の約0.6倍しかなく、
 * 拡大されて画面外へはみ出し何も映らなかった）。見た目のスケールは実測から出す。
 *
 * 頂点が1つも無ければ幅・高さ 0 を返す（呼び出し側はこれを描画しない条件に使う）。
 * 末尾の y が欠けた奇数長は、その x を無視する。
 */
export function verticesExtent(vertexArrays: ArrayLike<number>[]): { width: number; height: number } {
  let minX = Number.POSITIVE_INFINITY
  let minY = Number.POSITIVE_INFINITY
  let maxX = Number.NEGATIVE_INFINITY
  let maxY = Number.NEGATIVE_INFINITY
  for (const vertices of vertexArrays) {
    for (let i = 0; i + 1 < vertices.length; i += 2) {
      const x = vertices[i]
      const y = vertices[i + 1]
      if (x < minX) minX = x
      if (x > maxX) maxX = x
      if (y < minY) minY = y
      if (y > maxY) maxY = y
    }
  }
  if (!Number.isFinite(minX) || !Number.isFinite(minY)) return { width: 0, height: 0 }
  return { width: maxX - minX, height: maxY - minY }
}
