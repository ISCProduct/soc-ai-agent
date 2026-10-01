'use client'

import { useCallback, useEffect, useRef, useState } from 'react'

/**
 * 音声入力。ブラウザ内蔵の音声認識（Web Speech API）を使う。
 *
 * サーバーの文字起こし（gpt-4o-mini-transcribe）を使う手もあるが、
 * あちらは $0.006/分 の従量課金で、音声はこの製品で最も大きいコスト要因にあたる。
 * ブラウザ内蔵なら追加コストが無く、サーバー往復も挟まない。
 * 学生が主に使う Chrome / Safari はどちらも対応している。
 *
 * 精度はサーバー側の方が高いので、固有名詞の取りこぼしが問題になったら
 * ここの実装だけ差し替えればよい。返す形（text / listening / error）は変えない。
 *
 * 認識結果は自動送信せず、呼び出し側が入力欄へ入れる。
 * 音声認識は誤りが珍しくないので、送る前に直せる状態にしておく。
 */

type SpeechInput = {
  /** この環境で音声入力が使えるか。false のときはボタンを出さない。 */
  supported: boolean
  listening: boolean
  /** 認識できなかった理由。画面に出す文言をそのまま入れる。 */
  error: string | null
  /**
   * 端末内だけで処理しているか。
   *
   * false のときは音声が Google のサーバーへ送られる。
   * 「端末内で処理します」と画面に書いてよいのは true のときだけ。
   */
  local: boolean
  start: () => void
  stop: () => void
}

type AvailabilityStatus = 'available' | 'downloadable' | 'downloading' | 'unavailable'

/** Chrome 139+ の端末内認識。実装が無いブラウザでは undefined。 */
type RecognitionCtor = (new () => SpeechRecognitionLike) & {
  available?: (o: { langs: string[]; processLocally: boolean }) => Promise<AvailabilityStatus>
  install?: (o: { langs: string[]; processLocally: boolean }) => Promise<boolean>
}

const LOCAL_OPTIONS = { langs: ['ja-JP'], processLocally: true }

/**
 * 端末内処理を試みるか。既定は有効。
 *
 * SpeechRecognition.available() は Chromium のビルドによってタブごと落ちる。
 * Playwright 同梱の Chromium で再現した（実物の Chrome 143 では
 * 'downloadable' を正常に返す）。レンダラが落ちると .catch() では拾えず、
 * 押した学生のチャットがその場で終わる。
 *
 * 実利用のブラウザでは問題ない見込みだが、万一 staging で出たときに
 * デプロイを待たずに止められるよう、環境変数で切れるようにしてある。
 *   NEXT_PUBLIC_SPEECH_ON_DEVICE=0
 */
function onDeviceEnabled(): boolean {
  return process.env.NEXT_PUBLIC_SPEECH_ON_DEVICE !== '0'
}

/** 標準名と webkit 接頭辞の両方を見る。Safari は後者。 */
function getRecognitionCtor(): RecognitionCtor | null {
  if (typeof window === 'undefined') return null
  const w = window as unknown as {
    SpeechRecognition?: RecognitionCtor
    webkitSpeechRecognition?: RecognitionCtor
  }
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null
}

/** 使う範囲だけを写した型。lib.dom の定義はブラウザ差があり当てにしない。 */
type SpeechRecognitionLike = {
  lang: string
  /** Chrome 139+ の端末内処理。古い実装には無いので任意。 */
  options?: { langs: string[]; processLocally: boolean }
  continuous: boolean
  interimResults: boolean
  start: () => void
  stop: () => void
  abort: () => void
  onresult: ((event: SpeechRecognitionEventLike) => void) | null
  onerror: ((event: { error: string }) => void) | null
  onend: (() => void) | null
}

type SpeechRecognitionEventLike = {
  resultIndex: number
  results: ArrayLike<ArrayLike<{ transcript: string }> & { isFinal: boolean }>
}

/**
 * 認識できなかった理由を、利用者が次に何をすればよいか分かる文言にする。
 * ブラウザが返すコードはそのままでは意味が伝わらない。
 */
function describeError(code: string): string {
  switch (code) {
    case 'not-allowed':
    case 'service-not-allowed':
      return 'マイクの使用が許可されていません。ブラウザの設定で許可してから、もう一度お試しください。'
    case 'no-speech':
      return '声が聞き取れませんでした。もう一度お試しください。'
    case 'audio-capture':
      return 'マイクが見つかりませんでした。接続を確認してください。'
    case 'network':
      return '通信エラーで聞き取れませんでした。通信状況を確認してください。'
    default:
      return '音声を聞き取れませんでした。キーボードで入力することもできます。'
  }
}

/**
 * 聞き取りを打ち切るまでの時間。
 *
 * Chrome の音声認識はクラウドへ投げるため、通信が詰まると onend が返らず、
 * ボタンが「聞き取り中」のまま固まってマイクを掴み続ける。
 * 実際、自動実行の環境では 30 秒待っても戻らなかった。
 * 一区切りを話すには十分な長さで切り上げる。
 */
const LISTEN_TIMEOUT_MS = 15_000

export function useSpeechInput(onText: (text: string) => void): SpeechInput {
  const [supported, setSupported] = useState(false)
  const [listening, setListening] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [local, setLocal] = useState(false)
  /** 端末内処理の可否を調べたか。調べるのはマイクを押したときの1回だけ。 */
  const checkedRef = useRef(false)
  const recognitionRef = useRef<SpeechRecognitionLike | null>(null)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // 認識中にコールバックが差し替わっても、古い参照を掴まないようにする。
  const onTextRef = useRef(onText)
  onTextRef.current = onText

  useEffect(() => {
    const Ctor = getRecognitionCtor()
    setSupported(Ctor !== null)

    // ここでは端末内処理の可否を調べない。
    //
    // SpeechRecognition.available() は Playwright 同梱の Chromium で
    // タブごとクラッシュした（実物の Chrome 143 では正常に 'downloadable' を返す）。
    // レンダラが落ちると .catch() では拾えず、チャット画面全体が開けなくなる。
    // 読み込み時に呼ぶと全利用者が巻き添えになるので、
    // マイクを押したとき（本人の操作）に初めて調べる。
    return () => {
      // 画面を離れるときにマイクを握ったままにしない。
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
      recognitionRef.current?.abort()
      recognitionRef.current = null
    }
  }, [])

  const clearTimer = useCallback(() => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current)
      timeoutRef.current = null
    }
  }, [])

  const stop = useCallback(() => {
    clearTimer()
    recognitionRef.current?.stop()
    setListening(false)
  }, [clearTimer])

  const start = useCallback(() => {
    const Ctor = getRecognitionCtor()
    if (!Ctor) return

    // 端末内で処理できるかを、押されたときに1度だけ調べる。
    //
    // 待たない。調べ終わる前に始めても従来どおり（クラウド）で動くし、
    // 言語パックの取得は数十MBになりうるので、そこで黙って止めない。
    // 2回目以降の押下から端末内処理になる。
    if (onDeviceEnabled() && !checkedRef.current && Ctor.available) {
      checkedRef.current = true
      void Ctor.available(LOCAL_OPTIONS)
        .then(async (status) => {
          if (status === 'available') {
            setLocal(true)
            return
          }
          // 言語パックが未取得。押したのは本人の操作なので取りに行く。
          if (status === 'downloadable' && Ctor.install) {
            const ok = await Ctor.install(LOCAL_OPTIONS)
            if (ok) setLocal(true)
          }
        })
        .catch(() => {
          // 判定できない。従来どおりで動かす。
        })
    }
    // 連打で複数の認識が走らないようにする。
    recognitionRef.current?.abort()
    setError(null)

    const recognition = new Ctor()
    recognition.lang = 'ja-JP'
    // 端末内で処理できると分かっているときだけ指定する。
    // 使えないのに true を渡すと language-not-supported で落ちる。
    if (local) recognition.options = LOCAL_OPTIONS
    // 一区切りで止める。話し続ける用途ではないので、長く開けてもマイクを握るだけ。
    recognition.continuous = false
    // 確定前の文字は入力欄へ入れない。書き換わる様子が見えると直しにくい。
    recognition.interimResults = false

    recognition.onresult = (event) => {
      let text = ''
      for (let i = event.resultIndex; i < event.results.length; i++) {
        const result = event.results[i]
        if (result.isFinal) text += result[0]?.transcript ?? ''
      }
      const trimmed = text.trim()
      if (trimmed) onTextRef.current(trimmed)
    }
    recognition.onerror = (event) => {
      clearTimer()
      setError(describeError(event.error))
      setListening(false)
    }
    recognition.onend = () => {
      clearTimer()
      setListening(false)
    }

    recognitionRef.current = recognition
    try {
      recognition.start()
      setListening(true)
      // 応答が返らないまま固まる場合に備えて打ち切る。
      clearTimer()
      timeoutRef.current = setTimeout(() => {
        recognitionRef.current?.abort()
        setListening(false)
        setError('音声を聞き取れませんでした。キーボードで入力することもできます。')
      }, LISTEN_TIMEOUT_MS)
    } catch {
      // すでに開始済みなど。握ったままにしないよう畳む。
      setListening(false)
    }
  }, [clearTimer, local])

  return { supported, listening, error, local, start, stop }
}
