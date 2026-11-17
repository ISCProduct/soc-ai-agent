'use client'

import { Box } from '@mui/material'
import type { InterviewReport, InterviewSession } from '@/lib/interview'
import type { WeightScore } from '@/components/feedback/ScoreUpdateBanner'
import type { ReportStatus } from '../hooks/useInterviewSession'
import type { ReportProgressView } from '../reportProgress'
import ActionButtons, { type ActionButtonsProps } from './report/ActionButtons'
import CompletionHeader from './report/CompletionHeader'
import FailedView from './report/FailedView'
import ProgressCard from './report/ProgressCard'
import { ReadyCard, ReportBody, type ReportBodyProps } from './report/ReportReadyView'
import UploadRow, { type VideoUploadStatus } from './report/UploadRow'
import WarningBanners from './report/WarningBanners'
import { buildMetaLine, resolveFailureDetail, resolveReportView, resolveRetryLabel } from './report/resolveReportView'
import { columnSx, pageSx, REPORT_BODY_MAX_WIDTH } from './report/reportStyles'

export interface ReportScreenProps extends Pick<ReportBodyProps, 'userId' | 'emailSending' | 'emailSent' | 'emailError' | 'onSendEmail' | 'isGuest' | 'onRegisterClick' | 'scoresBefore' | 'scoresAfter'> {
  /** 「もう一度練習する」。面接選択へ戻る */
  onPracticeAgain: () => void
  errorMessage: string | null
  reportStatus: ReportStatus
  /** 生成の段階・%。バックエンドが返さない間は null（従来の不定表示） */
  reportProgress?: ReportProgressView | null
  /** サーバーが「失敗」と確定したか（タイムアウト・通信失敗とは文言を分ける） */
  reportFailedByServer?: boolean
  report: InterviewReport | null
  session: InterviewSession | null
  /** レポート生成のポーリング開始時刻（ms）。経過時間の基準 */
  reportStartedAt?: number | null
  /** 見出しの「質問N問・M分S秒　企業名／職種」。取れた項目だけ出す */
  questionCount?: number
  durationSeconds?: number
  companyName?: string
  positionTitle?: string
  onRetryReport?: () => void
  /** 再生成API自体が失敗したときのメッセージ。握り潰さず即座に出す(#1476) */
  reportRetryError?: string
  /** 面接終了API(finishSession)が失敗したか(#1015) */
  finishFailed?: boolean
  onRetryFinish?: () => void
  utteranceSaveFailed?: boolean
  videoUploadStatus: VideoUploadStatus
  videoUploadProgress: number
  videoSizeWarning: string | null
}

/**
 * 面接後画面。ここは「状態に応じて部品を選ぶ」だけで、
 * 状態の判定は resolveReportView、見た目は report/ 配下の部品が持つ。
 * データ取得（ポーリング・動画アップロード）は親側。
 */
export default function ReportScreen(props: ReportScreenProps) {
  const { reportStatus, reportProgress, report, reportFailedByServer = false, onRetryReport, onPracticeAgain } = props
  const view = resolveReportView({ reportStatus, reportProgress, report })
  const metaLine = buildMetaLine(props)
  const showsCard = view === 'fallback' || view === 'progress' || view === 'stalled'

  const primary: ActionButtonsProps['primary'] =
    view === 'ready' ? { kind: 'view', enabled: true }
    : showsCard ? { kind: 'view', enabled: false }
    : view === 'failed' && onRetryReport ? { kind: 'retry', label: resolveRetryLabel(reportFailedByServer), onClick: onRetryReport }
    : null

  return (
    <Box sx={pageSx}>
      <Box sx={columnSx(view === 'ready' ? REPORT_BODY_MAX_WIDTH : undefined)}>
        <CompletionHeader metaLine={metaLine} />
        <WarningBanners {...props} />
        {showsCard && (
          <ProgressCard
            view={reportProgress ?? null}
            stalled={view === 'stalled'}
            startedAtMs={props.reportStartedAt ?? null}
            uploadStatus={props.videoUploadStatus}
            uploadProgress={props.videoUploadProgress}
          />
        )}
        {view === 'failed' && <FailedView detail={resolveFailureDetail(reportStatus, reportFailedByServer)} retryError={props.reportRetryError} />}
        {view === 'ready' && report && <ReadyCard report={report} />}
        {/* 進捗カードの外では、完了済みの動画アップロードは出さない（進行中・失敗だけ知らせる） */}
        {!showsCard && props.videoUploadStatus !== 'done' && (
          <UploadRow status={props.videoUploadStatus} progress={props.videoUploadProgress} />
        )}
        <ActionButtons primary={primary} onPracticeAgain={onPracticeAgain} />
        {view === 'ready' && report && <ReportBody {...props} report={report} />}
      </Box>
    </Box>
  )
}
