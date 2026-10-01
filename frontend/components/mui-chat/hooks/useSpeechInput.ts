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
  start: () => void
  stop: () => void
}

/** 標準名と webkit 接頭辞の両方を見る。Safari は後者。 */
function getRecognitionCtor(): (new () => SpeechRecognitionLike) | null {
  if (typeof window === 'undefined') return null
  const w = window as unknown as {
    SpeechRecognition?: new () => SpeechRecognitionLike
    webkitSpeechRecognition?: new () => SpeechRecognitionLike
  }
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null
}

/** 使う範囲だけを写した型。lib.dom の定義はブラウザ差があり当てにしない。 */
type SpeechRecognitionLike = {
  lang: string
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
  const recognitionRef = useRef<SpeechRecognitionLike | null>(null)
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  // 認識中にコールバックが差し替わっても、古い参照を掴まないようにする。
  const onTextRef = useRef(onText)
  onTextRef.current = onText

  useEffect(() => {
    setSupported(getRecognitionCtor() !== null)
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
    // 連打で複数の認識が走らないようにする。
    recognitionRef.current?.abort()
    setError(null)

    const recognition = new Ctor()
    recognition.lang = 'ja-JP'
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
  }, [clearTimer])

  return { supported, listening, error, start, stop }
}
