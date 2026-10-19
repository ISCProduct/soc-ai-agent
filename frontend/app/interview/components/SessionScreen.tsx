'use client'

import { useEffect, useState, type ReactNode, type RefCallback, type RefObject } from 'react'
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
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import HistoryIcon from '@mui/icons-material/History'

import { UTTERANCE_SAVE_FAILED_MESSAGE } from '../utteranceSave'
import { formatSeconds } from '@/lib/interview/utils'
import type { Utterance } from '../types'
import styles from './interview.module.css'
import { UI } from '@/lib/design/tokens'
import { setInterviewSessionActive } from '../sessionChrome'
import Avatar2D from './Avatar2D'

/** 回答に迷ったときの思考の補助線。チケット固有の採点基準ではなく、どの質問にも共通する一般的な枠組み。 */
const STAR_GUIDE = [
  { key: 'S', label: 'Situation', description: 'どんな状況・背景だったか' },
  { key: 'T', label: 'Task', description: '達成すべき課題は何だったか' },
  { key: 'A', label: 'Action', description: '自分が具体的に取った行動' },
  { key: 'R', label: 'Result', description: '行動の結果どうなったか' },
] as const

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
  /** 学生の発話の区切りの回数。変わるたびに面接官が相槌を打つ。 */
  studentPauseCount?: number
  cameraEnabled: boolean
  captionsVisible: boolean
  handsFreeMode: boolean
  utterances: Utterance[]
  partialAi: string
  partialUser: string
  isRecording: boolean
  turnPending: boolean
  errorMessage: string | null
  /** 発話保存が再試行しても失敗したか。true なら「記録できていない」ことを画面に出す（#1476） */
  utteranceSaveFailed?: boolean
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
  studentPauseCount = 0,
  cameraEnabled,
  captionsVisible,
  handsFreeMode,
  utterances,
  partialAi,
  partialUser,
  isRecording,
  turnPending,
  errorMessage,
  utteranceSaveFailed,
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
  // 面接セッション中は学生下部ナビを隠す（#1569）。SelectionScreen/LobbyScreen は
  // 同じ /interview パス上の別 status なので、ここでは「このコンポーネントが
  // マウントされている間」を「セッション中」として扱う。
  useEffect(() => {
    setInterviewSessionActive(true)
    return () => setInterviewSessionActive(false)
  }, [])

  const [starGuideOpen, setStarGuideOpen] = useState(true)
  const [historyOpen, setHistoryOpen] = useState(false)

  // 「現在の質問」「ライブ回答」は新規のデータを持たず、既存の発話ストリーム
  // (utterances/partialAi/partialUser)から導出する。確定済みの発話が無い間は
  // 直近のAI/ユーザー発話にフォールバックする。
  const latestAiUtterance = [...utterances].reverse().find((u) => u.role === 'ai')?.text ?? ''
  const currentQuestionText = partialAi || latestAiUtterance
  const latestUserUtterance = [...utterances].reverse().find((u) => u.role === 'user')?.text ?? ''
  const liveAnswerText = partialUser || latestUserUtterance

  return (
    <Box className={styles.sessionRoot} sx={{ bgcolor: UI.nightStage, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>

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
              <Typography sx={{ color: UI.nightMuted, fontSize: 10, fontWeight: 700, textTransform: 'uppercase', letterSpacing: 0.5 }}>Q</Typography>
              <Typography sx={{ color: '#fff', fontSize: 13, fontWeight: 700, lineHeight: 1 }}>
                {currentQuestionIndex}
              </Typography>
              <Typography sx={{ color: UI.nightMuted, fontSize: 10 }}>
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
      {/* md未満では縦積みにする（自分のカメラ→面接官→質問パネル）。
          useMediaQueryでの分岐は使わず、sxのブレークポイントだけで解く
          （video要素の再マウント・Realtime再接続を避けるため #1569） */}
      {/* xsでは全体を縦スクロール可能にする。自分のカメラ+面接官帯(固定高さ)と
          質問パネルの合計が、横向き(844x390等)の短いビューポートではヘッダー分を
          引いた残り高さに収まらないことがあり、overflow:hiddenのままだと右パネルの
          操作ボタンが高さ0まで圧縮されて消える（レビュー指摘, #1569の再発パターン）。
          右パネル側にもminHeightの floor を持たせた上で、収まらない場合は
          画面全体をスクロールして操作ボタンへ到達できるようにする。 */}
      <Box sx={{ flex: 1, display: 'flex', flexDirection: { xs: 'column', md: 'row' }, gap: 2, px: 2, pb: 2, overflowX: 'hidden', overflowY: { xs: 'auto', md: 'hidden' }, minHeight: 0 }}>

        {/* ── 左: 自分のカメラ(主役) + 面接官(2Dアバター、小) ── */}
        {/* 自分の映像を中心に据える構成に変更(#2D化)。以前は面接官アバターが
            全面で自分のカメラは左下の小さいオーバーレイだったが、面接練習で
            本人が自分の映り・姿勢を確認しやすいよう主従を入れ替えた。 */}
        <Box sx={{ flex: { xs: '0 0 auto', md: '0 0 58%' }, display: 'flex', flexDirection: 'column', gap: 1.5, minHeight: 0 }}>

          {/* 自分のカメラ(大) */}
          <Box sx={{ flex: { xs: '0 0 auto', md: 1 }, height: { xs: 180, md: 'auto' }, minHeight: 0, position: 'relative', borderRadius: 3, overflow: 'hidden', bgcolor: '#1a1a1a' }}>
            <video
              ref={sessionVideoCallbackRef}
              muted
              playsInline
              className={`${styles.cameraVideo} ${styles.cameraVideoFill} ${cameraEnabled ? '' : styles.cameraVideoHidden}`}
            />
            {!cameraEnabled && (
              <Box sx={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <VideocamOffIcon sx={{ color: '#666', fontSize: 40 }} />
              </Box>
            )}

            {/* 自分のラベル */}
            <Box sx={{
              position: 'absolute', top: 12, left: 12,
              bgcolor: 'rgba(0,0,0,0.55)', backdropFilter: 'blur(6px)',
              borderRadius: 9999, px: 1.75, py: 0.5,
            }}>
              <Typography sx={{ color: '#fff', fontSize: 12, fontWeight: 600 }}>
                あなた{userName ? `（${userName}）` : ''}
              </Typography>
            </Box>

            {/* 接続中オーバーレイ */}
            {status === 'connecting' && (
              <Box sx={{ position: 'absolute', inset: 0, bgcolor: 'rgba(0,0,0,0.55)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 2 }}>
                <Typography sx={{ color: UI.nightInk, fontSize: 15 }}>接続中...</Typography>
                <LinearProgress sx={{ width: 160, bgcolor: 'rgba(255,255,255,0.1)', '& .MuiLinearProgress-bar': { bgcolor: UI.mark } }} />
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

          {/* 面接官(2Dアバター、小) + 発話中の字幕 */}
          <Box sx={{ flexShrink: 0, height: { xs: 100, md: 128 }, display: 'flex', gap: 1.5 }}>
            <Box sx={{ width: { xs: 100, md: 128 }, flexShrink: 0, borderRadius: 2, overflow: 'hidden', position: 'relative', bgcolor: UI.nightPanel, border: '1px solid rgba(255,255,255,0.14)' }}>
              <Avatar2D
                gender={avatarGender}
                level={aiLevel}
                speaking={aiSpeaking}
                active={isConnected}
                pending={turnPending}
                acknowledgeSignal={studentPauseCount}
              />
              <Box sx={{
                position: 'absolute', top: 6, left: 6, right: 6,
                display: 'flex', alignItems: 'center', gap: 0.5,
              }}>
                <Box sx={{ width: 6, height: 6, borderRadius: '50%', bgcolor: aiSpeaking ? '#34a853' : 'rgba(255,255,255,0.4)', flexShrink: 0 }} />
                <Typography noWrap sx={{ color: '#fff', fontSize: 10.5, fontWeight: 600 }}>
                  {(companyName || 'AI面接練習') + ' 面接官'}
                </Typography>
              </Box>
            </Box>

            {/* 発話中の字幕(リアルタイムTTSテキスト)。「字幕」トグルで表示/非表示。
                右パネルの「現在の質問」カードと内容が重複するため、実際に
                読み上げている間だけ出す（クローズドキャプションと同じ考え方）。 */}
            {captionsVisible && aiSpeaking ? (
              <Box sx={{
                flex: 1, minWidth: 0, borderRadius: 2, px: 2, py: 1.25,
                bgcolor: 'rgba(0,0,0,0.35)', backdropFilter: 'blur(8px)',
                display: 'flex', flexDirection: 'column', justifyContent: 'center', gap: 0.3,
                overflow: 'hidden',
              }}>
                <Typography sx={{ color: 'rgba(255,255,255,0.6)', fontSize: 10, fontWeight: 700, letterSpacing: 0.5 }}>
                  AI面接官 音声読み上げ中
                </Typography>
                <Typography sx={{
                  color: '#fff', fontSize: 13, lineHeight: 1.5,
                  display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical', overflow: 'hidden',
                }}>
                  {partialAi || currentQuestionText}
                </Typography>
              </Box>
            ) : (
              <Box sx={{ flex: 1 }} />
            )}
          </Box>
        </Box>

        {/* ── 右: チャット + コントロール ── */}
        {/* minHeight:0 がないとflex:1の文字起こし領域がoverflow:autoで
            縮まらず、コントロールが画面外へ押し出される（xs縦積み時） */}
        {/* xsでは右パネル自体を縦スクロール可能にする。横向き(844x390等)で
            アバター帯200px+ヘッダー+2行の操作行の合計がビューポートを超えると
            overflow:hiddenのままでは主要ボタンが完全にクリップされる
            （レビュー指摘, #1569） */}
        {/* minHeightに床を設ける。0のままだと、短い縦積みビューポートで左の
            カメラ+面接官帯に押されてこのパネルが高さ0まで潰れ、話す/完了する
            ボタンが見えなくなる（レビュー指摘）。床を超える分は、親Main側の
            overflowY:autoで画面全体をスクロールして到達できるようにしてある。 */}
        <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 1.5, minWidth: 0, minHeight: { xs: 220, md: 0 }, overflowX: 'hidden', overflowY: { xs: 'auto', md: 'hidden' } }}>

          {/* 発話保存の失敗を面接中に見せる（#1476）。面接は続行できるため止めない */}
          {utteranceSaveFailed && (
            <Box role="alert" sx={{ bgcolor: 'rgba(251,188,5,0.15)', border: '1px solid rgba(251,188,5,0.4)', borderRadius: 2, p: 1.5 }}>
              <Typography sx={{ color: '#fdd663', fontSize: 13, lineHeight: 1.6 }}>
                {UTTERANCE_SAVE_FAILED_MESSAGE}
              </Typography>
            </Box>
          )}

          {/* 質問・回答エリア(スクロール可能) */}
          <Box sx={{ flex: 1, overflowY: 'auto', minHeight: 0, display: 'flex', flexDirection: 'column', gap: 1.5, py: 1,
            '&::-webkit-scrollbar': { width: 4 },
            '&::-webkit-scrollbar-thumb': { bgcolor: 'rgba(255,255,255,0.2)', borderRadius: 2 },
          }}>
            {utterances.length === 0 && !partialAi ? (
              <Typography sx={{ color: 'rgba(255,255,255,0.4)', fontSize: 13, textAlign: 'center', mt: 4 }}>
                面接が始まると質問がここに表示されます
              </Typography>
            ) : (
              <>
                {/* 現在の質問 */}
                <Box sx={{ bgcolor: 'rgba(255,255,255,0.08)', border: '1px solid rgba(255,255,255,0.12)', borderRadius: 2, p: 2 }}>
                  <Typography sx={{ color: 'rgba(255,255,255,0.7)', fontSize: 10.5, fontWeight: 700, letterSpacing: 0.5, mb: 1 }}>
                    {isActive ? '現在の質問' : '前回の質問'}
                  </Typography>
                  <Typography sx={{ color: '#fff', fontSize: 15, lineHeight: 1.75, whiteSpace: 'pre-wrap' }}>
                    {currentQuestionText || '質問を準備しています…'}
                  </Typography>
                </Box>

                {/* STAR回答フレームワーク思考ガイド */}
                <Box sx={{ bgcolor: 'rgba(255,255,255,0.05)', border: '1px solid rgba(255,255,255,0.1)', borderRadius: 2, overflow: 'hidden' }}>
                  <Box
                    role="button"
                    tabIndex={0}
                    onClick={() => setStarGuideOpen((v) => !v)}
                    onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setStarGuideOpen((v) => !v) } }}
                    aria-expanded={starGuideOpen}
                    sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 2, py: 1.25, cursor: 'pointer', minHeight: 44 }}
                  >
                    <Typography sx={{ color: 'rgba(255,255,255,0.85)', fontSize: 12.5, fontWeight: 600 }}>
                      STAR回答フレームワーク思考ガイド
                    </Typography>
                    <ExpandMoreIcon sx={{ color: 'rgba(255,255,255,0.6)', fontSize: 20, transform: starGuideOpen ? 'rotate(180deg)' : 'none', transition: 'transform 0.15s ease' }} />
                  </Box>
                  {starGuideOpen && (
                    <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1, px: 2, pb: 2 }}>
                      {STAR_GUIDE.map((s) => (
                        <Box key={s.key} sx={{ bgcolor: 'rgba(255,255,255,0.06)', borderRadius: 1.5, p: 1.25 }}>
                          {/* UI.markは白背景前提の色で、この画面の夜間背景ではほぼ見えない
                              (コントラスト比約1.1:1、レビュー指摘)。暗背景用の見やすい
                              アクセント色に変える。 */}
                          <Typography sx={{ color: '#8ab4f8', fontSize: 12, fontWeight: 700 }}>
                            {s.key} ({s.label})
                          </Typography>
                          <Typography sx={{ color: 'rgba(255,255,255,0.75)', fontSize: 11.5, lineHeight: 1.5, mt: 0.3 }}>
                            {s.description}
                          </Typography>
                        </Box>
                      ))}
                    </Box>
                  )}
                </Box>

                {/* ライブ回答(あなたの発話) */}
                {isActive && (
                  <Box sx={{ bgcolor: 'rgba(255,255,255,0.08)', border: '1px solid rgba(255,255,255,0.12)', borderRadius: 2, p: 2 }}>
                    <Typography sx={{ color: 'rgba(255,255,255,0.7)', fontSize: 10.5, fontWeight: 700, letterSpacing: 0.5, mb: 1 }}>
                      あなたの回答{partialUser ? '（認識中）' : ''}
                    </Typography>
                    <Typography sx={{ color: liveAnswerText ? '#fff' : 'rgba(255,255,255,0.4)', fontSize: 13.5, lineHeight: 1.65, whiteSpace: 'pre-wrap' }}>
                      {liveAnswerText || '「話す」ボタンを押して回答してください'}
                    </Typography>
                  </Box>
                )}

                {/* これまでの質問・回答(折りたたみ) */}
                {utterances.length > 0 && (
                  <Box sx={{ bgcolor: 'rgba(255,255,255,0.05)', border: '1px solid rgba(255,255,255,0.1)', borderRadius: 2, overflow: 'hidden' }}>
                    <Box
                      role="button"
                      tabIndex={0}
                      onClick={() => setHistoryOpen((v) => !v)}
                      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setHistoryOpen((v) => !v) } }}
                      aria-expanded={historyOpen}
                      sx={{ display: 'flex', alignItems: 'center', gap: 0.75, px: 2, py: 1.25, cursor: 'pointer', minHeight: 44 }}
                    >
                      <HistoryIcon sx={{ color: 'rgba(255,255,255,0.6)', fontSize: 18 }} />
                      <Typography sx={{ color: 'rgba(255,255,255,0.85)', fontSize: 12.5, fontWeight: 600, flex: 1 }}>
                        これまでの質問・回答（{utterances.length}）
                      </Typography>
                      <ExpandMoreIcon sx={{ color: 'rgba(255,255,255,0.6)', fontSize: 20, transform: historyOpen ? 'rotate(180deg)' : 'none', transition: 'transform 0.15s ease' }} />
                    </Box>
                    {historyOpen && (
                      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, px: 2, pb: 2 }}>
                        {utterances.map((u, i) => (
                          <Box key={i} sx={{ display: 'flex', flexDirection: 'column', gap: 0.3, alignItems: u.role === 'ai' ? 'flex-start' : 'flex-end' }}>
                            <Box sx={{
                              bgcolor: u.role === 'ai' ? 'rgba(255,255,255,0.92)' : 'rgba(255,255,255,0.18)',
                              color: u.role === 'ai' ? '#1a1a1a' : '#fff',
                              px: 2, py: 1.2,
                              borderRadius: u.role === 'ai' ? '4px 16px 16px 16px' : '16px 4px 16px 16px',
                              maxWidth: '92%',
                            }}>
                              <Typography sx={{ fontSize: 13, lineHeight: 1.6 }}>{u.text}</Typography>
                            </Box>
                          </Box>
                        ))}
                      </Box>
                    )}
                  </Box>
                )}
              </>
            )}

            <div ref={transcriptEndRef} />
          </Box>

          {/* ── コントロールボタン ── */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, flexShrink: 0 }}>
            {/* サブコントロール行 */}
            <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
              {/* #1481: タップ領域は44px以上にする */}
              <Tooltip title={cameraEnabled ? 'カメラをオフ' : 'カメラをオン'}>
                <span>
                  <IconButton onClick={onToggleCamera} disabled={!isConnected} aria-label="カメラの入切"
                    sx={{ bgcolor: cameraEnabled ? 'rgba(255,255,255,0.15)' : '#ea4335', width: 44, height: 44, '&:hover': { bgcolor: cameraEnabled ? 'rgba(255,255,255,0.25)' : '#c5221f' }, '&:disabled': { bgcolor: 'rgba(255,255,255,0.06)' } }}>
                    {cameraEnabled ? <VideocamIcon sx={{ color: '#fff', fontSize: 18 }} /> : <VideocamOffIcon sx={{ color: '#fff', fontSize: 18 }} />}
                  </IconButton>
                </span>
              </Tooltip>
              <Tooltip title={captionsVisible ? '字幕をオフ' : '字幕をオン'}>
                <IconButton onClick={onToggleCaptions} aria-label="字幕の表示切替"
                  sx={{ bgcolor: captionsVisible ? 'rgba(255,255,255,0.25)' : 'rgba(255,255,255,0.1)', width: 44, height: 44 }}>
                  <ClosedCaptionIcon sx={{ color: captionsVisible ? '#fff' : 'rgba(255,255,255,0.5)', fontSize: 18 }} />
                </IconButton>
              </Tooltip>
              <Tooltip title={handsFreeMode ? 'ハンズフリーをオフ' : 'ハンズフリーをオン'}>
                <IconButton onClick={onToggleHandsFree} disabled={!isConnected} aria-label="ハンズフリーの入切"
                  sx={{ bgcolor: handsFreeMode ? 'rgba(255,255,255,0.25)' : 'rgba(255,255,255,0.1)', width: 44, height: 44 }}>
                  <MicIcon sx={{ color: handsFreeMode ? '#fff' : 'rgba(255,255,255,0.5)', fontSize: 18 }} />
                </IconButton>
              </Tooltip>
            </Box>

            {/* 主要ボタン行 */}
            <Box sx={{ display: 'flex', gap: 1 }}>
              {/* 録音 / 話す ボタン */}
              {/*
                「AI発話中」「AIが考えています」をやめた。
                面接練習は「面接官と話している」という前提が成り立ってこそ
                練習になる。話し相手をAIと呼ぶとその前提が崩れる。
              */}
              <Tooltip title={aiSpeaking ? '面接官が話しています' : turnPending ? '面接官が考えています' : isRecording ? 'クリックして送信' : 'クリックして話す'}>
                <span className={styles.recordButtonWrap}>
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
                      borderRadius: 9999, py: 1.1, minHeight: 44,
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
                    {isRecording ? '録音中' : turnPending ? '送信中' : aiSpeaking ? '面接官が発言中' : '話す'}
                  </Button>
                </span>
              </Tooltip>

              {/* 完了 / 開始ボタン */}
              {!isActive ? (
                <Button
                  variant="contained"
                  onClick={onJoin}
                  sx={{ borderRadius: 9999, px: 2.5, py: 1.1, minHeight: 44, fontWeight: 600, textTransform: 'none', fontSize: 13, bgcolor: '#34a853', '&:hover': { bgcolor: '#2d8f47' }, flexShrink: 0 }}
                >
                  面接を開始
                </Button>
              ) : (
                <Button
                  variant="contained"
                  onClick={() => onStop(false)}
                  sx={{ borderRadius: 9999, px: 2.5, py: 1.1, minHeight: 44, fontWeight: 600, textTransform: 'none', fontSize: 13, bgcolor: 'rgba(255,255,255,0.22)', color: '#fff', border: '1px solid rgba(255,255,255,0.3)', boxShadow: 'none', '&:hover': { bgcolor: 'rgba(255,255,255,0.32)', boxShadow: 'none' }, flexShrink: 0 }}
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
