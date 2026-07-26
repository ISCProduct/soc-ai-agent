/**
 * @jest-environment jsdom
 */
import { shouldStartInterviewMediaPreview } from '@/app/interview/hooks/mediaPreviewGate'

describe('shouldStartInterviewMediaPreview', () => {
  it('auth loading 中は開始しない', () => {
    expect(shouldStartInterviewMediaPreview({ loading: true, status: 'lobby' })).toBe(false)
  })

  it('selection / finished では開始しない', () => {
    expect(shouldStartInterviewMediaPreview({ loading: false, status: 'selection' })).toBe(false)
    expect(shouldStartInterviewMediaPreview({ loading: false, status: 'finished' })).toBe(false)
  })

  it('lobby / connecting / connected では開始する', () => {
    expect(shouldStartInterviewMediaPreview({ loading: false, status: 'lobby' })).toBe(true)
    expect(shouldStartInterviewMediaPreview({ loading: false, status: 'connecting' })).toBe(true)
    expect(shouldStartInterviewMediaPreview({ loading: false, status: 'connected' })).toBe(true)
  })
})
