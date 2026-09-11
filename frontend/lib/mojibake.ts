// OAuthコールバックの氏名文字化け(mojibake)修復ヒューリスティック(#1068)。
//
// UTF-8バイト列がLatin-1として誤解釈された典型的な文字化けパターンでは、
// "Ã"/"Â" の直後にLatin-1の0x80-0xBF範囲の文字(UTF-8継続バイトの見た目)が
// 続くことが多い。単純な文字含有チェックだと "Åke"/"François" のような
// 正当な氏名まで誤って変換してしまうため、往復変換(escape→decodeURIComponent)を
// 実際に試し、変換前後でこのパターンの出現数が減った場合にのみ採用する。
// (制御文字域を正規表現リテラルへ直書きすると可読性が落ちるため String.fromCharCode で組み立てる)
const CONTINUATION_BYTE_RANGE = `${String.fromCharCode(0x80)}-${String.fromCharCode(0xbf)}`
const MOJIBAKE_LIKE_PATTERN = new RegExp(`[ÃÂ][${CONTINUATION_BYTE_RANGE}]`, 'g')
const ASCII_ONLY_PATTERN = new RegExp(`^[${String.fromCharCode(0x00)}-${String.fromCharCode(0x7f)}]*$`)

function countMojibakeLikeChars(s: string): number {
  const matches = s.match(MOJIBAKE_LIKE_PATTERN)
  return matches ? matches.length : 0
}

export function fixMojibake(s: string): string {
  if (ASCII_ONLY_PATTERN.test(s)) return s
  try {
    const repaired = decodeURIComponent(escape(s))
    const suspiciousBefore = countMojibakeLikeChars(s)
    const suspiciousAfter = countMojibakeLikeChars(repaired)
    return suspiciousAfter < suspiciousBefore ? repaired : s
  } catch {
    // decodeURIComponentが不正なシーケンスで例外を投げた場合は元の文字列を維持する
    return s
  }
}
