/**
 * @jest-environment jsdom
 */
import { act, render } from '@testing-library/react'
import Avatar2D from '@/app/interview/components/Avatar2D'

/**
 * Avatar2D(#2D化)の動作検証。SessionScreenのテストは文言・折りたたみしか
 * 見ておらず、Avatar2Dが何も描画しなくなっても検知できなかった(レビュー指摘)。
 * ここでは実コンポーネントを通して、口パク・うなずき・まばたきタイマーの
 * 解放を確認する。
 */

const EYE = '[cx="79"][cy="98"]'
const SPEAKING_MOUTH = '[cx="100"][cy="130"]'
const IDLE_MOUTH_SMILE = 'path[d="M88 129 Q100 137 112 129"]'

describe('Avatar2D', () => {
  afterEach(() => {
    jest.useRealTimers()
  })

  it('speaking=falseのときは柔らかい笑みの口になる', () => {
    const { container } = render(
      <Avatar2D gender="female" level={0} speaking={false} active={true} />
    )
    expect(container.querySelector(IDLE_MOUTH_SMILE)).not.toBeNull()
    expect(container.querySelector(SPEAKING_MOUTH)).toBeNull()
  })

  it('speaking中はlevelが高いほど口が大きく開く', () => {
    const { container, rerender } = render(
      <Avatar2D gender="female" level={0.1} speaking={true} active={true} />
    )
    const mouthLow = container.querySelector(SPEAKING_MOUTH)
    expect(mouthLow).not.toBeNull()
    const ryLow = Number(mouthLow!.getAttribute('ry'))

    rerender(<Avatar2D gender="female" level={0.9} speaking={true} active={true} />)
    const ryHigh = Number(container.querySelector(SPEAKING_MOUTH)!.getAttribute('ry'))

    expect(ryHigh).toBeGreaterThan(ryLow)
  })

  it('acknowledgeSignalが変わるとうなずき(transform)が一時的にかかり、時間経過後に戻る', () => {
    jest.useFakeTimers()
    const { container, rerender } = render(
      <Avatar2D gender="female" level={0} speaking={false} active={true} acknowledgeSignal={0} />
    )
    const svg = container.querySelector('svg')!
    expect(svg.style.transform).toBe('translateY(0) rotate(0deg)')

    rerender(<Avatar2D gender="female" level={0} speaking={false} active={true} acknowledgeSignal={1} />)
    expect(svg.style.transform).toContain('translateY(3px)')

    act(() => {
      jest.advanceTimersByTime(500) // NOD_DURATION_MS(450ms)より後
    })
    expect(svg.style.transform).toBe('translateY(0) rotate(0deg)')
  })

  it('まばたきで目のry(縦半径)が一時的に縮む', () => {
    jest.useFakeTimers()
    jest.spyOn(Math, 'random').mockReturnValue(0) // randomBetween([2500,6000]) -> 2500ms固定
    const { container } = render(
      <Avatar2D gender="female" level={0} speaking={false} active={true} />
    )
    const eye = () => container.querySelector(EYE)!
    expect(Number(eye().getAttribute('ry'))).toBeGreaterThan(5)

    act(() => {
      jest.advanceTimersByTime(2500) // 最初のまばたきが発火
    })
    expect(Number(eye().getAttribute('ry'))).toBeLessThan(1)

    act(() => {
      jest.advanceTimersByTime(120) // BLINK_DURATION_MS
    })
    expect(Number(eye().getAttribute('ry'))).toBeGreaterThan(5)

    jest.spyOn(Math, 'random').mockRestore()
  })

  it('アンマウント時にまばたき・うなずきのタイマーを解放する(以降の時間経過で状態更新エラーが出ない)', () => {
    jest.useFakeTimers()
    const { unmount } = render(
      <Avatar2D gender="male" level={0} speaking={false} active={true} acknowledgeSignal={0} />
    )
    unmount()
    // アンマウント後にタイマーが残っていると act の外での setState warning / エラーになる。
    // ここで例外が起きないこと自体が「解放できている」ことの確認になる。
    expect(() => {
      jest.advanceTimersByTime(20000)
    }).not.toThrow()
  })
})
