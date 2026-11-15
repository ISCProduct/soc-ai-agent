import { barWidthPercent, buildAxisItems, findLowestLowAxis, isLowAxis } from '@/app/results/components/list/axisStatus'
import { buildMetaLine, countByTab, isThinEvidence, matchesTab, maxMatchedAxisCount } from '@/app/results/components/list/companyRules'

describe('companyRules', () => {
  const list = [
    { isFavorited: true, isApplied: false, matchedAxisCount: 9 },
    { isFavorited: false, isApplied: true, matchedAxisCount: 6 },
    { matchedAxisCount: undefined },
  ]

  it.each([
    ['all', 3],
    ['favorite', 1],
    ['applied', 1],
  ] as const)('%s タブの件数と絞り込みが一致する', (tab, n) => {
    expect(countByTab(list)[tab]).toBe(n)
    expect(list.filter((c) => matchesTab(c, tab))).toHaveLength(n)
  })

  it('最大根拠軸数は未設定を0として数える', () => {
    expect(maxMatchedAxisCount(list)).toBe(9)
    expect(maxMatchedAxisCount([])).toBe(0)
  })

  it.each([
    [6, 9, false, true],
    [9, 9, false, false],
    [6, 9, true, false], // 暫定評価では個別の印を出さない
    [undefined, 9, false, false],
  ])('根拠が薄い行の判定 count=%s max=%s provisional=%s', (count, max, prov, want) => {
    expect(isThinEvidence({ matchedAxisCount: count }, max, prov)).toBe(want)
  })

  it('メタ情報は空を除いて全角スペースでつなぐ', () => {
    expect(buildMetaLine({ industry: '情報通信', location: '', employees: '120名' })).toBe('情報通信　120名')
  })
})

describe('axisStatus', () => {
  const items = buildAxisItems({ job: 100, interest: 40, aptitude: 55, future: 20 })

  it('50未満だけを低い軸とする（50は含まない）', () => {
    expect(isLowAxis(49)).toBe(true)
    expect(isLowAxis(50)).toBe(false)
  })

  it('最低の軸を選ぶ。全軸50以上なら null', () => {
    expect(findLowestLowAxis(items)?.label).toBe('将来')
    expect(findLowestLowAxis(buildAxisItems({ job: 50, interest: 60, aptitude: 70, future: 80 }))).toBeNull()
  })

  it('棒の幅は0〜100に丸める', () => {
    expect(barWidthPercent(-5)).toBe(0)
    expect(barWidthPercent(130)).toBe(100)
    expect(barWidthPercent(42)).toBe(42)
  })
})
