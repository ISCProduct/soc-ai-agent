import { needsLowMatchConfirm, LOW_MATCH_THRESHOLD, MIN_MATCHED_AXES_FOR_LOW_MATCH } from '@/lib/low-match'

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

// マッチ度は「計測できた軸だけ」の平均なので、軸が少ない学生は実力ではなく
// 計測不足で低い値になる（#1124）。そこに確認ダイアログを出すと
// 「スコアが取れていないこと」を「スコアが低いこと」として扱ってしまう。
describe('算出軸が少ない場合', () => {
  it.each([
    ['軸が0本', 0],
    ['軸が1本', 1],
    ['軸が3本（下限未満）', 3],
  ])('%s なら低スコアでも確認を出さない', (_label, axes) => {
    expect(needsLowMatchConfirm(10, axes as number)).toBe(false)
  })

  it('下限ちょうどなら従来どおり判定する', () => {
    expect(needsLowMatchConfirm(10, MIN_MATCHED_AXES_FOR_LOW_MATCH)).toBe(true)
    expect(needsLowMatchConfirm(90, MIN_MATCHED_AXES_FOR_LOW_MATCH)).toBe(false)
  })

  it.each([
    ['未指定', undefined],
    ['null', null],
  ])('軸数が %s なら従来どおりスコアだけで判定する', (_label, axes) => {
    // 古いレスポンスとの互換。ここで false に倒すと警告が全く出なくなる
    expect(needsLowMatchConfirm(10, axes as undefined)).toBe(true)
  })
})
