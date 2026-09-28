import {
  canDismiss,
  withDismissing,
  withoutDismissing,
} from '@/lib/teacher-guidance'

// 学生側ホームの閉じるボタンは進行中の状態を持っておらず、連打すると同じ案内へ
// 何度も POST が飛んでいた。教員側の送信ボタンは disabled で防いでいるのに、
// 学生側だけ抜けていた。
//
// app/page-content.tsx は jsdom で描画できない（依存が重く OOM する）ため、
// 判定だけをここで固定する。
describe('教員案内の閉じる操作の多重実行防止', () => {
  it('進行中でなければ受け付ける', () => {
    expect(canDismiss(new Set(), 1)).toBe(true)
  })

  it('進行中なら受け付けない（連打で二重POSTを防ぐ）', () => {
    expect(canDismiss(new Set([1]), 1)).toBe(false)
  })

  it('別の案内は進行中でも受け付ける', () => {
    expect(canDismiss(new Set([1]), 2)).toBe(true)
  })

  it('連打を模しても2回目以降は弾かれる', () => {
    let ids = new Set<number>()
    const accepted: number[] = []
    for (let i = 0; i < 5; i++) {
      if (canDismiss(ids, 1)) {
        accepted.push(1)
        ids = withDismissing(ids, 1)
      }
    }
    expect(accepted).toHaveLength(1)
  })

  it('完了後は再度受け付ける（失敗したら閉じ直せる）', () => {
    let ids = withDismissing(new Set<number>(), 1)
    expect(canDismiss(ids, 1)).toBe(false)
    ids = withoutDismissing(ids, 1)
    expect(canDismiss(ids, 1)).toBe(true)
  })

  // Set を直接変更すると React が同一参照と見て再描画しない。
  // その場合ボタンが無効化されず、連打防止が効かなくなる。
  it('新しい Set を返す（参照が変わらないと再描画されない）', () => {
    const before = new Set<number>([1])
    const added = withDismissing(before, 2)
    const removed = withoutDismissing(before, 1)

    expect(added).not.toBe(before)
    expect(removed).not.toBe(before)
    expect(before.has(2)).toBe(false)
    expect(before.has(1)).toBe(true)
  })
})
