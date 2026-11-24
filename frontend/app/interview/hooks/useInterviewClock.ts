'use client'

import { useRef, useState } from 'react'
import { interviewLimits } from '@/lib/interview'
import type { Position } from '../types'

type UseInterviewClockArgs = {
  selectedPosition: Position
  /**
   * 時間切れで呼ぶ終了処理。
   * タイマーを始めたレンダーの関数がそのまま呼ばれる（分割前と同じ閉包）。
   */
  handleStop: (forced?: boolean) => Promise<void>
}

/**
 * 面接の残り時間・経過時間と、質問の進行（番号・総数・深掘り・カテゴリ）。
 * タイマーの停止（timerRef の clear）は cleanupConnection が行う。
 */
export function useInterviewClock({ selectedPosition, handleStop }: UseInterviewClockArgs) {
  const [remainingSeconds, setRemainingSeconds] = useState(interviewLimits.maxMinutes * 60)
  const [elapsedSeconds, setElapsedSeconds] = useState(0)
  const [currentQuestionIndex, setCurrentQuestionIndex] = useState(1)
  const [totalQuestionCount, setTotalQuestionCount] = useState(Math.max(1, selectedPosition.questions))
  const [questionElapsedSeconds, setQuestionElapsedSeconds] = useState(0)
  const [isDeepeningQuestion, setIsDeepeningQuestion] = useState(false)
  const [questionCategory, setQuestionCategory] = useState<string | null>(null)
  const [sessionWarningShown, setSessionWarningShown] = useState(false)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const sessionStartRef = useRef<number | null>(null)
  const questionStartedAtRef = useRef<number | null>(null)

  const startTimer = () => {
    sessionStartRef.current = Date.now()
    questionStartedAtRef.current = Date.now()
    timerRef.current = setInterval(() => {
      if (!sessionStartRef.current) return
      const elapsed = Math.floor((Date.now() - sessionStartRef.current) / 1000)
      const remaining = Math.max(0, interviewLimits.maxMinutes * 60 - elapsed)
      const safeQuestionDuration = Math.max(60, interviewLimits.questionDurationSeconds || 180)
      const questionElapsed = Math.min(
        safeQuestionDuration,
        Math.max(0, Math.floor((Date.now() - (questionStartedAtRef.current ?? Date.now())) / 1000)),
      )

      setElapsedSeconds(elapsed)
      setRemainingSeconds(remaining)
      setQuestionElapsedSeconds(questionElapsed)
      if (remaining <= 120 && !sessionWarningShown) setSessionWarningShown(true)
      if (remaining <= 0) handleStop(true)
    }, 1000)
  }

  return {
    remainingSeconds,
    setRemainingSeconds,
    elapsedSeconds,
    setElapsedSeconds,
    currentQuestionIndex,
    setCurrentQuestionIndex,
    totalQuestionCount,
    setTotalQuestionCount,
    questionElapsedSeconds,
    setQuestionElapsedSeconds,
    isDeepeningQuestion,
    setIsDeepeningQuestion,
    questionCategory,
    setQuestionCategory,
    sessionWarningShown,
    setSessionWarningShown,
    timerRef,
    questionStartedAtRef,
    startTimer,
  }
}
