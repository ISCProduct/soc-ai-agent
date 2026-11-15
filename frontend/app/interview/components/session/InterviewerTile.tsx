import type { ReactNode } from 'react'
import { Box, Chip, Typography } from '@mui/material'

import { CALL } from '../../callTokens'
import InterviewerAvatar from '../InterviewerAvatar'

/** タイル下端の、質問表示に覆われる分（高さに対する割合） */
const AVATAR_AREA_BOTTOM_RESERVE = '30%'

interface Props {
  /** 名札。名前のデータが無いので「{企業名} 面接官」 */
  nameTag: string
  avatarGender: 'male' | 'female'
  aiLevel: number
  aiSpeaking: boolean
  isConnected: boolean
  turnPending: boolean
  studentPauseCount: number
  recording: boolean
  /** 深掘り質問 or カテゴリ。無ければ null */
  chipLabel: string | null
  isDeepening: boolean
  /** 自分の小窓・字幕・オーバーレイなど、タイルに重ねるもの */
  children: ReactNode
}

/** 大きな面接官タイル。録音中は縁取りが緑になり、色だけでなく左上の「録音中」でも示す。 */
export default function InterviewerTile(p: Props) {
  return (
    <Box
      sx={{
        position: 'relative', flex: 1, minWidth: 0, minHeight: 0, overflow: 'hidden',
        borderRadius: 2, bgcolor: CALL.panel,
        border: '2px solid', borderColor: p.recording ? CALL.ok : CALL.accent,
      }}
    >
      {/* 下部は字幕風の質問表示に覆われるので、アバターと名札はその分だけ上の領域に収める */}
      <Box sx={{ position: 'absolute', top: 0, left: 0, right: 0, bottom: AVATAR_AREA_BOTTOM_RESERVE }}>
        <InterviewerAvatar
          gender={p.avatarGender}
          level={p.aiLevel}
          speaking={p.aiSpeaking}
          active={p.isConnected}
          pending={p.turnPending}
          acknowledgeSignal={p.studentPauseCount}
        />
        <Typography
          component="span"
          sx={{ position: 'absolute', left: 12, bottom: 12, px: 1.25, py: 0.5, borderRadius: 1, bgcolor: CALL.stage, color: CALL.ink, fontSize: 12, fontWeight: 600 }}
        >
          {p.nameTag}
        </Typography>
      </Box>

      <Box sx={{ position: 'absolute', top: 12, left: 12, zIndex: 1, display: 'flex', gap: 1, alignItems: 'center' }}>
        {p.recording && (
          <Typography component="span" sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.75, px: 1.5, py: 0.5, borderRadius: 1, bgcolor: CALL.stage, color: CALL.ink, fontSize: 12, fontWeight: 600 }}>
            <Box aria-hidden sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: CALL.rec }} />
            録音中
          </Typography>
        )}
        {p.chipLabel && (
          <Chip size="small" label={p.chipLabel} sx={{ bgcolor: p.isDeepening ? 'warning.dark' : CALL.control, color: '#fff' }} />
        )}
      </Box>

      {p.children}
    </Box>
  )
}
