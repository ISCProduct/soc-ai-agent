'use client'

import { Box } from '@mui/material'
import styles from './mui-chat.module.css'
import { PageLoading } from '@/components/common/PageLoading'
import { useMuiChat } from './mui-chat/hooks/useMuiChat'
import { ChatHeader } from './mui-chat/components/ChatHeader'
import { ChatMessageList } from './mui-chat/components/ChatMessageList'
import { ChatInputBar } from './mui-chat/components/ChatInputBar'
import { ChatModals } from './mui-chat/components/ChatModals'

/**
 * IT業界キャリアエージェントのチャット UI。
 * 状態・副作用は useMuiChat、表示は各コンポーネントに委譲する。
 */
export function MuiChat() {
  const chat = useMuiChat()

  if (!chat.mounted) {
    return (
      <PageLoading fullScreen={false} showBrand={false} message="チャットを準備しています..." />
    )
  }

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
          input={chat.input}
          inputPlaceholder={chat.inputPlaceholder}
          isLoading={chat.isLoading}
          historyLoadError={chat.historyLoadError}
          inputRef={chat.inputRef}
          onInputChange={chat.setInput}
          onSend={chat.handleSend}
          onOtherChoice={chat.handleOtherChoice}
          onShowCompletionModal={() => chat.setShowCompletionModal(true)}
        />
      </Box>
    </>
  )
}
