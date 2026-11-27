/**
 * ターン応答の ai_fallback / 空音声から、テキスト継続の表示を決める（#1831）。
 * メタの型は multipart JSON 由来で string に限らない。
 */

export type VoiceDegradeReason = 'no_audio' | 'fallback' | null

/** meta.ai_fallback を工程名の配列にする。無ければ空。 */
export function parseAIFallback(meta: Record<string, unknown>): string[] {
  const raw = meta.ai_fallback
  if (!Array.isArray(raw)) return []
  return raw.filter((v): v is string => typeof v === 'string' && v.trim() !== '')
}

/** 音声 Blob が再生に使えない（空）か。 */
export function isEmptyAudio(audio: Blob | null | undefined): boolean {
  return !audio || audio.size === 0
}

/**
 * このターンでテキスト継続バナーを出すべきか。
 * - 音声が空（TTS 失敗）→ no_audio
 * - ローカル縮退あり → fallback（音声があっても知らせる）
 */
export function voiceDegradeReason(
  audio: Blob | null | undefined,
  fallbackStages: string[],
): VoiceDegradeReason {
  if (isEmptyAudio(audio)) return 'no_audio'
  if (fallbackStages.length > 0) return 'fallback'
  return null
}

export function voiceDegradeBannerText(reason: VoiceDegradeReason): string | null {
  switch (reason) {
    case 'no_audio':
      return '面接官の音声を再生できません。字幕で質問を確認し、下の入力欄から文字で回答できます。'
    case 'fallback':
      return '音声の一部を代替経路で処理しています。聞き取りづらい場合は字幕をオンにするか、文字で回答できます。'
    default:
      return null
  }
}
