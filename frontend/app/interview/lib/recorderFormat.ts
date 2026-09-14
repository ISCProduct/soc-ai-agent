/**
 * MediaRecorder が実際に出せる音声形式を選ぶ。
 *
 * 以前は 'audio/webm' を固定で渡していた。仕様上、対応していない mimeType を
 * 渡すと MediaRecorder のコンストラクタは NotSupportedError を投げる。
 * Safari（iOS を含む）は WebM を作れないため、Safari の学生は
 * 「話す」を押しても録音が始まらない状態だった。
 *
 * バックエンドは受け取ったバイト列からコンテナを判定する
 * (Backend/internal/services/interview/audio_format.go) ので、
 * ここで形式が変わっても認識側は追随する。
 */

/** 優先順。前にあるものほど音声認識で扱いやすい。 */
const AUDIO_MIME_CANDIDATES = [
  'audio/webm;codecs=opus',
  'audio/webm',
  'audio/ogg;codecs=opus',
  'audio/mp4',
] as const

export type RecorderFormat = {
  /** MediaRecorder に渡す mimeType。空文字ならブラウザ既定に任せる。 */
  mimeType: string
  /** Blob と送信ファイル名に使う拡張子。 */
  ext: string
}

/** ブラウザ既定に任せる場合の値。拡張子は最もありふれた webm を仮置きする。 */
const BROWSER_DEFAULT: RecorderFormat = { mimeType: '', ext: 'webm' }

export function extFromMimeType(mimeType: string): string {
  const base = mimeType.split(';')[0].trim().toLowerCase()
  switch (base) {
    case 'audio/webm':
      return 'webm'
    case 'audio/ogg':
      return 'ogg'
    case 'audio/mp4':
      return 'mp4'
    case 'audio/mpeg':
      return 'mp3'
    case 'audio/wav':
    case 'audio/wave':
      return 'wav'
    default:
      return 'webm'
  }
}

/**
 * 対応している形式のうち最も優先度の高いものを返す。
 *
 * どれも対応していない場合は mimeType を指定せずブラウザ既定に任せる。
 * 指定せずに構築する分にはコンストラクタは投げないため、
 * 「録音が始まらない」よりは形式が読めないほうがまだ回復できる。
 */
export function pickRecorderFormat(
  isSupported: (t: string) => boolean = (t) =>
    typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(t),
): RecorderFormat {
  for (const candidate of AUDIO_MIME_CANDIDATES) {
    try {
      if (isSupported(candidate)) {
        return { mimeType: candidate, ext: extFromMimeType(candidate) }
      }
    } catch {
      // isTypeSupported が無い環境では既定に落とす
      return BROWSER_DEFAULT
    }
  }
  return BROWSER_DEFAULT
}
