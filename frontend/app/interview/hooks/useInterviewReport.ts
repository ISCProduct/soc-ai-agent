'use client'

import { useRef, useState } from 'react'
import type { User } from '@/lib/auth'
import { interviewApi, InterviewReport, InterviewSession } from '@/lib/interview'
import { WeightScore } from '@/components/feedback/ScoreUpdateBanner'
import {
  evaluateReportPollTick,
  REPORT_POLL_INTERVAL_MS,
  REPORT_POLL_TIMEOUT_MS,
} from '../reportPolling'
import {
  advanceProgress,
  INITIAL_PROGRESS_TRACKER,
  parseReportProgress,
  type ProgressTracker,
  type ReportProgressView,
} from '../reportProgress'

export type ReportStatus = 'idle' | 'pending' | 'ready' | 'error' | 'timeout'

/** レポート生成の再投入API自体が失敗したときの文言(#1476) */
export const REPORT_RETRY_FAILED_MESSAGE =
  'レポート生成の再試行リクエストに失敗しました。通信状況を確認して、もう一度お試しください。'

/** 面接前後のスコア比較に使うセッションID。チャットのセッションがあればそれを使う */
export const resolveScoreSessionId = (userId: number) => {
  if (typeof window !== 'undefined') {
    const chatSession = window.localStorage.getItem('chat_session_id')
    if (chatSession && chatSession.trim() !== '') {
      return chatSession
    }
  }
  return `interview-${userId}`
}

type UseInterviewReportArgs = {
  session: InterviewSession | null
  user: User | null
}

/**
 * 面接後のレポート（生成待ちのポーリング・再試行・面接後スコア・メール送信）。
 * ポーリングの停止（pollRef の clear）は cleanupConnection も行う。
 */
export function useInterviewReport({ session, user }: UseInterviewReportArgs) {
  const [report, setReport] = useState<InterviewReport | null>(null)
  const [reportStatus, setReportStatus] = useState<ReportStatus>('idle')
  /** レポート生成の進捗表示（バックエンドが返さない間は null で従来表示）。SOCAIAGENT-652 */
  const [reportProgress, setReportProgress] = useState<ReportProgressView | null>(null)
  const [reportFailedByServer, setReportFailedByServer] = useState(false)
  /** 経過時間表示の基準（ポーリング開始時刻 ms） */
  const [reportStartedAt, setReportStartedAt] = useState<number | null>(null)
  const progressTrackerRef = useRef<ProgressTracker>(INITIAL_PROGRESS_TRACKER)
  /** レポート生成の再投入API(regenerateReport)が失敗したときのメッセージ(#1476) */
  const [reportRetryError, setReportRetryError] = useState('')
  const [emailSending, setEmailSending] = useState(false)
  const [emailSent, setEmailSent] = useState(false)
  // メール送信失敗をUIへ伝えるためのメッセージ（#1056）
  const [emailError, setEmailError] = useState('')
  const [scoresAfter, setScoresAfter] = useState<WeightScore[] | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pollStartedAtRef = useRef<number | null>(null)
  const pollSessionRef = useRef<{ sessionId: number; userId: number } | null>(null)
  /** 再試行時に古い tick の結果を破棄するための世代カウンタ */
  const pollGenerationRef = useRef(0)
  // レポートポーリングの連続fetch失敗数。成功したら0に戻す（#1057）
  const pollFailureCountRef = useRef(0)

  const stopReportPolling = () => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }

  const loadScoresAfter = async (userId: number) => {
    const scoreSessionId = resolveScoreSessionId(userId)
    try {
      const res = await fetch(`/api/user/weight-scores?session_id=${encodeURIComponent(scoreSessionId)}`)
      const data = await res.json()
      setScoresAfter(data.weight_scores ?? null)
    } catch { /* ignore */ }
  }

  const startReportPolling = (sessionId: number, userId: number) => {
    stopReportPolling()
    const generation = ++pollGenerationRef.current
    pollSessionRef.current = { sessionId, userId }
    pollStartedAtRef.current = Date.now()
    setReportStartedAt(pollStartedAtRef.current)
    pollFailureCountRef.current = 0
    progressTrackerRef.current = INITIAL_PROGRESS_TRACKER
    setReportProgress(null)
    setReportFailedByServer(false)
    setReportStatus('pending')

    const tick = async () => {
      if (generation !== pollGenerationRef.current) return

      const startedAt = pollStartedAtRef.current ?? Date.now()
      let hasReport = false
      let reportPayload: InterviewReport | null = null
      let serverFailed = false

      try {
        const detail = await interviewApi.getDetail(sessionId, userId)
        // 通信が復帰したら連続失敗数をリセットする（#1057）
        pollFailureCountRef.current = 0
        if (detail.report) {
          hasReport = true
          reportPayload = detail.report
        }
        const progress = parseReportProgress(detail.session)
        if (progress && !detail.report) {
          if (progress.stage === 'failed') {
            serverFailed = true
          } else {
            const next = advanceProgress(progressTrackerRef.current, progress, Date.now())
            progressTrackerRef.current = next.tracker
            if (generation === pollGenerationRef.current) setReportProgress(next.view)
          }
        }
      } catch {
        pollFailureCountRef.current += 1
      }

      if (generation !== pollGenerationRef.current) return

      const outcome = evaluateReportPollTick({
        startedAtMs: startedAt,
        nowMs: Date.now(),
        timeoutMs: REPORT_POLL_TIMEOUT_MS,
        hasReport,
        consecutiveFailures: pollFailureCountRef.current,
      })

      if (outcome === 'ready' && reportPayload) {
        setReport(reportPayload)
        setReportStatus('ready')
        stopReportPolling()
        await loadScoresAfter(userId)
        return
      }
      // サーバーが失敗と確定した場合は、タイムアウトを待たずに失敗表示へ（最終リトライ後にだけ書かれる）
      if (serverFailed && outcome !== 'ready') {
        setReportFailedByServer(true)
        setReportStatus('error')
        stopReportPolling()
        return
      }
      if (outcome === 'error') {
        setReportStatus('error')
        stopReportPolling()
        return
      }
      if (outcome === 'timeout') {
        setReportStatus('timeout')
        stopReportPolling()
      }
    }

    // 初回は即時、以降は interval
    void tick()
    pollRef.current = setInterval(() => { void tick() }, REPORT_POLL_INTERVAL_MS)
  }

  /**
   * レポート取得の再試行(#1015, #1476)。
   *
   * ポーリングを再開するだけでは、生成ジョブ自体が失われている場合に永久に復旧しない
   * （asynqの再試行を使い切る／フォールバックworkerで1回失敗する／キューが溢れる、
   * かつ finishSession は終了済みセッションを再キューしない）。
   * そこで再開の前に生成ジョブの再投入を依頼する。既にレポートがあればサーバー側で何もしない。
   */
  const retryReportPolling = async () => {
    const target = pollSessionRef.current
      ?? (session && user ? { sessionId: session.id, userId: user.user_id } : null)
    if (!target) return
    setReportRetryError('')
    try {
      await interviewApi.regenerateReport(target.sessionId, target.userId)
    } catch (e) {
      // 再投入が失敗した＝ジョブは入り直っていない。ここでポーリングへ戻すと、
      // ユーザーは失敗を知らされないまま「生成中」をさらに3分見せられて再びタイムアウトする(#1476)。
      console.error('[report regenerate error]', e)
      setReportRetryError(REPORT_RETRY_FAILED_MESSAGE)
      return
    }
    startReportPolling(target.sessionId, target.userId)
  }

  const sendReportEmail = async () => {
    if (!session || !user) return
    setEmailSending(true)
    setEmailError('')
    try {
      await interviewApi.sendReportEmail(session.id, user.user_id)
      setEmailSent(true)
    } catch {
      // 握り潰すとユーザーが送信成功と誤解するため、必ずUIへ伝える（#1056）
      setEmailError('メールの送信に失敗しました。時間をおいて再度お試しください。')
    } finally {
      setEmailSending(false)
    }
  }

  return {
    report,
    setReport,
    reportStatus,
    setReportStatus,
    reportProgress,
    setReportProgress,
    reportFailedByServer,
    setReportFailedByServer,
    reportStartedAt,
    reportRetryError,
    setReportRetryError,
    emailSending,
    emailSent,
    emailError,
    scoresAfter,
    pollRef,
    startReportPolling,
    retryReportPolling,
    sendReportEmail,
  }
}
