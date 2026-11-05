import { FEMALE_OVERLAY_MESH_IDS, OVERLAY_MESH_IDS, overlayOpacities } from '@/lib/interview/live2d-overlay'
import { verticesExtent } from '@/lib/interview/live2d-extent'

describe('overlayOpacities', () => {
  it('shows neither overlay when the eyes are open and the mouth is closed', () => {
    expect(overlayOpacities(1, 0)).toEqual({ eyesClosed: 0, mouthOpen: 0 })
  })

  it('switches the open-mouth image crisply instead of blending it half-transparent', () => {
    expect(overlayOpacities(1, 0.2).mouthOpen).toBe(0)
    expect(overlayOpacities(1, 0.6).mouthOpen).toBe(1)
    const mid = overlayOpacities(1, 0.375).mouthOpen
    expect(mid).toBeGreaterThan(0)
    expect(mid).toBeLessThan(1)
  })

  it('shows the closed-eye image and hides the open-mouth image mid-blink', () => {
    // 口開き差分は目閉じ差分より手前に描かれるため、まばたき中は口を消して目閉じを見せる
    expect(overlayOpacities(0, 1)).toEqual({ eyesClosed: 1, mouthOpen: 0 })
    const half = overlayOpacities(0.5, 1)
    expect(half.eyesClosed).toBeCloseTo(0.5, 5)
    expect(half.mouthOpen).toBeCloseTo(0.5, 5)
  })

  it('clamps out-of-range and non-finite inputs to 0-1', () => {
    expect(overlayOpacities(2, 5)).toEqual({ eyesClosed: 0, mouthOpen: 1 })
    expect(overlayOpacities(-1, -1)).toEqual({ eyesClosed: 1, mouthOpen: 0 })
    expect(overlayOpacities(Number.NaN, Number.NaN)).toEqual({ eyesClosed: 1, mouthOpen: 0 })
  })

  it('targets the exported male model meshes', () => {
    expect(OVERLAY_MESH_IDS).toEqual({ base: 'ArtMesh', eyesClosed: 'ArtMesh0', mouthOpen: 'ArtMesh1' })
  })

  it('targets the exported female model meshes', () => {
    expect(FEMALE_OVERLAY_MESH_IDS).toEqual({ base: 'ArtMesh1', eyesClosed: 'ArtMesh', mouthOpen: 'ArtMesh0' })
  })
})

describe('verticesExtent', () => {
  it('全drawableの頂点を囲む矩形の幅・高さを返す', () => {
    // 2枚目が1枚目の外に出ている。片方だけ見ると小さく測ってしまう。
    const extent = verticesExtent([
      new Float32Array([-1, -2, 0, 0, 1, 2]),
      new Float32Array([-3, -1, 2, 5]),
    ])
    expect(extent).toEqual({ width: 5, height: 7 })
  })

  it('頂点が無ければ 0 を返す（呼び出し側が描画をやめる条件）', () => {
    expect(verticesExtent([])).toEqual({ width: 0, height: 0 })
    expect(verticesExtent([new Float32Array([])])).toEqual({ width: 0, height: 0 })
  })

  it('末尾に y が欠けた奇数長でも落ちない', () => {
    expect(verticesExtent([new Float32Array([0, 0, 4, 3, 9])])).toEqual({ width: 4, height: 3 })
  })
})
