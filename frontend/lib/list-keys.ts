/**
 * リストの要素に、並べ替えても変わらない React の key を付ける（#2088）。
 *
 * 配列の位置（index）を key にすると、並べ替え・途中への追加・削除のときに、React が別の要素の状態（フォーカス・入力中の値・
 * アニメーション）を使い回してしまう。文字列のリストのように ID が無いものは、値から key を作る。
 * 同じ値が重なったときは、2つ目以降に「#何個目か」を付けて重ならないようにする。
 */
export function withStableKeys<T>(
  items: readonly T[],
  toKey: (item: T) => string = (item) => String(item),
): Array<{ key: string; item: T; index: number }> {
  const seen = new Map<string, number>()
  return items.map((item, index) => {
    const base = toKey(item)
    const n = seen.get(base) ?? 0
    seen.set(base, n + 1)
    return { key: n === 0 ? base : `${base}#${n}`, item, index }
  })
}
