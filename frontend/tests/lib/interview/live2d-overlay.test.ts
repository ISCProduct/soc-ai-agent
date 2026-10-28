import { OVERLAY_MESH_IDS, overlayOpacities } from '@/lib/interview/live2d-overlay'

describe('overlayOpacities', () => {
  it('shows neither overlay when the eyes are open and the mouth is closed', () => {
    expect(overlayOpacities(1, 0)).toEqual({ eyesClosed: 0, mouthOpen: 0 })
  })

  it('follows the mouth value while the eyes are open', () => {
    expect(overlayOpacities(1, 0.6)).toEqual({ eyesClosed: 0, mouthOpen: 0.6 })
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
    expect(OVERLAY_MESH_IDS).toEqual({ eyesClosed: 'ArtMesh0', mouthOpen: 'ArtMesh1' })
  })
})
