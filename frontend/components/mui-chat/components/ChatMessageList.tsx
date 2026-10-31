'use client'

import React from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Skeleton,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import { visuallyHidden } from '@mui/utils'
import { ErrorOutline, WarningAmber } from '@mui/icons-material'
import { EmptyState } from '@/components/common/EmptyState'
import styles from '../MuiChat.module.css'
import { TypingIndicator } from './TypingIndicator'
import {
  CHAT_ACCENT,
  CHAT_ACCENT_HOVER,
  CHAT_STOP_EDGE,
  CHAT_STOP_TEXT,
  CHAT_WARN_EDGE,
  CHAT_WARN_TEXT,
  extractChoices,
  findLastAssistantQuestionMessage,
  isValidationFeedbackMessage,
  isValidationTerminationMessage,
  JOB_QUICK_OPTIONS,
  messageAccessibleLabel,
  stripChoiceLines,
} from '../utils'
import type { Message } from '../types'

type ChatMessageListProps = {
  messages: Message[]
  isLoading: boolean
  /** 履歴の初回読み込み中。true のあいだは吹き出しの骨格を出す */
  historyLoading?: boolean
  historyLoadError: string | null
  historyRetrying: boolean
  messagesEndRef: React.RefObject<HTMLDivElement | null>
  messagesAreaRef: React.RefObject<HTMLDivElement | null>
  onRetryHistoryLoad: () => void
  onQuickSelect: (option: string) => void
}

/** メッセージ一覧・履歴エラー・クイック選択・ローディング表示 */
export function ChatMessageList({
  messages,
  isLoading,
  historyLoading = false,
  historyLoadError,
  historyRetrying,
  messagesEndRef,
  messagesAreaRef,
  onRetryHistoryLoad,
  onQuickSelect,
}: ChatMessageListProps) {
  const hasUserMessage = messages.some((m) => m.role === 'user')
  /**
   * 直近の質問が自前の選択肢を持っているか。
   *
   * 選択肢付きの質問では ChatInputBar に A)〜D) が並ぶ。そこへ職種の候補を
   * 重ねると、質問と関係のない候補を選ばせることになるので出さない。
   * 判定は入力欄側（useMuiChat の showChoiceButtons）と同じ 2件以上に揃える。
   */
  const lastAssistantMessage = findLastAssistantQuestionMessage(messages)
  const lastQuestionHasChoices = lastAssistantMessage
    ? extractChoices(lastAssistantMessage.content).length >= 2
    : false
  const showQuickSelect =
    !historyLoadError && !hasUserMessage && messages.length > 0 && !lastQuestionHasChoices

  // ログには名前を与える。無いと読み上げでは「ログ」としか案内されず、
  // 何の一覧なのか分からない。
  //
  // 応答の到着はこのライブリージョンが伝える。politeness は polite のまま。
  // assertive にすると入力中でも読み上げを断ち切るので、書きかけの回答を
  // 見失う。応答は待てる情報なので割り込ませない。
  //
  // aria-relevant は指定しない。"additions" だけに絞ると文字の差し替えが
  // 通知から外れる。role="log" の既定（additions text）に任せる。
  //
  // 読み込み中は aria-busy を立てる。骨格の追加を「新しい発言」として
  // 読み上げさせない（共通の ListSkeleton と同じ扱い）。
  return (
    <Box
      ref={messagesAreaRef}
      className={styles.messagesArea}
      role="log"
      aria-label="自己分析チャットのやり取り"
      aria-live="polite"
      aria-busy={historyLoading}
      sx={{
        flexGrow: 1,
        // 選択肢の最大高さとホーム上部の帯で、この領域が中身の高さまで潰れる。
        // 375×700 でもやり取りが読めるよう、ここは縮めない（#1763）。
        minHeight: 160,
        overflowY: 'auto',
        backgroundColor: '#fff',
      }}
    >
      {historyLoadError && (
        <Box sx={{ textAlign: 'center', mt: 4, px: 2 }}>
          <Alert severity="error" sx={{ mb: 2, textAlign: 'left' }}>
            {historyLoadError}
          </Alert>
          <Button
            variant="contained"
            onClick={onRetryHistoryLoad}
            disabled={historyRetrying}
            startIcon={historyRetrying ? <CircularProgress size={16} color="inherit" /> : undefined}
            sx={{ bgcolor: CHAT_ACCENT, '&:hover': { bgcolor: CHAT_ACCENT_HOVER } }}
          >
            {historyRetrying ? '再読み込み中...' : '再試行'}
          </Button>
        </Box>
      )}

      {/*
        履歴の読み込み中。履歴が0件でも挨拶が入るので、messages が空なのはこの間だけ。
        以前はここが空白で、通信が遅いと壊れているようにしか見えなかった（§23）。
        吹き出しの形に合わせた骨格にして、これから会話が出ることを示す。
      */}
      {historyLoading && messages.length === 0 && !historyLoadError && (
        <>
          {/*
            読み上げ向けの通知。骨格そのものは装飾なので aria-hidden のままにする。
            初期表示の PageLoading は mounted が立った時点で消えるため、そのあと
            履歴を待っている間は、これが無いと空のチャットログと入力欄だけが
            読み上げられ、待機中だと分からない。
          */}
          <Box component="span" role="status" sx={visuallyHidden}>
            チャット履歴を読み込んでいます
          </Box>
          <Box sx={{ px: { xs: 2, md: 3 }, pt: 2 }} aria-hidden>
            {[0, 1, 2].map((i) => (
              <Box
                key={i}
                sx={{
                  display: 'flex',
                  mb: { xs: 2, md: 2.5 },
                  justifyContent: i % 2 === 1 ? 'flex-end' : 'flex-start',
                }}
              >
                <Skeleton
                  variant="rounded"
                  height={i % 2 === 1 ? 40 : 64}
                  sx={{
                    width: i % 2 === 1 ? '45%' : '70%',
                    borderRadius: 2,
                    // Skeleton の既定は pulse で、点滅し続ける。TypingIndicator の
                    // 回転と跳ねだけ止めても、この画面で最初に出る反復アニメーションが
                    // 残ってしまう。止めても灰色の面は残るので、待ち状態は伝わる。
                    '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
                  }}
                />
              </Box>
            ))}
          </Box>
        </>
      )}

      {messages.map((message) => {
        // 目印は utils の VALIDATION_FEEDBACK_MARKERS に寄せる。
        // ここだけ別の文字列を見ていると、文言を変えたときに
        // 片方の判定だけ外れて表示が崩れる。
        const isTerminationMessage =
          message.role === 'assistant' && isValidationTerminationMessage(message.content)

        const isValidationError =
          message.role === 'assistant' &&
          !isTerminationMessage &&
          isValidationFeedbackMessage(message.content)

        const choices =
          message.role === 'assistant' ? extractChoices(message.content) : []
        const displayContent =
          choices.length > 0 ? stripChoiceLines(message.content) : message.content
        const speakerLabel = messageAccessibleLabel(message.role, message.timestamp)

        return (
          <Box
            key={message.id}
            // 1件ずつを名前付きの区切りにする。発言者と時刻が無いと、
            // 読み上げでは誰の発言か分からないまま本文だけが並ぶ。
            component="article"
            aria-label={speakerLabel}
            sx={{
              display: 'flex',
              mb: { xs: 2, md: 2.5 },
              justifyContent: message.role === 'user' ? 'flex-end' : 'flex-start',
            }}
          >
            <Paper
              elevation={0}
              className={styles.messageBubble}
              sx={{
                // 聞き取りの記録として読ませる。
                //
                // 以前は左右に吹き出しを並べ、ロボットと人のアイコンを向かい合わせていた。
                // これは「AIとの雑談」の見た目で、15問の聞き取りという中身と合わない。
                //
                // 質問は枠で囲わず左の罫だけを引いて本文として読ませ、
                // 回答は右寄せで塗り、発言した側に向いた角を落として向きを示す。
                // 位置と形で区別するので、色が読めなくても誰の発言か分かる（§19）。
                backgroundColor:
                  message.role === 'user'
                    ? CHAT_ACCENT
                    : isTerminationMessage
                      ? '#FDF1EA'
                      : isValidationError
                        ? '#FDF6E7'
                        : 'transparent',
                color: message.role === 'user' ? '#fff' : '#1a1a1a',
                border: 'none',
                borderLeft:
                  message.role === 'assistant'
                    ? `3px solid ${
                        isTerminationMessage
                          ? CHAT_STOP_EDGE
                          : isValidationError
                            ? CHAT_WARN_EDGE
                            : CHAT_ACCENT
                      }`
                    : 'none',
                borderRadius:
                  message.role === 'user' ? '12px 12px 2px 12px' : '0 10px 10px 0',
                // 囲いが無いぶん行長が伸びすぎないよう上限を付ける。
                maxWidth: message.role === 'assistant' ? '46rem' : undefined,
              }}
            >
              {/*
                発言者を文字としても置く。ライブリージョンは追加された部分の
                文字列を読み上げるため、発言者が aria-label にしか無いと、
                名前を読む実装では本文が落ち、本文を読む実装では誰の発言かが落ちる。
                画面には出さないので、見た目は変わらない。
              */}
              <Box component="span" sx={visuallyHidden}>
                {speakerLabel}
              </Box>
              {(isTerminationMessage || isValidationError) && (
                <Stack direction="row" alignItems="center" spacing={0.5} sx={{ mb: 0.5 }}>
                  {isTerminationMessage ? (
                    <ErrorOutline sx={{ fontSize: 16, color: CHAT_STOP_TEXT }} />
                  ) : (
                    <WarningAmber sx={{ fontSize: 16, color: CHAT_WARN_TEXT }} />
                  )}
                  <Typography
                    variant="caption"
                    sx={{ fontWeight: 700, color: isTerminationMessage ? CHAT_STOP_TEXT : CHAT_WARN_TEXT }}
                  >
                    {isTerminationMessage ? 'チャット終了' : '注意'}
                  </Typography>
                </Stack>
              )}
              <Typography
                variant="body1"
                sx={{ whiteSpace: 'pre-line', lineHeight: 1.65, fontSize: { xs: '0.9375rem', md: '1rem' } }}
              >
                {displayContent}
              </Typography>
            </Paper>
          </Box>
        )
      })}

      {isLoading && (
        <Box sx={{ display: 'flex', mb: 2, justifyContent: 'flex-start' }} role="status" aria-label="エージェントが入力中です">
          <Paper
            elevation={0}
            className={styles.messageBubble}
            sx={{ backgroundColor: 'transparent', border: 'none', borderLeft: `3px solid ${CHAT_ACCENT}`, borderRadius: '0 10px 10px 0' }}
          >
            <TypingIndicator />
          </Paper>
        </Box>
      )}

      {/*
        初回利用の空状態（§24）。
        まだ1問も答えていないあいだは候補のチップだけが並んでいて、
        今どういう状態なのか・選ぶと何が起きるのか・当てはまるものが
        無いときどうするのかが、どこにも書かれていなかった。
        共通の EmptyState（§30）に寄せて「状態 → 次の操作」の順に読ませる。

        「クイック選択（タップで送信）」のような見出しは置かない。
        押せば送られることはボタンの見た目で分かるので、
        説明を足すとフォームではなくAIの機能紹介に見える。
        チップは EmptyState の操作枠に入れ、名前付きのまとまりとして渡す。
      */}
      {showQuickSelect && (
        <Box sx={{ mt: 1, mb: 2, px: 1 }}>
          <EmptyState
            dense
            title="まだ回答がありません"
            description="近いものを選ぶと、その内容に合わせて次の質問が続きます。当てはまるものが無ければ、下の入力欄に書いて送ってください。"
            action={
              <Stack
                direction="row"
                spacing={1}
                justifyContent="center"
                flexWrap="wrap"
                useFlexGap
                gap={1}
                role="group"
                aria-label="興味のある仕事の候補"
              >
                {JOB_QUICK_OPTIONS.map((option) => (
                  <Chip
                    key={option}
                    label={option}
                    onClick={() => onQuickSelect(option)}
                    clickable
                    sx={{
                      cursor: 'pointer',
                      borderColor: CHAT_ACCENT,
                      '&:hover': { bgcolor: 'rgba(236,91,19,0.08)' },
                    }}
                    variant="outlined"
                  />
                ))}
              </Stack>
            }
          />
        </Box>
      )}

      <div ref={messagesEndRef} />
    </Box>
  )
}
