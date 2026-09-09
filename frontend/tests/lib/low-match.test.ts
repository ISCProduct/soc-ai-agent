import { needsLowMatchConfirm, LOW_MATCH_THRESHOLD } from '@/lib/low-match'

// マッチ度が低い企業への応募時に確認を出すかの判定（#1028）。
// 出しすぎると邪魔になり、出さなすぎると気づけない。境界を固定する。
describe('needsLowMatchConfirm', () => {
  it.each([
    [0, true],
    [39, true],
    [39.9, true],
    [LOW_MATCH_THRESHOLD, false], // 境界値ちょうどは出さない（DesignDoc の指定）
    [41, false],
    [100, false],
  ])('スコア %s → 確認 %s', (score, want) => {
    expect(needsLowMatchConfirm(score)).toBe(want)
  })

  // 「スコアが取れていない」と「スコアが低い」は違う。
  // 前者で警告すると、算出前の企業すべてに警告が出る。
  it.each([
    ['undefined', undefined],
    ['null', null],
    ['NaN', NaN],
  ])('スコアが %s のときは確認しない', (_label, score) => {
    expect(needsLowMatchConfirm(score as number | null | undefined)).toBe(false)
  })

  it('閾値は40', () => {
    expect(LOW_MATCH_THRESHOLD).toBe(40)
  })
})

// スコアが数値として成立しない場合を明示的に除外していること。
// NaN < 40 は false なので結果は同じだが、
// 判定を「数値であること」から始めないと、後で比較演算を
// 変えたときに NaN が確認対象へ紛れ込む。
describe('needsLowMatchConfirm の入力検証', () => {
  it.each([
    ['Infinity', Infinity],
    ['-Infinity', -Infinity],
  ])('%s は確認しない', (_l, v) => {
    expect(needsLowMatchConfirm(v)).toBe(false)
  })
})
