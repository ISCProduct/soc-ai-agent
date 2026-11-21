import { withStableKeys } from '@/lib/list-keys'

// #2088: 配列の位置ではなく値から key を作り、並べ替えても同じ要素に同じ key が付くこと
describe('withStableKeys', () => {
  it('値から key を作る', () => {
    expect(withStableKeys(['Go', 'AWS']).map((x) => x.key)).toEqual(['Go', 'AWS'])
  })

  it('並べ替えても、同じ値には同じ key が付く', () => {
    const before = Object.fromEntries(withStableKeys(['Go', 'AWS', 'React']).map((x) => [x.item, x.key]))
    const after = Object.fromEntries(withStableKeys(['React', 'Go', 'AWS']).map((x) => [x.item, x.key]))
    expect(after).toEqual(before)
  })

  it('同じ値が重なったときは、2つ目以降に何個目かを付けて重ならないようにする', () => {
    const keys = withStableKeys(['Go', 'AWS', 'Go', 'Go']).map((x) => x.key)
    expect(keys).toEqual(['Go', 'AWS', 'Go#1', 'Go#2'])
    expect(new Set(keys).size).toBe(keys.length)
  })

  it('toKey で key にする値を選べる。元の要素と位置も返す', () => {
    const roles = [
      { title: 'バックエンド', reason: 'a' },
      { title: 'インフラ', reason: 'b' },
    ]
    expect(withStableKeys(roles, (r) => r.title)).toEqual([
      { key: 'バックエンド', item: roles[0], index: 0 },
      { key: 'インフラ', item: roles[1], index: 1 },
    ])
  })
})
