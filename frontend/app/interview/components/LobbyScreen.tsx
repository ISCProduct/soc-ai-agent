'use client'

import type { ReactNode, RefObject } from 'react'
import {
  Box,
  Button,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import MicIcon from '@mui/icons-material/Mic'
import MicOffIcon from '@mui/icons-material/MicOff'
import VideocamIcon from '@mui/icons-material/Videocam'
import VideocamOffIcon from '@mui/icons-material/VideocamOff'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import RefreshIcon from '@mui/icons-material/Refresh'
import PsychologyIcon from '@mui/icons-material/Psychology'
import { interviewLimits } from '@/lib/interview'
import { PRIMARY, BG_LIGHT } from '../constants'
import type { InterviewCompany } from '../types'

export interface LobbyScreenProps {
  userName: string
  companyName: string
  interviewCompany: InterviewCompany | null
  /** マッチング結果画面から遷移してきた場合に表示するバナー用フラグ */
  fromMatchingResults: boolean
  lobbyPermissionError: string | null
  onRetryPermissions: () => void
  micEnabled: boolean
  cameraEnabled: boolean
  onToggleMic: () => void
  onToggleCamera: () => void
  lobbyVideoRef: RefObject<HTMLVideoElement | null>
  onBack: () => void
  onJoinWithConsent: () => void
  /** 親コンポーネントが保持する ConsentDialog インスタンス（LOBBY / SESSION 画面で共有） */
  consentDialog: ReactNode
}

/**
 * LOBBY SCREEN
 * カメラ・マイクの確認と面接参加操作を行う画面。
 * ストリームの取得・所有は親コンポーネント（page.tsx）が行い、
 * このコンポーネントは受け取った props をそのまま描画する presentational component。
 */
export default function LobbyScreen({
  userName,
  companyName,
  interviewCompany,
  fromMatchingResults,
  lobbyPermissionError,
  onRetryPermissions,
  micEnabled,
  cameraEnabled,
  onToggleMic,
  onToggleCamera,
  lobbyVideoRef,
  onBack,
  onJoinWithConsent,
  consentDialog,
}: LobbyScreenProps) {
  return (
    <Box sx={{ minHeight: '100vh', bgcolor: BG_LIGHT, display: 'flex', flexDirection: 'column' }}>
      {/* Header */}
      <Box component="header" sx={{ px: 3, py: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box sx={{ width: 40, height: 40, borderRadius: 2, bgcolor: PRIMARY, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <PsychologyIcon sx={{ color: '#fff', fontSize: 24 }} />
          </Box>
          <Box>
            <Typography sx={{ fontWeight: 700, fontSize: 16, lineHeight: 1.2 }}>AI面接練習</Typography>
            <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>セッション: {companyName}</Typography>
          </Box>
        </Box>
        <IconButton size="small" onClick={onBack}>
          <ArrowBackIcon fontSize="small" />
        </IconButton>
      </Box>

      {/* Content */}
      <Box sx={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', p: { xs: 2, md: 6 } }}>
        <Box sx={{ maxWidth: 960, width: '100%', display: 'grid', gridTemplateColumns: { xs: '1fr', lg: '7fr 5fr' }, gap: { xs: 4, lg: 8 }, alignItems: 'center' }}>

          {/* Camera preview */}
          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 3 }}>
            <Box sx={{ position: 'relative', width: '100%', aspectRatio: '16/9', bgcolor: '#202124', borderRadius: 2, overflow: 'hidden', boxShadow: '0 4px 20px rgba(0,0,0,0.25)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              {lobbyPermissionError ? (
                <Box sx={{ textAlign: 'center', p: 3 }}>
                  <Typography sx={{ color: '#f28b82', mb: 1.5, fontSize: 14 }}>{lobbyPermissionError}</Typography>
                  <Box sx={{ mb: 2, textAlign: 'left', bgcolor: 'rgba(0,0,0,0.4)', borderRadius: 1, p: 1.5 }}>
                    <Typography sx={{ color: '#e8eaed', fontSize: 12, mb: 0.5 }}>【ブラウザの許可】</Typography>
                    <Typography sx={{ color: '#9aa0a6', fontSize: 12, lineHeight: 1.8, mb: 1 }}>
                      アドレスバー左端の 🔒 → カメラ・マイクを「許可」
                    </Typography>
                    <Typography sx={{ color: '#e8eaed', fontSize: 12, mb: 0.5 }}>【Mac のシステム設定】</Typography>
                    <Typography sx={{ color: '#9aa0a6', fontSize: 12, lineHeight: 1.8 }}>
                      システム設定 → プライバシーとセキュリティ<br />
                      → カメラ / マイク → 使用中のブラウザをオン
                    </Typography>
                  </Box>
                  <Button size="small" startIcon={<RefreshIcon />} onClick={onRetryPermissions} sx={{ color: '#8ab4f8' }}>
                    再試行
                  </Button>
                </Box>
              ) : (
                <video
                  ref={lobbyVideoRef}
                  muted
                  playsInline
                  style={{ width: '100%', height: '100%', objectFit: 'cover', transform: 'scaleX(-1)', display: cameraEnabled ? 'block' : 'none' }}
                />
              )}
              {!lobbyPermissionError && !cameraEnabled && (
                <Box sx={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <VideocamOffIcon sx={{ color: '#9aa0a6', fontSize: 48 }} />
                </Box>
              )}

              {/* Camera label */}
              <Box sx={{ position: 'absolute', top: 12, left: 12, bgcolor: 'rgba(0,0,0,0.5)', px: 1.5, py: 0.5, borderRadius: 1 }}>
                <Typography sx={{ color: '#fff', fontSize: 13 }}>{userName || 'あなた'}</Typography>
              </Box>

              {/* Controls overlay */}
              <Box sx={{ position: 'absolute', bottom: 16, left: 0, right: 0, display: 'flex', justifyContent: 'center', gap: 2 }}>
                <Tooltip title={micEnabled ? 'マイクをオフ' : 'マイクをオン'}>
                  <IconButton
                    onClick={onToggleMic}
                    sx={{ bgcolor: micEnabled ? 'rgba(255,255,255,0.15)' : '#ea4335', border: '1px solid rgba(255,255,255,0.3)', '&:hover': { bgcolor: micEnabled ? 'rgba(255,255,255,0.25)' : '#c5221f' } }}
                  >
                    {micEnabled ? <MicIcon sx={{ color: '#fff' }} /> : <MicOffIcon sx={{ color: '#fff' }} />}
                  </IconButton>
                </Tooltip>
                <Tooltip title={cameraEnabled ? 'カメラをオフ' : 'カメラをオン'}>
                  <IconButton
                    onClick={onToggleCamera}
                    sx={{ bgcolor: cameraEnabled ? 'rgba(255,255,255,0.15)' : '#ea4335', border: '1px solid rgba(255,255,255,0.3)', '&:hover': { bgcolor: cameraEnabled ? 'rgba(255,255,255,0.25)' : '#c5221f' } }}
                  >
                    {cameraEnabled ? <VideocamIcon sx={{ color: '#fff' }} /> : <VideocamOffIcon sx={{ color: '#fff' }} />}
                  </IconButton>
                </Tooltip>
              </Box>
            </Box>

            <Typography variant="body2" sx={{ color: 'text.secondary', fontSize: 13 }}>
              カメラとマイクの確認が完了したら「面接に参加」を押してください
            </Typography>
          </Box>

          {/* Join panel */}
          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: { xs: 'center', lg: 'flex-start' }, textAlign: { xs: 'center', lg: 'left' } }}>
            {fromMatchingResults && (
              <Box sx={{ mb: 2, px: 2, py: 1, bgcolor: '#e8f5e9', borderRadius: 2, border: '1px solid #a5d6a7', width: '100%', maxWidth: 340 }}>
                <Typography sx={{ fontSize: 13, color: '#2e7d32', fontWeight: 600 }}>
                  {interviewCompany?.name}（{interviewCompany?.industry || '業種未設定'}）向けの面接練習を始めます
                </Typography>
              </Box>
            )}
            <Typography variant="h4" sx={{ fontWeight: 400, color: '#202124', mb: 1 }}>
              準備はできましたか？
            </Typography>
            <Typography sx={{ color: 'text.secondary', mb: 1, fontSize: 15 }}>
              {companyName}
            </Typography>
            {interviewCompany && interviewCompany.id === 0 && interviewCompany.name.trim() && (
              <Box sx={{ mb: 2, px: 2, py: 1.5, bgcolor: '#fff8e1', borderRadius: 2, border: '1px solid #ffe082', width: '100%', maxWidth: 340, textAlign: 'left' }}>
                <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
                  <WarningAmberIcon sx={{ color: '#f9a825', fontSize: 18, mt: 0.2, flexShrink: 0 }} />
                  <Box>
                    <Typography sx={{ fontSize: 13, color: '#f57f17', fontWeight: 600 }}>
                      カスタム質問・深掘りは無効です
                    </Typography>
                    <Typography sx={{ fontSize: 12, color: '#795548', mt: 0.5, lineHeight: 1.5 }}>
                      企業管理に未登録のため、一般的な面接練習のみ行えます。登録企業を選ぶとカスタム質問が有効になります。
                    </Typography>
                  </Box>
                </Box>
              </Box>
            )}
            <Box sx={{ mb: 3, px: 2, py: 1, bgcolor: '#e3f2fd', borderRadius: 1, border: '1px solid #90caf9' }}>
              <Typography sx={{ fontSize: 13, color: '#1565c0' }}>
                ⏱ このセッションは最大{interviewLimits.maxMinutes}分です
              </Typography>
            </Box>

            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ width: '100%', maxWidth: 340 }}>
              <Button
                variant="contained"
                onClick={onJoinWithConsent}
                sx={{
                  flex: 1,
                  bgcolor: '#1a73e8',
                  '&:hover': { bgcolor: '#1557b0' },
                  borderRadius: 9999,
                  py: 1.2,
                  fontWeight: 500,
                  fontSize: 15,
                  textTransform: 'none',
                  boxShadow: '0 1px 3px rgba(0,0,0,0.2)',
                }}
              >
                面接に参加
              </Button>
            </Stack>

            {/* 企業特徴プレビュー */}
            {interviewCompany && (interviewCompany.culture || interviewCompany.work_style || interviewCompany.welfare_details) && (
              <Box sx={{ mt: 3, width: '100%', maxWidth: 340 }}>
                <Typography variant="body2" sx={{ color: 'text.secondary', mb: 1, fontWeight: 600 }}>企業情報</Typography>
                <Stack spacing={1}>
                  {interviewCompany.culture && (
                    <Box sx={{ p: 1.5, bgcolor: '#f0f4ff', borderRadius: 2, border: '1px solid #c7d7f0' }}>
                      <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#3b5bdb', mb: 0.3, textTransform: 'uppercase', letterSpacing: 0.5 }}>企業文化</Typography>
                      <Typography sx={{ fontSize: 13, color: '#1e3a8a', lineHeight: 1.5 }}>{interviewCompany.culture}</Typography>
                    </Box>
                  )}
                  {interviewCompany.work_style && (
                    <Box sx={{ p: 1.5, bgcolor: '#f0fff4', borderRadius: 2, border: '1px solid #b2dfdb' }}>
                      <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#2e7d32', mb: 0.3, textTransform: 'uppercase', letterSpacing: 0.5 }}>働き方</Typography>
                      <Typography sx={{ fontSize: 13, color: '#1b5e20', lineHeight: 1.5 }}>{interviewCompany.work_style}</Typography>
                    </Box>
                  )}
                  {interviewCompany.welfare_details && (
                    <Box sx={{ p: 1.5, bgcolor: '#fff8f0', borderRadius: 2, border: '1px solid #ffcc80' }}>
                      <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#e65100', mb: 0.3, textTransform: 'uppercase', letterSpacing: 0.5 }}>福利厚生</Typography>
                      <Typography sx={{ fontSize: 13, color: '#bf360c', lineHeight: 1.5 }}>{interviewCompany.welfare_details}</Typography>
                    </Box>
                  )}
                </Stack>
              </Box>
            )}
          </Box>
        </Box>
      </Box>

      {/* Footer */}
      <Box component="footer" sx={{ py: 2.5, display: 'flex', justifyContent: 'center', gap: 4 }}>
        {['プライバシー', '利用規約', 'ヘルプ'].map(label => (
          <Typography key={label} variant="body2" sx={{ color: 'text.secondary', cursor: 'pointer', '&:hover': { textDecoration: 'underline' } }}>
            {label}
          </Typography>
        ))}
      </Box>

      {consentDialog}
    </Box>
  )
}
