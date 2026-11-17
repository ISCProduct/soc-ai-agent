import { Box, Divider, LinearProgress, Stack, Typography } from '@mui/material'
import type { ReportProgressView } from '../../reportProgress'
import { UI } from '@/lib/design/tokens'
import StageList from './StageList'
import UploadRow, { type VideoUploadStatus } from './UploadRow'
import { useElapsedSeconds } from './useElapsedSeconds'
import { formatClock } from './resolveReportView'
import { cardSx, progressBarSx } from './reportStyles'

/** 時間がかかっているときの案内（Figma） */
const STALLED_NOTE = 'いつもより時間がかかっています。もう少し待つか、もう一度試せます。'
const WAIT_NOTE = '待たなくても大丈夫です。できあがると面接履歴に入ります。'
const PERCENT_NOTE = '分析中の%は、進み具合の目安です'
const PERCENT_NOTE_STAGE = 'generating'

export type ProgressCardProps = {
  /** null は旧バックエンド。段階リストを出さず不定の進捗バーにする */
  view: ReportProgressView | null
  stalled: boolean
  /** ポーリング開始時刻（ms） */
  startedAtMs: number | null
  uploadStatus: VideoUploadStatus
  uploadProgress: number
}

export default function ProgressCard({ view, stalled, startedAtMs, uploadStatus, uploadProgress }: ProgressCardProps) {
  const elapsed = useElapsedSeconds(startedAtMs, true)
  const title = stalled ? '時間がかかっています' : 'レポートを作っています'
  return (
    <Box sx={cardSx}>
      <Stack spacing={2}>
        <Box sx={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between' }}>
          <Typography component="h2" sx={{ fontWeight: 700 }}>{title}</Typography>
          {view && <Typography sx={{ color: UI.mark, fontWeight: 700, fontSize: 28 }}>{view.percent}%</Typography>}
        </Box>
        {view ? (
          <LinearProgress variant="determinate" value={view.percent} aria-label="レポート作成の進み具合" sx={progressBarSx} />
        ) : (
          <LinearProgress aria-label="レポートを作成中" sx={progressBarSx} />
        )}
        {view && <StageList stage={view.stage} />}
        {uploadStatus !== 'idle' && (
          <>
            <Divider sx={{ borderColor: UI.rule }} />
            <UploadRow status={uploadStatus} progress={uploadProgress} />
          </>
        )}
        <Box sx={{ display: 'flex', flexDirection: { xs: 'column', sm: 'row' }, justifyContent: 'space-between', gap: 0.5 }}>
          {startedAtMs && <Typography variant="caption" sx={{ color: UI.inkSoft }}>経過 {formatClock(elapsed)}</Typography>}
          {view?.stage === PERCENT_NOTE_STAGE && <Typography variant="caption" sx={{ color: UI.inkSoft }}>{PERCENT_NOTE}</Typography>}
        </Box>
        <Typography role={stalled ? 'status' : undefined} variant="body2" sx={{ color: UI.inkSoft }}>
          {stalled ? STALLED_NOTE : WAIT_NOTE}
        </Typography>
      </Stack>
    </Box>
  )
}
