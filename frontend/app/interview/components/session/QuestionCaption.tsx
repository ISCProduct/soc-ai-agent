import type { RefObject } from 'react'
import { Box, Typography } from '@mui/material'

import { CALL } from '../../callTokens'
import PhaseIconView from './PhaseIconView'
import { PHASE_VIEW, type TurnPhase } from './turnPhase'
import type { Urgency } from './sessionDerive'

/** 質問タイマーの緊急度の色。色だけに頼らず、残り時間の文言も常に出す。 */
const URGENCY_COLOR: Record<Urgency, string> = {
  safe: CALL.ok,
  warning: 'warning.main',
  critical: CALL.rec,
}

interface Props {
  phase: TurnPhase
  /** 字幕オフでも状況行とタイマーは出し、質問文だけ隠す */
  captionsVisible: boolean
  questionText: string
  questionProgress: number
  questionRemainingLabel: string
  urgency: Urgency
  /**
   * useInterviewSession が utterances/partialAi の変化ごとにこの要素へ自動スクロールする。
   * 「現在の質問」を表示するこの要素に付けておけば、新しい質問が届くたびに
   * 現在の質問が見える位置へスクロールされる（履歴の末尾に置くと質問が画面外へ流れる。Codexレビュー指摘）。
   */
  anchorRef: RefObject<HTMLDivElement | null>
}

/** タイル下部の字幕風の質問表示。1行目=いまの状況、録音中は回答時間バー、その下に質問文。 */
export default function QuestionCaption(p: Props) {
  const view = PHASE_VIEW[p.phase]
  return (
    <Box
      ref={p.anchorRef}
      sx={{
        position: 'absolute', left: 12, right: 12, bottom: 12, zIndex: 1,
        px: 2.5, py: 1.75, borderRadius: 1.5, bgcolor: CALL.stage,
        display: 'flex', flexDirection: 'column', gap: 0.75,
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: CALL.soft, fontSize: 13 }}>
        {p.phase === 'recording'
          ? <Box aria-hidden sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: CALL.rec }} />
          : <PhaseIconView icon={p.phase === 'yourTurn' ? 'mic' : view.talkIcon} size={16} />}
        <Typography component="span" role="status" sx={{ fontSize: 'inherit', fontWeight: 600, flex: 1 }}>
          {view.statusText}
        </Typography>
        <Typography component="span" sx={{ fontSize: 12, color: URGENCY_COLOR[p.urgency] }}>
          {p.questionRemainingLabel}
        </Typography>
      </Box>
      {p.phase === 'recording' && (
        <Box
          role="progressbar"
          aria-label="回答時間"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={p.questionProgress}
          sx={{ height: 6, borderRadius: 3, bgcolor: CALL.line, overflow: 'hidden' }}
        >
          <Box sx={{ height: '100%', width: `${p.questionProgress}%`, bgcolor: URGENCY_COLOR[p.urgency] }} />
        </Box>
      )}
      {p.captionsVisible && (
        <Typography sx={{ fontSize: { xs: 16, md: 22 }, fontWeight: 700, lineHeight: 1.5, color: CALL.ink }}>
          {p.questionText || '質問を準備しています…'}
        </Typography>
      )}
    </Box>
  )
}
