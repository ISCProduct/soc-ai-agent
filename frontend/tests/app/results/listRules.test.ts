import { AXIS_LEVEL_LABEL, axisLevel, buildAxisItems, buildNextStep, findWeakestAxis } from '@/app/results/components/list/axisStatus'
import { buildMetaLine, isThinEvidence, maxMatchedAxisCount } from '@/app/results/components/list/companyRules'

describe('companyRules', () => {
  const list = [{ matchedAxisCount: 9 }, { matchedAxisCount: 6 }, { matchedAxisCount: undefined }]

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
  it('80以上は「よく聞けています」、40以上は「やや少ない」、それ未満は「まだ少ない」', () => {
    expect(axisLevel(80)).toBe('good')
    expect(axisLevel(79)).toBe('some')
    expect(axisLevel(40)).toBe('some')
    expect(axisLevel(39)).toBe('few')
    expect(AXIS_LEVEL_LABEL.some).toBe('やや少ない')
  })

  const items = buildAxisItems({ job: 100, interest: 40, aptitude: 55, future: 20 })

  it('「よく聞けています」に届いていない軸のうち最低のものを選ぶ。全て届いていれば null', () => {
    expect(findWeakestAxis(items)?.label).toBe('将来')
    expect(findWeakestAxis(buildAxisItems({ job: 80, interest: 90, aptitude: 100, future: 80 }))).toBeNull()
  })

  it('同じ値なら並びの先頭を選ぶ', () => {
    expect(findWeakestAxis(buildAxisItems({ job: 60, interest: 60, aptitude: 90, future: 90 }))?.label).toBe('職種')
  })

  it('次にやることの文と導線', () => {
    const weakest = findWeakestAxis(items)
    expect(buildNextStep(weakest, false)).toEqual({
      message: '将来の話がまだ少ないため、チャットで続けると順位が変わることがあります。',
      showChatAction: true,
    })
    expect(buildNextStep(null, false).showChatAction).toBe(false)
    expect(buildNextStep(null, true).showChatAction).toBe(true) // 暫定評価は、軸が揃っていても続けてもらう
  })
})
