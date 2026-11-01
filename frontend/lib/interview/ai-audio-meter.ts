/**
 * 再生中のAI音声の音量を、Reactの状態(再描画)を経由せずに読むための受け渡し口。
 * 口パクを毎フレームの描画ループで直接読めるようにして、1〜2フレームの遅れと
 * ページ全体の再描画を避ける。再生中だけ useInterviewSession がセットする。
 */
export interface AiAudioMeter {
  /** 現在の音量 0-1 */
  read(): number
  /** 音量を測った時点から実際に聞こえるまでの遅れ（秒）。AudioContext の出力遅延。 */
  latencySec: number
}

let current: AiAudioMeter | null = null

export function setAiAudioMeter(meter: AiAudioMeter | null): void {
  current = meter
}

export function getAiAudioMeter(): AiAudioMeter | null {
  return current
}

/** AnalyserNode の時間領域データから音量(RMS)を 0-1 で返す。Avatar2D 用の level と同じ計算。 */
export function measureLevel(analyser: AnalyserNode, buf: Uint8Array<ArrayBuffer>): number {
  analyser.getByteTimeDomainData(buf)
  let sum = 0
  for (const v of buf) {
    const n = (v - 128) / 128
    sum += n * n
  }
  return Math.min(1, Math.sqrt(sum / buf.length) * 6)
}
