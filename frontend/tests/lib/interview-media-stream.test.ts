import {
  isStreamUsable,
  classifyMediaErrorKind,
  mediaErrorMessage,
} from '@/app/interview/lib/mediaStream'
import { parseMediaError } from '@/lib/interview/utils'

function track(readyState: 'live' | 'ended') {
  return { readyState } as MediaStreamTrack
}
function stream(...states: Array<'live' | 'ended'>) {
  const tracks = states.map(track)
  return { getTracks: () => tracks } as unknown as MediaStream
}

describe('isStreamUsable', () => {
  it('全トラックが生きていれば使える', () => {
    expect(isStreamUsable(stream('live', 'live'))).toBe(true)
  })

  // 以前は「全トラックが ended」を取り直し条件にしていたため、
  // 音声だけ死んだストリームを再利用して MediaRecorder が動かなかった。
  it('1本でも死んでいれば使えない', () => {
    expect(isStreamUsable(stream('live', 'ended'))).toBe(false)
    expect(isStreamUsable(stream('ended', 'live'))).toBe(false)
  })

  it('全部死んでいれば使えない', () => {
    expect(isStreamUsable(stream('ended', 'ended'))).toBe(false)
  })

  it('トラックが無ければ使えない', () => {
    expect(isStreamUsable(stream())).toBe(false)
  })

  it('null は使えない', () => {
    expect(isStreamUsable(null)).toBe(false)
  })
})

describe('classifyMediaErrorKind', () => {
  const dom = (name: string) => new DOMException('msg', name)

  it.each([
    ['NotAllowedError', 'denied'],
    ['PermissionDeniedError', 'denied'],
    ['SecurityError', 'denied'],
    ['NotFoundError', 'notfound'],
    ['DevicesNotFoundError', 'notfound'],
    ['NotReadableError', 'busy'],
    ['TrackStartError', 'busy'],
    ['AbortError', 'busy'],
  ])('%s -> %s', (name, want) => {
    expect(classifyMediaErrorKind(dom(name))).toBe(want)
  })

  it('未知のエラーは unknown', () => {
    expect(classifyMediaErrorKind(dom('WeirdError'))).toBe('unknown')
    expect(classifyMediaErrorKind(new Error('boom'))).toBe('unknown')
    expect(classifyMediaErrorKind(null)).toBe('unknown')
  })

  // ensureStream は種別を Error の message に載せて投げ直す。
  it('message に名前が入った Error も分類できる', () => {
    expect(classifyMediaErrorKind(new Error('NotReadableError'))).toBe('busy')
    expect(classifyMediaErrorKind(new Error('NotAllowedError'))).toBe('denied')
  })
})

describe('デバイス使用中を権限拒否と混同しない', () => {
  // 利用者の報告「一度許可したのに2回目で権限エラーになる」の核心。
  // 使用中と拒否で案内すべき操作がまったく違う。
  it('使用中の文言に「拒否」を含めない', () => {
    const busy = mediaErrorMessage('busy')
    expect(busy).toContain('使用中')
    expect(busy).not.toContain('拒否')
    expect(busy).not.toContain('権限')
  })

  it('拒否の文言は権限の許可を案内する', () => {
    const denied = mediaErrorMessage('denied')
    expect(denied).toContain('拒否')
    expect(denied).toContain('権限')
  })

  it('拒否されたデバイスだけを挙げる', () => {
    expect(mediaErrorMessage('denied', 'マイク')).toContain('マイクへのアクセス')
  })

  // parseMediaError（画面表示の最終段）でも同じ区別ができること。
  it('parseMediaError が使用中を権限拒否にしない', () => {
    const msg = parseMediaError(new Error('NotReadableError'))
    expect(msg).toContain('使用中')
    expect(msg).not.toContain('拒否')
  })

  it('parseMediaError は本物の権限拒否を従来どおり案内する', () => {
    const msg = parseMediaError(new Error('NotAllowedError'))
    expect(msg).toContain('拒否')
  })

  // 生のエラー名をそのまま画面に出さない。
  it('parseMediaError が NotReadableError を素通ししない', () => {
    expect(parseMediaError(new Error('NotReadableError'))).not.toBe('NotReadableError')
  })
})
