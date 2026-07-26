'use client'

import type { ReactNode, RefCallback, RefObject } from 'react'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  LinearProgress,
  Tooltip,
  Typography,
} from '@mui/material'
import MicIcon from '@mui/icons-material/Mic'
import VolumeUpIcon from '@mui/icons-material/VolumeUp'
import VideocamIcon from '@mui/icons-material/Videocam'
import VideocamOffIcon from '@mui/icons-material/VideocamOff'
import RefreshIcon from '@mui/icons-material/Refresh'
import ClosedCaptionIcon from '@mui/icons-material/ClosedCaption'
import ThreeAvatar from './ThreeAvatar'
import { PRIMARY } from '../constants'
import { formatSeconds } from '@/lib/interview-utils'
import type { Utterance } from '../types'

const waveHeights = [5, 8, 13, 18, 25, 34, 42, 48, 44, 36, 28, 20, 14, 9, 6, 7, 11, 18, 27, 36, 44, 48, 42, 34, 26, 18, 12, 8, 5, 7, 10, 14]

export interface SessionScreenProps {
  status: string
  companyName: string
  userName: string
  isActive: boolean
  isConnected: boolean
  remainingSeconds: number
  sessionWarningShown: boolean
  currentQuestionIndex: number
  totalQuestionCount: number
  questionProgress: number
  questionRemainingSeconds: number
  questionRemainingLabel: string
  isDeepeningQuestion: boolean
  questionCategory: string | null
  avatarGender: 'male' | 'female'
  aiLevel: number
  aiSpeaking: boolean
  cameraEnabled: boolean
  captionsVisible: boolean
  handsFreeMode: boolean
  utterances: Utterance[]
  partialAi: string
  partialUser: string
  isRecording: boolean
  turnPending: boolean
  errorMessage: string | null
  /** video 要素がマウントした瞬間にストリームをアタッチするための callback ref（page.tsx 側で理由を解説） */
  sessionVideoCallbackRef: RefCallback<HTMLVideoElement>
  transcriptEndRef: RefObject<HTMLDivElement | null>
  aiAudioRef: RefObject<HTMLAudioElement | null>
  /** 親コンポーネントが保持する ConsentDialog インスタンス（LOBBY / SESSION 画面で共有） */
  consentDialog: ReactNode
  onToggleCamera: () => void
  onToggleCaptions: () => void
  onToggleHandsFree: () => void
  onStartRecording: () => void
  onStopRecording: () => void
  onJoin: () => void
  onStop: (forced: boolean) => void
}

/**
 * SESSION SCREEN (connecting / connected / error)
 * VAD・MediaRecorder・AudioContext・タイマー等の状態管理は親コンポーネント（page.tsx）が行い、
 * このコンポーネントは受け取った props をそのまま描画する presentational component。
 */
export default function SessionScreen({
  status,
  companyName,
  userName,
  isActive,
  isConnected,
  remainingSeconds,
  sessionWarningShown,
  currentQuestionIndex,
  totalQuestionCount,
  questionProgress,
  questionRemainingSeconds,
  questionRemainingLabel,
  isDeepeningQuestion,
  questionCategory,
  avatarGender,
  aiLevel,
  aiSpeaking,
  cameraEnabled,
  captionsVisible,
  handsFreeMode,
  utterances,
  partialAi,
  partialUser,
  isRecording,
  turnPending,
  errorMessage,
  sessionVideoCallbackRef,
  transcriptEndRef,
  aiAudioRef,
  consentDialog,
  onToggleCamera,
  onToggleCaptions,
  onToggleHandsFree,
  onStartRecording,
  onStopRecording,
  onJoin,
  onStop,
}: SessionScreenProps) {
  return (
    <Box sx={{ height: '100vh', width: '100vw', bgcolor: '#606553', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>

      {/* ── Header ── */}
      <Box component="header" sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 3, py: 1.5, flexShrink: 0 }}>
        {/* 企業名 */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, bgcolor: 'rgba(255,255,255,0.18)', borderRadius: 2, px: 2, py: 0.8, backdropFilter: 'blur(8px)' }}>
          <Typography sx={{ color: '#fff', fontWeight: 600, fontSize: 14 }}>
            {companyName || 'AI面接練習'}
          </Typography>
          <Typography sx={{ color: 'rgba(255,255,255,0.65)', fontSize: 12, lineHeight: 1 }}>▾</Typography>
        </Box>

        {/* タイマー + ユーザーアバター */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          {isActive && (
            <Typography sx={{ color: 'rgba(255,255,255,0.75)', fontSize: 13, fontWeight: 500 }}>
              {formatSeconds(remainingSeconds)}
            </Typography>
          )}
          <Box sx={{ width: 34, height: 34, borderRadius: '50%', bgcolor: 'rgba(255,255,255,0.22)', border: '1.5px solid rgba(255,255,255,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Typography sx={{ fontSize: 13, fontWeight: 700, color: '#fff' }}>
              {(userName || 'U').charAt(0).toUpperCase()}
            </Typography>
          </Box>
        </Box>
      </Box>

      {/* ── 残り時間警告 ── */}
      {isConnected && sessionWarningShown && (
        <Box sx={{ px: 2, pb: 1, flexShrink: 0 }}>
          <Box sx={{ bgcolor: remainingSeconds <= 0 ? 'rgba(234,67,53,0.25)' : 'rgba(251,188,4,0.2)', borderRadius: 1.5, px: 2, py: 0.7 }}>
            <Typography sx={{ fontSize: 13, color: remainingSeconds <= 0 ? '#ffb3ae' : '#ffe082', fontWeight: 600 }}>
              {remainingSeconds <= 0 ? 'セッションが終了しました。' : `⚠️ 残り約${Math.ceil(remainingSeconds / 60)}分です。`}
            </Typography>
          </Box>
        </Box>
      )}

      {/* ── 質問タイマー ── */}
      {isConnected && (
        <Box sx={{ px: 2, pb: 1.5, flexShrink: 0 }}>
          <Box sx={{
            bgcolor: 'rgba(0,0,0,0.35)', backdropFilter: 'blur(8px)',
            borderRadius: 2, px: 2, py: 1,
            display: 'flex', alignItems: 'center', gap: 2,
          }}>
            {/* 質問番号バッジ */}
            <Box sx={{
              flexShrink: 0, display: 'flex', alignItems: 'center', gap: 0.5,
              bgcolor: 'rgba(255,255,255,0.12)', borderRadius: 1, px: 1, py: 0.3,
            }}>
              <Typography sx={{ color: 'rgba(255,255,255,0.5)', fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: 0.5 }}>Q</Typography>
              <Typography sx={{ color: '#fff', fontSize: 13, fontWeight: 700, lineHeight: 1 }}>
                {currentQuestionIndex}
              </Typography>
              <Typography sx={{ color: 'rgba(255,255,255,0.4)', fontSize: 10 }}>
                /{totalQuestionCount}
              </Typography>
            </Box>

            {/* プログレスバー */}
            <Box sx={{ flex: 1, height: 5, bgcolor: 'rgba(255,255,255,0.12)', borderRadius: 9999, overflow: 'hidden' }}>
              <Box sx={{
                height: '100%',
                width: `${questionProgress}%`,
                bgcolor: questionRemainingSeconds <= 30 ? '#f28b82'
                       : questionRemainingSeconds <= 60 ? '#fdd663'
                       : '#34a853',
                borderRadius: 9999,
                transition: 'width 1s linear, background-color 0.5s',
              }} />
            </Box>

            {/* カウントダウンテキスト */}
            <Typography sx={{
              flexShrink: 0, fontSize: 12, fontWeight: 600,
              color: questionRemainingSeconds <= 30 ? '#f28b82'
                   : questionRemainingSeconds <= 60 ? '#fdd663'
                   : 'rgba(255,255,255,0.75)',
            }}>
              {questionRemainingLabel}
            </Typography>
            {(isDeepeningQuestion || questionCategory) && (
              <Chip
                size="small"
                label={isDeepeningQuestion ? '深掘り質問' : questionCategory || ''}
                sx={{
                  flexShrink: 0,
                  height: 22,
                  fontSize: 11,
                  bgcolor: isDeepeningQuestion ? 'rgba(251,188,4,0.25)' : 'rgba(255,255,255,0.12)',
                  color: '#fff',
                }}
              />
            )}
          </Box>
        </Box>
      )}

      {/* ── Main ── */}
      <Box sx={{ flex: 1, display: 'flex', gap: 2, px: 2, pb: 2, overflow: 'hidden', minHeight: 0 }}>

        {/* ── 左: アバターパネル ── */}
        <Box sx={{ flex: '0 0 62%', position: 'relative', borderRadius: 3, overflow: 'hidden' }}>

          {/* アバター（全面） */}
          <Box sx={{ width: '100%', height: '100%' }}>
            <ThreeAvatar gender={avatarGender} audioStream={null} level={aiLevel} speaking={aiSpeaking} />
          </Box>

          {/* 面接官ロールラベル */}
          <Box sx={{
            position: 'absolute', top: 12, left: 0, right: 0,
            display: 'flex', justifyContent: 'center',
          }}>
            <Box sx={{
              bgcolor: 'rgba(0,0,0,0.55)', backdropFilter: 'blur(6px)',
              borderRadius: 9999, px: 2, py: 0.6,
              display: 'flex', alignItems: 'center', gap: 0.75,
            }}>
              <Box sx={{ width: 7, height: 7, borderRadius: '50%', bgcolor: '#34a853' }} />
              <Typography sx={{ color: '#fff', fontSize: 12, fontWeight: 600 }}>
                {companyName} 面接官
              </Typography>
            </Box>
          </Box>

          {/* 音声波形 */}
          <Box sx={{
            position: 'absolute', bottom: 88, left: 0, right: 0,
            display: 'flex', justifyContent: 'center', alignItems: 'flex-end',
            gap: '3px', height: 52, px: 6,
          }}>
            {waveHeights.map((h, i) => (
              <Box key={i} sx={{
                width: 3,
                height: `${Math.max(4, h * Math.max(0.12, aiSpeaking ? aiLevel * 1.2 : 0.12))}px`,
                bgcolor: 'rgba(255,255,255,0.75)',
                borderRadius: '2px 2px 1px 1px',
                transition: 'height 0.09s ease',
              }} />
            ))}
          </Box>

          {/* ユーザーカメラ（小）- 左下オーバーレイ */}
          <Box sx={{
            position: 'absolute', bottom: 16, left: 16,
            width: 128, borderRadius: 2, overflow: 'hidden',
            border: '2px solid rgba(255,255,255,0.3)',
            boxShadow: '0 4px 20px rgba(0,0,0,0.5)',
            bgcolor: '#1a1a1a',
          }}>
            <Box sx={{ paddingTop: '75%', position: 'relative' }}>
              <video
                ref={sessionVideoCallbackRef}
                muted
                playsInline
                style={{
                  position: 'absolute', inset: 0,
                  width: '100%', height: '100%',
                  objectFit: 'cover', transform: 'scaleX(-1)',
                  display: cameraEnabled ? 'block' : 'none',
                }}
              />
              {!cameraEnabled && (
                <Box sx={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <VideocamOffIcon sx={{ color: '#666', fontSize: 28 }} />
                </Box>
              )}
            </Box>
          </Box>

          {/* 接続中オーバーレイ */}
          {status === 'connecting' && (
            <Box sx={{ position: 'absolute', inset: 0, bgcolor: 'rgba(0,0,0,0.55)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 2 }}>
              <Typography sx={{ color: '#e8eaed', fontSize: 15 }}>接続中...</Typography>
              <LinearProgress sx={{ width: 160, bgcolor: 'rgba(255,255,255,0.1)', '& .MuiLinearProgress-bar': { bgcolor: PRIMARY } }} />
            </Box>
          )}

          {/* エラーオーバーレイ */}
          {status === 'error' && (
            <Box sx={{ position: 'absolute', inset: 0, bgcolor: 'rgba(0,0,0,0.75)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', p: 3, gap: 2 }}>
              <Typography sx={{ color: '#f28b82', textAlign: 'center', fontSize: 14, lineHeight: 1.6 }}>{errorMessage}</Typography>
              <Button variant="contained" startIcon={<RefreshIcon />} onClick={onJoin}
                sx={{ bgcolor: '#4285f4', '&:hover': { bgcolor: '#3367d6' }, textTransform: 'none' }}>
                再接続する
              </Button>
            </Box>
          )}
        </Box>

        {/* ── 右: チャット + コントロール ── */}
        <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 1.5, minWidth: 0, overflow: 'hidden' }}>

          {/* 発話履歴 */}
          <Box sx={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 1.5, py: 1,
            '&::-webkit-scrollbar': { width: 4 },
            '&::-webkit-scrollbar-thumb': { bgcolor: 'rgba(255,255,255,0.2)', borderRadius: 2 },
          }}>
            {utterances.length === 0 && !partialAi && (
              <Typography sx={{ color: 'rgba(255,255,255,0.4)', fontSize: 13, textAlign: 'center', mt: 4 }}>
                面接が始まると会話がここに表示されます
              </Typography>
            )}
            {utterances.map((u, i) => (
              <Box key={i} sx={{ display: 'flex', flexDirection: 'column', gap: 0.3, alignItems: u.role === 'ai' ? 'flex-start' : 'flex-end' }}>
                <Box sx={{
                  bgcolor: u.role === 'ai' ? 'rgba(255,255,255,0.92)' : 'rgba(255,255,255,0.18)',
                  color: u.role === 'ai' ? '#1a1a1a' : '#fff',
                  px: 2, py: 1.2,
                  borderRadius: u.role === 'ai' ? '4px 16px 16px 16px' : '16px 4px 16px 16px',
                  maxWidth: '88%',
                  boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
                }}>
                  <Typography sx={{ fontSize: 13.5, lineHeight: 1.65 }}>{u.text}</Typography>
                </Box>
              </Box>
            ))}

            {partialAi && (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.3 }}>
                <Box sx={{ bgcolor: 'rgba(255,255,255,0.7)', px: 2, py: 1.2, borderRadius: '4px 16px 16px 16px', maxWidth: '88%', opacity: 0.8 }}>
                  <Typography sx={{ fontSize: 13.5, color: '#1a1a1a', lineHeight: 1.65 }}>{partialAi}</Typography>
                </Box>
              </Box>
            )}

            {partialUser && (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.3, alignItems: 'flex-end' }}>
                <Box sx={{ bgcolor: 'rgba(255,255,255,0.12)', px: 2, py: 1.2, borderRadius: '16px 4px 16px 16px', maxWidth: '88%', opacity: 0.8 }}>
                  <Typography sx={{ fontSize: 13.5, color: '#fff', lineHeight: 1.65 }}>{partialUser}</Typography>
                </Box>
              </Box>
            )}

            <div ref={transcriptEndRef} />
          </Box>

          {/* ── コントロールボタン ── */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, flexShrink: 0 }}>
            {/* サブコントロール行 */}
            <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
              <Tooltip title={cameraEnabled ? 'カメラをオフ' : 'カメラをオン'}>
                <span>
                  <IconButton onClick={onToggleCamera} disabled={!isConnected} size="small"
                    sx={{ bgcolor: cameraEnabled ? 'rgba(255,255,255,0.15)' : '#ea4335', width: 36, height: 36, '&:hover': { bgcolor: cameraEnabled ? 'rgba(255,255,255,0.25)' : '#c5221f' }, '&:disabled': { bgcolor: 'rgba(255,255,255,0.06)' } }}>
                    {cameraEnabled ? <VideocamIcon sx={{ color: '#fff', fontSize: 18 }} /> : <VideocamOffIcon sx={{ color: '#fff', fontSize: 18 }} />}
                  </IconButton>
                </span>
              </Tooltip>
              <Tooltip title={captionsVisible ? '字幕をオフ' : '字幕をオン'}>
                <IconButton onClick={onToggleCaptions} size="small"
                  sx={{ bgcolor: captionsVisible ? 'rgba(255,255,255,0.25)' : 'rgba(255,255,255,0.1)', width: 36, height: 36 }}>
                  <ClosedCaptionIcon sx={{ color: captionsVisible ? '#fff' : 'rgba(255,255,255,0.5)', fontSize: 18 }} />
                </IconButton>
              </Tooltip>
              <Tooltip title={handsFreeMode ? 'ハンズフリーをオフ' : 'ハンズフリーをオン'}>
                <IconButton onClick={onToggleHandsFree} disabled={!isConnected} size="small"
                  sx={{ bgcolor: handsFreeMode ? 'rgba(255,255,255,0.25)' : 'rgba(255,255,255,0.1)', width: 36, height: 36 }}>
                  <MicIcon sx={{ color: handsFreeMode ? '#fff' : 'rgba(255,255,255,0.5)', fontSize: 18 }} />
                </IconButton>
              </Tooltip>
            </Box>

            {/* 主要ボタン行 */}
            <Box sx={{ display: 'flex', gap: 1 }}>
              {/* 録音 / 話す ボタン */}
              <Tooltip title={aiSpeaking ? 'AI発話中...' : turnPending ? 'AIが考えています...' : isRecording ? 'クリックして送信' : 'クリックして話す'}>
                <span style={{ flex: 1 }}>
                  <Button
                    fullWidth
                    onClick={isRecording ? onStopRecording : onStartRecording}
                    disabled={!isConnected || turnPending || aiSpeaking}
                    startIcon={
                      turnPending
                        ? <CircularProgress size={16} sx={{ color: 'inherit' }} />
                        : aiSpeaking
                          ? <VolumeUpIcon sx={{ fontSize: 18 }} />
                          : <MicIcon sx={{ fontSize: 18 }} />
                    }
                    sx={{
                      borderRadius: 9999, py: 1.1,
                      bgcolor: isRecording ? 'rgba(234,67,53,0.25)' : 'rgba(255,255,255,0.18)',
                      color: isRecording ? '#ff8a80' : 'rgba(255,255,255,0.9)',
                      border: `1px solid ${isRecording ? 'rgba(234,67,53,0.5)' : 'rgba(255,255,255,0.25)'}`,
                      textTransform: 'none', fontWeight: 600, fontSize: 13,
                      animation: isRecording ? 'micPulse 1.4s ease-in-out infinite' : 'none',
                      '@keyframes micPulse': { '0%,100%': { opacity: 1 }, '50%': { opacity: 0.75 } },
                      '&:hover': { bgcolor: isRecording ? 'rgba(234,67,53,0.35)' : 'rgba(255,255,255,0.25)' },
                      '&:disabled': { bgcolor: 'rgba(255,255,255,0.06)', color: 'rgba(255,255,255,0.3)', border: '1px solid rgba(255,255,255,0.1)' },
                    }}
                  >
                    {isRecording ? 'レコーディング中...' : turnPending ? '処理中...' : aiSpeaking ? 'AI発話中' : '話す'}
                  </Button>
                </span>
              </Tooltip>

              {/* 完了 / 開始ボタン */}
              {!isActive ? (
                <Button
                  variant="contained"
                  onClick={onJoin}
                  sx={{ borderRadius: 9999, px: 2.5, py: 1.1, fontWeight: 600, textTransform: 'none', fontSize: 13, bgcolor: '#34a853', '&:hover': { bgcolor: '#2d8f47' }, flexShrink: 0 }}
                >
                  面接を開始
                </Button>
              ) : (
                <Button
                  variant="contained"
                  onClick={() => onStop(false)}
                  sx={{ borderRadius: 9999, px: 2.5, py: 1.1, fontWeight: 600, textTransform: 'none', fontSize: 13, bgcolor: 'rgba(255,255,255,0.22)', color: '#fff', border: '1px solid rgba(255,255,255,0.3)', boxShadow: 'none', '&:hover': { bgcolor: 'rgba(255,255,255,0.32)', boxShadow: 'none' }, flexShrink: 0 }}
                >
                  完了する
                </Button>
              )}
            </Box>
          </Box>
        </Box>
      </Box>

      <audio ref={aiAudioRef} />

      {consentDialog}
    </Box>
  )
}
