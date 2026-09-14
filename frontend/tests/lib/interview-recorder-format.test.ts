import { pickRecorderFormat, extFromMimeType } from '@/app/interview/lib/recorderFormat'

describe('pickRecorderFormat', () => {
  // Chrome / Edge / Firefox は WebM+Opus を出せる
  it('WebM+Opus に対応していればそれを選ぶ', () => {
    const f = pickRecorderFormat(() => true)
    expect(f.mimeType).toBe('audio/webm;codecs=opus')
    expect(f.ext).toBe('webm')
  })

  // Safari (iOS を含む) は WebM を作れない。
  // 以前は 'audio/webm' 固定で MediaRecorder を作っており、
  // NotSupportedError で録音そのものが始まらなかった。
  it('Safari では MP4 に落ちる', () => {
    const f = pickRecorderFormat((t) => t === 'audio/mp4')
    expect(f.mimeType).toBe('audio/mp4')
    expect(f.ext).toBe('mp4')
  })

  it('Ogg しか無ければ Ogg を選ぶ', () => {
    const f = pickRecorderFormat((t) => t.startsWith('audio/ogg'))
    expect(f.mimeType).toBe('audio/ogg;codecs=opus')
    expect(f.ext).toBe('ogg')
  })

  // どれも非対応なら mimeType を指定せずブラウザ既定に任せる。
  // 指定しなければコンストラクタは投げないので、録音は始まる。
  it('どれも非対応ならブラウザ既定に任せる', () => {
    const f = pickRecorderFormat(() => false)
    expect(f.mimeType).toBe('')
    expect(f.ext).toBe('webm')
  })

  it('isTypeSupported が例外を投げても落ちない', () => {
    const f = pickRecorderFormat(() => {
      throw new Error('not implemented')
    })
    expect(f.mimeType).toBe('')
  })

  // 候補は優先順に評価される。WebM が使えるのに MP4 を選ばないこと。
  it('対応が複数あれば優先度の高いものを選ぶ', () => {
    const f = pickRecorderFormat((t) => t === 'audio/mp4' || t === 'audio/webm')
    expect(f.mimeType).toBe('audio/webm')
  })
})

describe('extFromMimeType', () => {
  it.each([
    ['audio/webm;codecs=opus', 'webm'],
    ['audio/webm', 'webm'],
    ['audio/mp4', 'mp4'],
    ['audio/ogg;codecs=opus', 'ogg'],
    ['audio/mpeg', 'mp3'],
    ['audio/wav', 'wav'],
    ['AUDIO/MP4', 'mp4'],
    ['  audio/mp4 ; codecs=mp4a ', 'mp4'],
  ])('%s -> %s', (mime, want) => {
    expect(extFromMimeType(mime)).toBe(want)
  })

  // 判定できないものは webm 扱い。バックエンドがバイト列から判定し直す。
  it('未知の型は webm', () => {
    expect(extFromMimeType('application/octet-stream')).toBe('webm')
    expect(extFromMimeType('')).toBe('webm')
  })
})
