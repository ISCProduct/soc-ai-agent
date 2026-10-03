/**
 * VRM の表情・ボーンの取り出しを固定する。
 *
 * VRoid の VRM はボーン名が 'J_Bip_C_Head' で、表情も expressionManager 経由。
 * 名前のパターン照合に頼ると実装依存になるので、規格のAPIを使っていることを留める。
 */
import { vrmExpressionSetter, vrmHeadBone, extractVrm } from '@/lib/interview/vrm-adapter'

function fakeVrm(opts: {
  expressions?: string[]
  head?: string | null
  neck?: string | null
} = {}) {
  const { expressions = ['blink', 'aa'], head = 'J_Bip_C_Head', neck = 'J_Bip_C_Neck' } = opts
  const set: Array<[string, number]> = []
  const nodes: Record<string, { name: string } | null> = {
    head: head ? { name: head } : null,
    neck: neck ? { name: neck } : null,
  }
  return {
    vrm: {
      scene: {} as never,
      humanoid: { getNormalizedBoneNode: (n: string) => nodes[n] ?? null },
      expressionManager: {
        getExpression: (n: string) => (expressions.includes(n) ? {} : undefined),
        setValue: (n: string, w: number) => { set.push([n, w]) },
      },
    },
    set,
  }
}

describe('vrm-adapter', () => {
  it('VRM 1.0 の表情名で blink / aa を引く', () => {
    const { vrm, set } = fakeVrm()
    const s = vrmExpressionSetter(vrm as never)!
    expect(s.hasBlink).toBe(true)
    expect(s.hasMouth).toBe(true)
    s.setBlink(1); s.setMouthOpen(0.4)
    expect(set).toEqual([['blink', 1], ['aa', 0.4]])
  })

  it('VRM 0.x の大文字の表情名でも引く', () => {
    const { vrm, set } = fakeVrm({ expressions: ['Blink', 'A'] })
    const s = vrmExpressionSetter(vrm as never)!
    s.setBlink(1); s.setMouthOpen(1)
    expect(set).toEqual([['Blink', 1], ['A', 1]])
  })

  it('口の表情だけのモデルでも成立する', () => {
    const { vrm } = fakeVrm({ expressions: ['aa'] })
    const s = vrmExpressionSetter(vrm as never)!
    expect(s.hasBlink).toBe(false)
    expect(s.hasMouth).toBe(true)
  })

  it('使える表情が無ければ null', () => {
    const { vrm } = fakeVrm({ expressions: [] })
    expect(vrmExpressionSetter(vrm as never)).toBeNull()
  })

  it('頭は規格名で引く（VRoid の内部名に依存しない）', () => {
    const { vrm } = fakeVrm()
    expect(vrmHeadBone(vrm as never)?.name).toBe('J_Bip_C_Head')
  })

  it('頭が無ければ首で代替する', () => {
    const { vrm } = fakeVrm({ head: null })
    expect(vrmHeadBone(vrm as never)?.name).toBe('J_Bip_C_Neck')
  })

  it('humanoid が無ければ null', () => {
    expect(vrmHeadBone({ scene: {} as never } as never)).toBeNull()
  })

  it('VRM でない GLTF からは取り出さない', () => {
    expect(extractVrm({})).toBeNull()
    expect(extractVrm({ userData: {} })).toBeNull()
    expect(extractVrm({ userData: { vrm: { scene: {} } } })).not.toBeNull()
  })
})
