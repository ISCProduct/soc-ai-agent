'use client'

import type { ReactNode } from 'react'
import { Box } from '@mui/material'
import styles from './MuiChat.module.css'
import { PageLoading } from '@/components/common/PageLoading'
import { useMuiChat } from './hooks/useMuiChat'
import { ChatHeader } from './components/ChatHeader'
import { ChatMessageList } from './components/ChatMessageList'
import { ChatInputBar } from './components/ChatInputBar'
import { ChatModals } from './components/ChatModals'

/** 診断を始める前（案内のメッセージだけ）の表示を差し替えるときに渡す値。 */
export interface MuiChatInitialViewContext {
  /** 回答として送る。履歴・送信の状態は MuiChat（useMuiChat）が持つ。 */
  onSend: (text: string) => void
  isLoading: boolean
  /** 診断の進捗（有効な回答数 / 想定の質問数 / 割合） */
  progress: { valid: number; required: number; percent: number }
}

interface MuiChatProps {
  /**
   * 診断を始める前の表示。渡すと、履歴が0件（案内のメッセージだけ）で、履歴の読み込みが終わり失敗もしていないときだけ、
   * チャットの代わりにこれを出す。1通でも送れば、従来のチャット表示に切り替わる。
   */
  initialView?: (ctx: MuiChatInitialViewContext) => ReactNode
}

/** 履歴が0件のときに useMuiChat が入れる、案内のメッセージだけの状態か（id '0'）。 */
function isInitialState(messages: { id: string; role: string }[]): boolean {
  return messages.length === 1 && messages[0].id === '0' && messages[0].role === 'assistant'
}

/**
 * IT業界キャリアエージェントのチャット UI。
 * 状態・副作用は useMuiChat、表示は各コンポーネントに委譲する。
 */
export function MuiChat({ initialView }: MuiChatProps = {}) {
  const chat = useMuiChat()

  if (!chat.mounted) {
    return (
      <PageLoading fullScreen={false} showBrand={false} message="チャットを準備しています..." />
    )
  }

  const showInitialView =
    initialView !== undefined &&
    isInitialState(chat.messages) &&
    !chat.historyLoading &&
    !chat.historyLoadError &&
    !chat.analysisComplete

  return (
    <>
      <ChatModals
        showCompletionModal={chat.showCompletionModal}
        showEndChatModal={chat.showEndChatModal}
        showTerminationModal={chat.showTerminationModal}
        allPhasesCompleted={chat.allPhasesCompleted}
        onContinueChat={chat.handleContinueChat}
        onViewResults={chat.handleViewResults}
        onCancelEndChat={chat.handleCancelEndChat}
        onConfirmEndChat={chat.handleConfirmEndChat}
      />

      {showInitialView ? (
        initialView({
          onSend: (text) => {
            void chat.handleSend(text)
          },
          isLoading: chat.isLoading,
          progress: chat.progressTotals,
        })
      ) : (
      <Box className={styles.chatContainer}>
        <ChatHeader
          progressTotals={chat.progressTotals}
          questionCount={chat.questionCount}
          totalQuestions={chat.totalQuestions}
          onEndChat={chat.handleEndChat}
        />

        <ChatMessageList
          messages={chat.messages}
          isLoading={chat.isLoading}
          historyLoading={chat.historyLoading}
          historyLoadError={chat.historyLoadError}
          historyRetrying={chat.historyRetrying}
          messagesEndRef={chat.messagesEndRef}
          messagesAreaRef={chat.messagesAreaRef}
          onRetryHistoryLoad={chat.handleRetryHistoryLoad}
          onQuickSelect={(option) => {
            void chat.handleSend(option)
          }}
        />

        <ChatInputBar
          analysisComplete={chat.analysisComplete}
          showChoiceButtons={chat.showChoiceButtons}
          choiceOptions={chat.choiceOptions}
          selectedChoiceValue={chat.selectedChoiceValue}
          input={chat.input}
          inputPlaceholder={chat.inputPlaceholder}
          isLoading={chat.isLoading}
          historyLoadError={chat.historyLoadError}
          canSend={chat.canSend}
          inputRef={chat.inputRef}
          onInputChange={chat.setInput}
          onSelectChoice={chat.handleSelectChoice}
          onSend={() => {
            void chat.handleSend()
          }}
          onOtherChoice={chat.handleOtherChoice}
          onShowCompletionModal={() => chat.setShowCompletionModal(true)}
        />
      </Box>
      )}
    </>
  )
}
