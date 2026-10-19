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
import MicOffIcon from '@mui/icons-material/MicOff'
import VolumeUpIcon from '@mui/icons-material/VolumeUp'
import VideocamIcon from '@mui/icons-material/Videocam'
import VideocamOffIcon from '@mui/icons-material/VideocamOff'
import RefreshIcon from '@mui/icons-material/Refresh'
import ClosedCaptionIcon from '@mui/icons-material/ClosedCaption'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import HistoryIcon from '@mui/icons-material/History'
import LightbulbOutlinedIcon from '@mui/icons-material/LightbulbOutlined'
import ScheduleIcon from '@mui/icons-material/Schedule'
import CallEndIcon from '@mui/icons-material/CallEnd'

import { UTTERANCE_SAVE_FAILED_MESSAGE } from '../utteranceSave'
import { formatSeconds } from '@/lib/interview/utils'
import type { Utterance } from '../types'
import styles from './interview.module.css'
import { setInterviewSessionActive } from '../sessionChrome'
import Avatar2D from './Avatar2D'

/** 回答に迷ったときの思考の補助線。チケット固有の採点基準ではなく、どの質問にも共通する一般的な枠組み。 */
const STAR_GUIDE = [
  { key: 'S', label: 'Situation', description: 'どんな状況・背景だったか' },
  { key: 'T', label: 'Task', description: '達成すべき課題は何だったか' },
  { key: 'A', label: 'Action', description: '自分が具体的に取った行動' },
  { key: 'R', label: 'Result', description: '行動の結果どうなったか' },
] as const

/** 質問タイマーの緊急度。色はすべて interview.module.css の data-urgency 側で持つ。 */
function urgencyOf(remainingSeconds: number): 'safe' | 'warning' | 'critical' {
  if (remainingSeconds <= 30) return 'critical'
  if (remainingSeconds <= 60) return 'warning'
  return 'safe'
}

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
 * 見た目(色・形)は interview.module.css に集約し、ここでは className の出し分けだけを行う。
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
  const lastAiIndex = utterances.map((u) => u.role).lastIndexOf('ai')
  const latestAiUtterance = lastAiIndex >= 0 ? utterances[lastAiIndex].text : ''
  const currentQuestionText = partialAi || latestAiUtterance
  // 現在の質問(最新のAI発話)より後に来たuser発話だけを「現在の質問への回答」として扱う。
  // 末尾のuser発話をそのまま使うと、次の質問が確定した直後(まだ何も話していない間)に
  // 前の質問への回答が「現在の質問の回答」として出続けてしまう(CodeRabbit指摘)。
  // AI発話が一度も無い(lastAiIndex=-1)間は、質問自体が無いのでuser発話も拾わない
  // (テスター指摘。通常の面接フローではAIが必ず先に質問するため起きないが念のため)。
  const latestUserUtterance = lastAiIndex >= 0
    ? utterances.slice(lastAiIndex + 1).findLast((u) => u.role === 'user')?.text ?? ''
    : ''
  const liveAnswerText = partialUser || latestUserUtterance
  const urgency = urgencyOf(questionRemainingSeconds)

  return (
    <Box className={styles.sessionRoot}>

      {/* ── Header ── */}
      <Box component="header" className={styles.header}>
        {/* LIVE表示 + 企業名 */}
        <Box className={styles.headerTitleRow}>
          {isConnected && (
            <Box className={styles.liveBadge}>
              <Box className={styles.liveDot} />
              <Typography className={styles.liveLabel} component="span">LIVE</Typography>
            </Box>
          )}
          <Box className={styles.headerCompanyRow}>
            <Typography noWrap className={styles.companyName} component="span">
              {companyName || 'AI面接練習'}
            </Typography>
            <Box className={styles.practiceModeBadge}>
              <Typography component="span" className={styles.practiceModeBadgeText}>Practice Mode</Typography>
            </Box>
          </Box>
        </Box>

        {/* タイマー + 終了 + ユーザーアバター */}
        <Box className={styles.headerRight}>
          {isActive && (
            <Box className={styles.timerBadge}>
              <ScheduleIcon sx={{ color: 'var(--primary)', fontSize: 16 }} />
              <Typography className={styles.timerText} component="span">
                {formatSeconds(remainingSeconds)}
              </Typography>
            </Box>
          )}
          {isActive && (
            <Button onClick={() => onStop(false)} startIcon={<CallEndIcon sx={{ fontSize: 18 }} />} className={styles.endButton}>
              <Box component="span" className={styles.endButtonLabel}>終了</Box>
            </Button>
          )}
          <Box className={styles.userAvatar}>
            {(userName || 'U').charAt(0).toUpperCase()}
          </Box>
        </Box>
      </Box>

      {/* ── 残り時間警告 ── */}
      {isConnected && sessionWarningShown && (
        <Box className={styles.warningBanner}>
          <Box className={styles.warningBannerInner} data-level={remainingSeconds <= 0 ? 'critical' : 'warning'}>
            {remainingSeconds <= 0 ? 'セッションが終了しました。' : `⚠️ 残り約${Math.ceil(remainingSeconds / 60)}分です。`}
          </Box>
        </Box>
      )}

      {/* ── スクロール可能なコンテンツ領域(ヘッダー・下部コントロールバーは常に表示されたまま) ── */}
      <Box className={styles.scrollArea}>
        <Box className={styles.scrollAreaInner}>

          {/* ── 質問タイマー ── */}
          {isConnected && (
            <Box className={styles.questionBar}>
              {/* 質問番号バッジ */}
              <Box className={styles.questionBadge}>
                <Typography component="span" className={styles.questionBadgeQ}>Q</Typography>
                <Typography component="span" className={styles.questionBadgeIndex}>{currentQuestionIndex}</Typography>
                <Typography component="span" className={styles.questionBadgeTotal}>/{totalQuestionCount}</Typography>
              </Box>

              {/* プログレスバー */}
              <Box className={styles.progressTrack}>
                <Box className={styles.progressFill} data-urgency={urgency} style={{ width: `${questionProgress}%` }} />
              </Box>

              {/* カウントダウンテキスト */}
              <Typography className={styles.countdownText} data-urgency={urgency} component="span">
                {questionRemainingLabel}
              </Typography>
              {(isDeepeningQuestion || questionCategory) && (
                <Chip
                  size="small"
                  label={isDeepeningQuestion ? '深掘り質問' : questionCategory || ''}
                  sx={{
                    flexShrink: 0, height: 22, fontSize: 11,
                    bgcolor: isDeepeningQuestion ? 'rgba(251,188,4,0.25)' : 'var(--surface-container-high)',
                    color: isDeepeningQuestion ? '#fff' : 'var(--on-surface-variant)',
                  }}
                />
              )}
            </Box>
          )}

          {/* ── Main: 左58% 自分のカメラ+面接官 / 右42% 質問パネル ── */}
          <Box className={styles.mainGrid}>

            {/* ── 左: 自分のカメラ(主役) + 面接官(2Dアバター、小) ── */}
            {/* 自分の映像を中心に据える構成に変更(#2D化)。以前は面接官アバターが
                全面で自分のカメラは左下の小さいオーバーレイだったが、面接練習で
                本人が自分の映り・姿勢を確認しやすいよう主従を入れ替えた。 */}
            <Box className={styles.leftStage}>

              {/* 自分のカメラ(大、16:9) */}
              <Box className={styles.cameraCard}>
                <video
                  ref={sessionVideoCallbackRef}
                  muted
                  playsInline
                  className={`${styles.cameraVideo} ${styles.cameraVideoFill} ${cameraEnabled ? '' : styles.cameraVideoHidden}`}
                />
                {!cameraEnabled && (
                  <Box className={styles.cameraOffIcon}>
                    <VideocamOffIcon sx={{ fontSize: 40 }} />
                  </Box>
                )}

                <Box className={styles.cameraScrim} />
                <Box className={styles.liveFeedBadge}>
                  <Box className={styles.liveFeedDot} />
                  <Typography component="span" className={styles.liveFeedLabel}>LIVE FEED</Typography>
                </Box>

                <Box className={styles.candidateLabelWrap}>
                  <Box className={styles.candidateLabel}>
                    <Typography component="div" className={styles.candidateLabelEyebrow}>CANDIDATE</Typography>
                    <Typography component="div" className={styles.candidateLabelName}>
                      あなた{userName ? `（${userName}）` : ''}
                    </Typography>
                  </Box>
                </Box>

                {/* 接続中オーバーレイ */}
                {status === 'connecting' && (
                  <Box className={styles.statusOverlay} data-tone="connecting">
                    <Typography component="span" className={styles.statusOverlayText}>接続中...</Typography>
                    <LinearProgress sx={{ width: 160, bgcolor: 'var(--surface-variant)', '& .MuiLinearProgress-bar': { bgcolor: 'var(--primary-container)' } }} />
                  </Box>
                )}

                {/* エラーオーバーレイ */}
                {status === 'error' && (
                  <Box className={styles.statusOverlay} data-tone="error">
                    <Typography component="div" className={styles.errorOverlayText}>{errorMessage}</Typography>
                    <Button variant="contained" startIcon={<RefreshIcon />} onClick={onJoin} className={styles.reconnectButton}>
                      再接続する
                    </Button>
                  </Box>
                )}
              </Box>

              {/* 面接官(2Dアバター、小) + 発話中の字幕 */}
              <Box className={styles.avatarCard}>
                <Box className={styles.avatarPanel}>
                  <Avatar2D
                    gender={avatarGender}
                    level={aiLevel}
                    speaking={aiSpeaking}
                    active={isConnected}
                    pending={turnPending}
                    acknowledgeSignal={studentPauseCount}
                  />
                  <Box className={styles.avatarBadgeRow}>
                    <Box className={styles.avatarBadgeDot} data-speaking={aiSpeaking} />
                    <Typography noWrap component="span" className={styles.avatarBadgeLabel}>
                      {(companyName || 'AI面接練習') + ' 面接官'}
                    </Typography>
                  </Box>
                </Box>

                {/* 発話中の字幕(リアルタイムTTSテキスト)。「字幕」トグルで表示/非表示。
                    右パネルの「現在の質問」カードと内容が重複するため、実際に
                    読み上げている間だけ出す（クローズドキャプションと同じ考え方）。 */}
                {captionsVisible && aiSpeaking ? (
                  <Box className={styles.captionBox}>
                    <Box className={styles.captionEyebrowRow}>
                      <VolumeUpIcon sx={{ color: 'var(--primary-container)', fontSize: 14 }} />
                      <Typography component="span" className={styles.captionEyebrow}>
                        AI面接官 音声読み上げ中
                      </Typography>
                    </Box>
                    <Typography component="div" className={styles.captionText}>
                      {partialAi || currentQuestionText}
                    </Typography>
                  </Box>
                ) : (
                  <Box sx={{ flex: 1 }} />
                )}
              </Box>
            </Box>

            {/* ── 右: 質問パネル ── */}
            <Box className={styles.questionPanel}>
              {/* useInterviewSession.ts側が utterances/partialAi の変化ごとにこの
                  要素へ自動スクロールする。以前のチャット履歴UIでは末尾(＝最新発言)に
                  置くのが正しかったが、この画面では「現在の質問」を先頭に固定表示する
                  構成に変えたため、末尾に置いたままだと新しい質問が届くたびに履歴の
                  下までスクロールして現在の質問が画面外に隠れてしまう(Codexレビュー指摘)。
                  パネルの先頭に置き、常に現在の質問が見える位置へスクロールさせる。 */}
              <div ref={transcriptEndRef} />

              {/* 発話保存の失敗を面接中に見せる（#1476）。面接は続行できるため止めない */}
              {utteranceSaveFailed && (
                <Box role="alert" className={styles.saveFailedBanner}>
                  <Typography component="div" className={styles.saveFailedText}>
                    {UTTERANCE_SAVE_FAILED_MESSAGE}
                  </Typography>
                </Box>
              )}

              {utterances.length === 0 && !partialAi ? (
                <Typography component="div" className={styles.emptyQuestionText}>
                  面接が始まると質問がここに表示されます
                </Typography>
              ) : (
                <>
                  {/* CURRENT QUESTION ヘッダー */}
                  <Box className={styles.currentQuestionHeader}>
                    <Typography component="span" className={styles.currentQuestionBadge}>
                      {isActive ? 'CURRENT QUESTION' : '前回の質問'}
                    </Typography>
                    <Typography component="span" className={styles.currentQuestionIndex}>
                      {currentQuestionIndex} / {totalQuestionCount}
                    </Typography>
                  </Box>

                  {/* 質問本文 */}
                  <Box className={styles.insetBox}>
                    <Typography component="div" className={styles.questionText}>
                      {currentQuestionText || '質問を準備しています…'}
                    </Typography>
                  </Box>

                  {/* ライブ回答(あなたの発話、STT) */}
                  {isActive && (
                    <Box className={styles.answerSection}>
                      <Box className={styles.answerEyebrowRow}>
                        <Box className={styles.answerDot} data-active={!!partialUser} />
                        <Typography component="span" className={styles.answerEyebrow}>
                          あなたの回答{partialUser ? '（認識中）' : ''}
                        </Typography>
                      </Box>
                      <Box className={`${styles.insetBox} ${styles.answerBox}`}>
                        <Typography component="div" className={styles.answerText} data-filled={!!liveAnswerText}>
                          {liveAnswerText || '「話す」ボタンを押して回答してください'}
                        </Typography>
                      </Box>
                    </Box>
                  )}

                  {/* STAR回答フレームワーク思考ガイド */}
                  <Box className={styles.collapsibleCard}>
                    <Box
                      role="button"
                      tabIndex={0}
                      onClick={() => setStarGuideOpen((v) => !v)}
                      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setStarGuideOpen((v) => !v) } }}
                      aria-expanded={starGuideOpen}
                      aria-controls="star-guide-panel"
                      className={styles.collapsibleHeader}
                    >
                      <Box className={styles.collapsibleHeaderLeft}>
                        <LightbulbOutlinedIcon sx={{ color: 'var(--tertiary)', fontSize: 18 }} />
                        <Typography component="span" className={styles.collapsibleTitle}>
                          STAR回答フレームワーク思考ガイド
                        </Typography>
                      </Box>
                      <ExpandMoreIcon className={styles.collapsibleChevron} data-open={starGuideOpen} />
                    </Box>
                    {starGuideOpen && (
                      <Box id="star-guide-panel" className={styles.starGrid}>
                        {STAR_GUIDE.map((s) => (
                          <Box key={s.key} className={styles.starItem}>
                            <Typography component="div" className={styles.starKey}>
                              {s.key} ({s.label})
                            </Typography>
                            <Typography component="div" className={styles.starDescription}>
                              {s.description}
                            </Typography>
                          </Box>
                        ))}
                      </Box>
                    )}
                  </Box>

                  {/* これまでの質問・回答(折りたたみ) */}
                  {utterances.length > 0 && (
                    <Box className={styles.collapsibleCard}>
                      <Box
                        role="button"
                        tabIndex={0}
                        onClick={() => setHistoryOpen((v) => !v)}
                        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setHistoryOpen((v) => !v) } }}
                        aria-expanded={historyOpen}
                        aria-controls="interview-history-panel"
                        className={styles.collapsibleHeader}
                      >
                        <HistoryIcon sx={{ color: 'var(--on-surface-variant)', fontSize: 18 }} />
                        <Typography component="span" className={styles.collapsibleTitle}>
                          これまでの質問・回答（{utterances.length}）
                        </Typography>
                        <ExpandMoreIcon className={styles.collapsibleChevron} data-open={historyOpen} />
                      </Box>
                      {historyOpen && (
                        <Box id="interview-history-panel" className={styles.historyList}>
                          {utterances.map((u, i) => (
                            <Box key={i} className={styles.historyRow} data-role={u.role}>
                              <Box className={styles.historyBubble} data-role={u.role}>
                                {u.text}
                              </Box>
                            </Box>
                          ))}
                        </Box>
                      )}
                    </Box>
                  )}
                </>
              )}
            </Box>
          </Box>
        </Box>
      </Box>

      {/* ── 下部コントロールバー(常に表示。スクロール領域の外に置き、
          短いビューポートでも操作ボタンが隠れないようにする) ── */}
      <Box className={styles.controlBar}>
        <Box className={styles.controlBarInner}>
          {/* #1481: タップ領域は44px以上にする */}
          <Tooltip title={cameraEnabled ? 'カメラをオフ' : 'カメラをオン'}>
            <span>
              <IconButton onClick={onToggleCamera} disabled={!isConnected} aria-label="カメラの入切" className={styles.iconToggle}>
                {cameraEnabled ? <VideocamIcon sx={{ color: 'var(--primary)', fontSize: 18 }} /> : <VideocamOffIcon sx={{ color: 'var(--on-surface-variant)', fontSize: 18 }} />}
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={captionsVisible ? '字幕をオフ' : '字幕をオン'}>
            <IconButton onClick={onToggleCaptions} aria-label="字幕の表示切替" className={styles.iconToggle} data-active={captionsVisible}>
              <ClosedCaptionIcon sx={{ color: captionsVisible ? 'var(--primary)' : 'var(--on-surface-variant)', fontSize: 18 }} />
            </IconButton>
          </Tooltip>
          <Tooltip title={handsFreeMode ? 'ハンズフリーをオフ' : 'ハンズフリーをオン'}>
            <span>
              <IconButton onClick={onToggleHandsFree} disabled={!isConnected} aria-label="ハンズフリーの入切" className={styles.iconToggle} data-active={handsFreeMode}>
                {handsFreeMode ? <MicIcon sx={{ color: 'var(--primary)', fontSize: 18 }} /> : <MicOffIcon sx={{ color: 'var(--on-surface-variant)', fontSize: 18 }} />}
              </IconButton>
            </span>
          </Tooltip>

          <Box className={styles.controlDivider} />

          {/* 録音 / 話す ボタン */}
          {/*
            「AI発話中」「AIが考えています」をやめた。
            面接練習は「面接官と話している」という前提が成り立ってこそ
            練習になる。話し相手をAIと呼ぶとその前提が崩れる。
          */}
          <Tooltip title={aiSpeaking ? '面接官が話しています' : turnPending ? '面接官が考えています' : isRecording ? 'クリックして送信' : 'クリックして話す'}>
            <span>
              <Button
                onClick={isRecording ? onStopRecording : onStartRecording}
                disabled={!isConnected || turnPending || aiSpeaking}
                startIcon={
                  turnPending
                    ? <CircularProgress size={16} sx={{ color: 'inherit' }} />
                    : aiSpeaking
                      ? <VolumeUpIcon sx={{ fontSize: 18 }} />
                      : <MicIcon sx={{ fontSize: 18 }} />
                }
                className={styles.talkButton}
                data-recording={isRecording}
              >
                {isRecording ? '録音中' : turnPending ? '送信中' : aiSpeaking ? '面接官が発言中' : '話す'}
              </Button>
            </span>
          </Tooltip>

          {/* 完了 / 開始ボタン */}
          {!isActive ? (
            <Button variant="contained" onClick={onJoin} className={styles.primaryActionButton}>
              面接を開始
            </Button>
          ) : (
            <Button variant="contained" onClick={() => onStop(false)} className={styles.primaryActionButton}>
              完了する
            </Button>
          )}
        </Box>
      </Box>

      <audio ref={aiAudioRef} />

      {consentDialog}
    </Box>
  )
}
