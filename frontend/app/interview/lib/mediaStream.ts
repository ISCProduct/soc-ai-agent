/**
 * 面接のカメラ/マイクストリームの取得まわり。
 *
 * 2回目の面接で「権限が付与されていない」旨のエラーが出る問題に対応する。
 * 原因は2つあった。
 *
 * 1. getUserMedia が同時に2回走り、片方のストリームが取り残される
 *
 *    handleStart は setStatus('connecting') したうえで ensureStream() を呼ぶ。
 *    status の変化でロビープレビューの effect も起動するため、両者が
 *    それぞれ getUserMedia する。どちらも streamRef へ代入するので、
 *    後から入ったほうが先のストリームを上書きし、上書きされた側は
 *    stop() されないまま残る。
 *
 *    残ったストリームはデバイスを掴み続ける。面接を終えても stopStream() は
 *    streamRef が指すものしか止めないため、回を重ねるほど掴みっぱなしの
 *    ストリームが増え、やがて次の getUserMedia が失敗する。
 *
 * 2. デバイス使用中の失敗を「権限拒否」と誤って表示する
 *
 *    NotAllowedError のときブラウザに再度問い合わせて、どちらが拒否された
 *    のかを調べていた。だがデバイスが使用中ならこの問い合わせも失敗するため、
 *    権限は付与されているのに「アクセスが拒否されました」と出る。
 *    利用者から見ると「一度許可したのに権限エラーになる」という症状になる。
 */

/** getUserMedia 共通制約（ロビープレビュー / ensureStream で共有） */
export const MEDIA_CONSTRAINTS: MediaStreamConstraints = {
  audio: { sampleRate: 48000, channelCount: 1, echoCancellation: true, noiseSuppression: true },
  video: true,
}

/**
 * ストリームが使い物になるか。
 *
 * 以前は「全トラックが ended」を取り直しの条件にしていたため、
 * 音声だけが死んだストリームを再利用してしまい、MediaRecorder が動かなかった。
 * 1本でも死んでいれば取り直す。
 */
export function isStreamUsable(stream: MediaStream | null): boolean {
  if (!stream) return false
  const tracks = stream.getTracks()
  if (tracks.length === 0) return false
  return tracks.every((t) => t.readyState !== 'ended')
}

/** 種別ごとのエラー。UI 文言はこの分類から決める。 */
export type MediaErrorKind = 'denied' | 'notfound' | 'busy' | 'unknown'

const ERROR_KIND_BY_NAME: Record<string, MediaErrorKind> = {
  NotAllowedError: 'denied',
  PermissionDeniedError: 'denied',
  SecurityError: 'denied',
  NotFoundError: 'notfound',
  DevicesNotFoundError: 'notfound',
  // 権限はあるが、他のタブ・アプリ・取り残したストリームがデバイスを掴んでいる。
  // これを 'denied' と混ぜると「許可したのに権限エラー」に見える。
  NotReadableError: 'busy',
  TrackStartError: 'busy',
  AbortError: 'busy',
}

export function classifyMediaErrorKind(err: unknown): MediaErrorKind {
  // instanceof は使わない。DOMException は環境によって Error を継承せず
  // (jsdom など)、別realm 由来の Error でも外れる。name / message を直接読む。
  if (typeof err !== 'object' || err === null) return 'unknown'
  const { name, message } = err as { name?: unknown; message?: unknown }

  if (typeof name === 'string') {
    const byName = ERROR_KIND_BY_NAME[name]
    if (byName) return byName
  }

  // ensureStream は種別を message に載せた Error を投げ直す。
  // 素の Error は name が 'Error' のままなので、message からも拾う。
  if (typeof message === 'string') {
    for (const [errName, kind] of Object.entries(ERROR_KIND_BY_NAME)) {
      if (message.includes(errName)) return kind
    }
  }
  return 'unknown'
}

export function mediaErrorMessage(kind: MediaErrorKind, blockedLabel?: string): string {
  switch (kind) {
    case 'denied':
      return `${blockedLabel || 'マイクとカメラ'}へのアクセスが拒否されました。ブラウザのアドレスバー横から権限を許可してください。`
    case 'notfound':
      return 'マイクまたはカメラが見つかりません。デバイスが正しく接続されているか確認してください。'
    case 'busy':
      return 'カメラまたはマイクを他のアプリ・タブが使用中です。使用中のタブやアプリを閉じてから、もう一度お試しください。'
    default:
      return 'カメラの起動に失敗しました。もう一度お試しください。'
  }
}
