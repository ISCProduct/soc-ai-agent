/**
 * LipsyncManager - Manages real-time lipsync using Web Audio API frequency analysis
 * Analyzes audio frequency bands to determine mouth shape without external dependencies
 */

export interface VisemeWeights {
  [visemeName: string]: number
}

// Simple vowel shapes mapped to common morph target names
// These are used as fallback when model has Oculus-style visemes
const OCULUS_VISEMES = [
  'viseme_sil',
  'viseme_PP',
  'viseme_FF',
  'viseme_TH',
  'viseme_DD',
  'viseme_kk',
  'viseme_CH',
  'viseme_SS',
  'viseme_nn',
  'viseme_RR',
  'viseme_aa',
  'viseme_E',
  'viseme_I',
  'viseme_O',
  'viseme_U',
] as const

type OculusViseme = (typeof OCULUS_VISEMES)[number]

// Frequency band definitions (Hz) for vowel detection
// Based on approximate F1/F2 formant ranges for Japanese vowels
const FREQ_BANDS = {
  sub:  [0,    80],   // sub-bass (jaw movement)
  low:  [80,   300],  // low fundamental
  mid:  [300,  800],  // first formant (F1)
  high: [800,  2500], // second formant (F2)
  air:  [2500, 8000], // sibilants / fricatives
} as const

// RMS amplitude below this threshold is treated as silence (no speech detected)
const SILENCE_RMS_THRESHOLD = 0.015

export class LipsyncManager {
  private audioContext: AudioContext | null = null
  private analyser: AnalyserNode | null = null
  private sourceNode: MediaStreamAudioSourceNode | null = null
  private freqData: Uint8Array<ArrayBuffer> | null = null
  private timeData: Uint8Array<ArrayBuffer> | null = null
  private animationFrameId: number | null = null

  // Current viseme state — smoothed toward target each frame
  private currentViseme: OculusViseme = 'viseme_sil'
  private currentWeight = 0
  private targetViseme: OculusViseme = 'viseme_sil'
  private targetWeight = 0

  // Smoothing constants — tune these to adjust lipsync responsiveness
  private readonly WEIGHT_ATTACK  = 0.25   // how fast weight rises (0–1, higher = faster)
  private readonly WEIGHT_RELEASE = 0.18   // how fast weight falls (0–1, higher = faster)
  private readonly VISEME_HOLD_MS = 60     // minimum ms before switching to a different viseme
  private lastVisemeChangeTime = 0

  constructor(audioStream: MediaStream | null) {
    if (audioStream) {
      this.initializeAudioAnalysis(audioStream)
    }
  }

  private initializeAudioAnalysis(audioStream: MediaStream): void {
    try {
      this.audioContext = new AudioContext()
      this.analyser = this.audioContext.createAnalyser()
      this.analyser.fftSize = 1024
      this.analyser.smoothingTimeConstant = 0.75

      this.freqData = new Uint8Array(this.analyser.frequencyBinCount)
      this.timeData = new Uint8Array(this.analyser.fftSize)

      this.sourceNode = this.audioContext.createMediaStreamSource(audioStream)
      this.sourceNode.connect(this.analyser)

      console.log('[LipsyncManager] Audio analysis initialized')
      this.startAnalysis()
    } catch (error) {
      console.error('[LipsyncManager] Failed to initialize audio analysis:', error)
    }
  }

  private startAnalysis(): void {
    const tick = () => {
      if (!this.analyser || !this.freqData || !this.timeData) return

      // RMS from time-domain data
      this.analyser.getByteTimeDomainData(this.timeData)
      let sum = 0
      for (const v of this.timeData) {
        const n = (v - 128) / 128
        sum += n * n
      }
      const rms = Math.sqrt(sum / this.timeData.length)

      // Frequency band energies
      this.analyser.getByteFrequencyData(this.freqData)
      const sampleRate = this.audioContext!.sampleRate
      const binHz = sampleRate / (this.analyser.fftSize)

      const bandEnergy = (lo: number, hi: number): number => {
        const start = Math.floor(lo / binHz)
        const end   = Math.min(Math.ceil(hi / binHz), this.freqData!.length)
        let s = 0
        for (let i = start; i < end; i++) s += this.freqData![i] / 255
        return end > start ? s / (end - start) : 0
      }

      const eMid  = bandEnergy(...FREQ_BANDS.mid)
      const eHigh = bandEnergy(...FREQ_BANDS.high)
      const eAir  = bandEnergy(...FREQ_BANDS.air)

      // Determine target viseme from frequency ratios (no randomness)
      const now = performance.now()
      const canChange = now - this.lastVisemeChangeTime > this.VISEME_HOLD_MS

      if (rms < SILENCE_RMS_THRESHOLD) {
        this.targetViseme  = 'viseme_sil'
        this.targetWeight  = 0
      } else {
        // Sibilants: high air-band energy
        if (eAir > 0.25 && eAir > eMid * 1.4) {
          if (canChange) { this.targetViseme = 'viseme_SS'; this.lastVisemeChangeTime = now }
          this.targetWeight = Math.min(1, rms * 4)
        }
        // Fricatives / FF: strong high-band without extreme air
        else if (eHigh > eMid * 1.5 && eHigh > 0.2) {
          if (canChange) { this.targetViseme = 'viseme_FF'; this.lastVisemeChangeTime = now }
          this.targetWeight = Math.min(1, rms * 3.5)
        }
        // Open vowel aa/O: strong mid-band (F1 dominant)
        else if (eMid > eHigh * 1.3 && eMid > 0.15) {
          if (canChange) {
            this.targetViseme = eHigh > 0.1 ? 'viseme_aa' : 'viseme_O'
            this.lastVisemeChangeTime = now
          }
          this.targetWeight = Math.min(1, rms * 4)
        }
        // Front/high vowels E/I: high F2
        else if (eHigh > 0.18) {
          if (canChange) {
            this.targetViseme = eHigh > eMid ? 'viseme_I' : 'viseme_E'
            this.lastVisemeChangeTime = now
          }
          this.targetWeight = Math.min(1, rms * 3.5)
        }
        // Round vowel U
        else if (eMid > 0.1) {
          if (canChange) { this.targetViseme = 'viseme_U'; this.lastVisemeChangeTime = now }
          this.targetWeight = Math.min(1, rms * 3)
        }
        // Default: open mouth
        else {
          if (canChange) { this.targetViseme = 'viseme_aa'; this.lastVisemeChangeTime = now }
          this.targetWeight = Math.min(0.6, rms * 4)
        }
      }

      // Smooth weight transitions
      if (this.targetWeight > this.currentWeight) {
        this.currentWeight += (this.targetWeight - this.currentWeight) * this.WEIGHT_ATTACK
      } else {
        this.currentWeight += (this.targetWeight - this.currentWeight) * this.WEIGHT_RELEASE
      }
      if (this.currentWeight < 0.01) {
        this.currentViseme = 'viseme_sil'
        this.currentWeight = 0
      } else {
        this.currentViseme = this.targetViseme
      }

      this.animationFrameId = requestAnimationFrame(tick)
    }
    this.animationFrameId = requestAnimationFrame(tick)
  }

  /**
   * Returns a weight map for all Oculus visemes (0 = closed, 1 = fully open).
   * All visemes are 0 except the currently active one.
   */
  getCurrentVisemes(): VisemeWeights {
    const weights: VisemeWeights = {}
    for (const v of OCULUS_VISEMES) weights[v] = 0
    if (this.currentViseme && this.currentWeight > 0) {
      weights[this.currentViseme] = this.currentWeight
    }
    return weights
  }

  /** Current overall amplitude (0–1), useful for driving simple mouth-open shape keys */
  getAmplitude(): number {
    return this.currentWeight
  }

  /**
   * Replaces the audio stream being analyzed.
   * Disposes the current AudioContext and creates a new one for the given stream.
   */
  updateAudioStream(audioStream: MediaStream | null): void {
    this.dispose()
    if (audioStream) this.initializeAudioAnalysis(audioStream)
  }

  dispose(): void {
    if (this.animationFrameId !== null) {
      cancelAnimationFrame(this.animationFrameId)
      this.animationFrameId = null
    }
    this.sourceNode?.disconnect()
    this.sourceNode = null
    this.audioContext?.close()
    this.audioContext = null
    this.analyser = null
    this.freqData = null
    this.timeData = null
    console.log('[LipsyncManager] Disposed')
  }
}
