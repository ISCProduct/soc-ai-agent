/**
 * ターン応答の ai_fallback / 空音声 / STT 失敗から、テキスト継続の表示を決める（#1831）。
 * メタの型は multipart JSON 由来で string に限らない。
 */

export type VoiceDegradeReason = 'no_audio' | 'fallback' | 'stt_failed' | null

/** 音声系の縮退（stt / tts）。llm だけの縮退は音声案内の対象外。 */
const VOICE_FALLBACK_STAGES = new Set(['stt', 'tts'])

/** meta.ai_fallback を工程名の配列にする。無ければ空。 */
export function parseAIFallback(meta: Record<string, unknown>): string[] {
  const raw = meta.ai_fallback
  if (!Array.isArray(raw)) return []
  return raw.filter((v): v is string => typeof v === 'string' && v.trim() !== '')
}

/** multipart の真偽値（boolean / "true"）を読む。 */
export function metaFlagTrue(meta: Record<string, unknown>, key: string): boolean {
  const v = meta[key]
  return v === true || v === 'true'
}

/** 音声 Blob が再生に使えない（空）か。 */
export function isEmptyAudio(audio: Blob | null | undefined): boolean {
  return !audio || audio.size === 0
}

/** stt / tts のローカル縮退があるか（llm のみは含めない）。 */
export function hasVoiceFallbackStages(stages: string[]): boolean {
  return stages.some((s) => VOICE_FALLBACK_STAGES.has(s))
}

/**
 * このターンのバナー理由（直近ターンの結果。null = 音声は問題なし）。
 * - 音声が空（TTS 失敗）→ no_audio
 * - STT 失敗フラグ → stt_failed
 * - 音声系のローカル縮退あり → fallback
 */
export function voiceDegradeReason(
  audio: Blob | null | undefined,
  fallbackStages: string[],
  options?: { sttFailed?: boolean },
): VoiceDegradeReason {
  if (isEmptyAudio(audio)) return 'no_audio'
  if (options?.sttFailed) return 'stt_failed'
  if (hasVoiceFallbackStages(fallbackStages)) return 'fallback'
  return null
}

/** 文字回答欄を出すべきか（一度 true にしたらセッション中 sticky にする想定）。 */
export function shouldEnterTextContinue(reason: VoiceDegradeReason): boolean {
  return reason !== null
}

export function voiceDegradeBannerText(reason: VoiceDegradeReason): string | null {
  switch (reason) {
    case 'no_audio':
      return '面接官の音声を再生できません。字幕で質問を確認し、下の入力欄から文字で回答できます。'
    case 'stt_failed':
      return '音声を聞き取れませんでした。下の入力欄から文字で回答できます。'
    case 'fallback':
      return '音声の一部を代替経路で処理しています。聞き取りづらい場合は字幕をオンにするか、文字で回答できます。'
    default:
      return null
  }
}
