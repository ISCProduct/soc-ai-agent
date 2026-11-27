/**
 * 面接官の声とスピーカーテストを鳴らす出力先（#2634）。
 * 準備画面で選び、面接中の再生（useInterviewAudioPlayback）でも同じ値を使う。'' は既定のスピーカー。
 *
 * 面接官の声は AudioContext 経由で鳴らしているので、出力先は Audio 要素ではなく
 * AudioContext.setSinkId で決まる。これが無いブラウザではスピーカーを選べない。
 */
type SinkAudioContext = AudioContext & { sinkId?: string; setSinkId(id: string): Promise<void> }

let outputId = ''

export function setAudioOutputId(id: string): void {
  outputId = id
}

/** スピーカーを選べるブラウザか。選べないブラウザではスピーカー欄を出さない。 */
export function canSelectAudioOutput(): boolean {
  return typeof AudioContext !== 'undefined' && 'setSinkId' in AudioContext.prototype
}

/** 選んだスピーカーを ctx に当てる。抜かれた等で失敗しても既定のスピーカーで鳴らす。 */
export async function applyAudioOutput(ctx: AudioContext): Promise<void> {
  if (!canSelectAudioOutput()) return
  const sink = ctx as SinkAudioContext
  if ((sink.sinkId ?? '') === outputId) return
  try {
    await sink.setSinkId(outputId)
  } catch {
    /* 既定のスピーカーのまま鳴らす */
  }
}

/** 短いテスト音を、選んだスピーカーから鳴らす。 */
export async function playTestTone(): Promise<void> {
  const ctx = new AudioContext()
  await applyAudioOutput(ctx)
  const osc = ctx.createOscillator()
  const gain = ctx.createGain()
  osc.frequency.value = 660
  gain.gain.value = 0.2
  osc.connect(gain).connect(ctx.destination)
  osc.onended = () => { void ctx.close() }
  osc.start()
  osc.stop(ctx.currentTime + 0.6)
}
