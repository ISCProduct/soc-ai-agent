'use client'

import { useState, useRef, useEffect, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import { sendChatMessage, getChatHistory, type ChatRequest, type ChatResponse } from '@/lib/api'
import { UserFacingApiError, gatewayErrorPath } from '@/lib/user-facing-error'
import { authService } from '@/lib/auth'
import { buildResultsPath, getResultsSessionContext } from '@/lib/results-navigation'
import { resolveChatOutgoingMessage } from '@/lib/chat-choices'
import {
  extractChoices,
  makeMessageId,
  INITIAL_GREETING,
  RESET_GREETING,
  clearChatSessionOnEnd,
  readStoredJobCategoryId,
  writeStoredJobCategoryId,
  computeProgressTotals,
  shouldAutoScrollToBottom,
  findLastAssistantQuestionMessage,
} from '../utils'
import type { Message, PhaseProgress, ProgressTotals, ChoiceOption } from '../types'

/**
 * MuiChat の状態・副作用・ハンドラを集約するフック。
 * UI はプレゼンテーショナルコンポーネント側へ委譲する。
 */
export function useMuiChat() {
  const router = useRouter()
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [analysisComplete, setAnalysisComplete] = useState(false)
  const [allPhasesCompleted, setAllPhasesCompleted] = useState(false)
  const [sessionId, setSessionId] = useState('')
  const [userId, setUserId] = useState<number>(0)
  const [questionCount, setQuestionCount] = useState(0)
  const [totalQuestions, setTotalQuestions] = useState(15)
  const [mounted, setMounted] = useState(false)
  const [showCompletionModal, setShowCompletionModal] = useState(false)
  const [showEndChatModal, setShowEndChatModal] = useState(false)
  const [showTerminationModal, setShowTerminationModal] = useState(false)
  const [otherChoiceActive, setOtherChoiceActive] = useState(false)
  const [phaseProgresses, setPhaseProgresses] = useState<PhaseProgress[] | null>(null)
  const [historyLoadError, setHistoryLoadError] = useState<string | null>(null)
  const [historyRetrying, setHistoryRetrying] = useState(false)
  const [jobCategoryId, setJobCategoryId] = useState(0)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const messagesAreaRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  const progressTotals: ProgressTotals = computeProgressTotals({
    phases: phaseProgresses,
    questionCount,
    totalQuestions,
  })

  const scrollToBottomIfNeeded = () => {
    const area = messagesAreaRef.current
    if (area) {
      const allow = shouldAutoScrollToBottom({
        scrollHeight: area.scrollHeight,
        scrollTop: area.scrollTop,
        clientHeight: area.clientHeight,
      })
      if (!allow) return
    }
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  useEffect(() => {
    scrollToBottomIfNeeded()
  }, [messages, isLoading])

  const applyHistoryOrGreeting = useCallback((history: Awaited<ReturnType<typeof getChatHistory>>) => {
    setHistoryLoadError(null)
    if (history && history.length > 0) {
      const restoredMessages: Message[] = history.map((msg) => ({
        id: String(msg.id),
        role: msg.role,
        content: msg.content,
        timestamp: new Date(msg.created_at),
      }))
      setMessages(restoredMessages)
      const userQuestionCount = history.filter((msg) => msg.role === 'user').length
      setQuestionCount(userQuestionCount)

      const savedTotalQuestions = sessionStorage.getItem('totalQuestions')
      const restoredTotalQuestions = savedTotalQuestions ? parseInt(savedTotalQuestions) : 15
      setTotalQuestions(restoredTotalQuestions)
      const savedPhases = sessionStorage.getItem('phaseProgress')
      const restoredPhases: PhaseProgress[] | null = savedPhases
        ? (JSON.parse(savedPhases) as PhaseProgress[])
        : null
      if (Array.isArray(restoredPhases)) {
        setPhaseProgresses(restoredPhases)
      }

      setTimeout(() => {
        window.dispatchEvent(
          new CustomEvent('chatProgress', {
            detail: {
              messageCount: restoredMessages.length,
              questionCount: userQuestionCount,
              totalQuestions: restoredTotalQuestions,
              phases: restoredPhases,
            },
          }),
        )
      }, 100)

      const lastMessage = history[history.length - 1]
      const isCompletionMessage =
        lastMessage?.content?.includes('分析が完了しました') ||
        lastMessage?.content?.includes('全てのフェーズが完了') ||
        lastMessage?.content?.includes('最適な企業をマッチング')

      if (isCompletionMessage) {
        setAnalysisComplete(true)
        setAllPhasesCompleted(true)
      }
      return
    }

    // 履歴 0 件（新規セッション）: 従来どおり挨拶表示
    setMessages([
      {
        id: '0',
        role: 'assistant',
        content: INITIAL_GREETING,
        timestamp: new Date(),
      },
    ])
  }, [])

  const loadChatHistory = useCallback(
    async (targetSessionId: string) => {
      const history = await getChatHistory(targetSessionId)
      applyHistoryOrGreeting(history)
    },
    [applyHistoryOrGreeting],
  )

  const handleRetryHistoryLoad = async () => {
    if (!sessionId || historyRetrying) return
    setHistoryRetrying(true)
    try {
      await loadChatHistory(sessionId)
    } catch (error) {
      console.error('[MUI Chat] Failed to load history (retry):', error)
      setHistoryLoadError(
        '履歴の読み込みに失敗しました。通信状況を確認して、もう一度お試しください。',
      )
      setMessages([])
    } finally {
      setHistoryRetrying(false)
    }
  }

  useEffect(() => {
    setMounted(true)

    const initializeChat = async () => {
      // ユーザー情報を初期化
      const user = authService.getStoredUser()
      if (!user) {
        router.replace('/login')
        return
      }
      const currentUserId = user.user_id
      setUserId(currentUserId)

      // セッションIDの取得優先順位:
      // 1. localStorageから（履歴ページから選択した場合）
      // 2. sessionStorageから（ページリロード時の復元）
      // 3. 新規生成
      let storedSessionId = localStorage.getItem('currentSessionId')
      if (storedSessionId) {
        console.log('[MUI Chat] Loading session from localStorage:', storedSessionId)
        // localStorageから読み込んだ後は削除
        localStorage.removeItem('currentSessionId')
      } else {
        storedSessionId = sessionStorage.getItem('chatSessionId')
      }

      if (!storedSessionId) {
        storedSessionId = `session_${Date.now()}_${Math.random().toString(36).substring(7)}`
        console.log('[MUI Chat] Created new session:', storedSessionId)
      }

      sessionStorage.setItem('chatSessionId', storedSessionId)
      setSessionId(storedSessionId)
      setJobCategoryId(readStoredJobCategoryId(storedSessionId))

      try {
        console.log('[MUI Chat] Loading history for session:', storedSessionId)
        await loadChatHistory(storedSessionId)
      } catch (error) {
        console.error('[MUI Chat] Failed to load history:', error)
        // #569: 失敗時は挨拶ではなくエラー UI（再試行で復元）
        setHistoryLoadError(
          '履歴の読み込みに失敗しました。通信状況を確認して、もう一度お試しください。',
        )
        setMessages([])
      }
    }

    void initializeChat()
    // マウント時のみ初期化（loadChatHistory / router は安定参照）
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleSend = async (overrideMessage?: string) => {
    const rawText = (overrideMessage ?? input).trim()
    if (!rawText || isLoading || !sessionId || !userId || historyLoadError) return

    // ボタン送信はそのまま。自由入力は直近の選択肢ラベルを記号へ正規化する
    const lastAssistant = findLastAssistantQuestionMessage(messages)
    const currentChoices = lastAssistant ? extractChoices(lastAssistant.content) : []
    const messageText =
      overrideMessage !== undefined
        ? rawText
        : resolveChatOutgoingMessage(rawText, currentChoices, otherChoiceActive)

    // 分析完了後はメッセージ送信を無効化
    if (analysisComplete) {
      console.log('[MUI Chat] Analysis already complete, ignoring message')
      return
    }

    const userMessage: Message = {
      id: makeMessageId(),
      role: 'user',
      content: messageText,
      timestamp: new Date(),
    }

    setMessages((prev) => [...prev, userMessage])
    setInput('')
    setOtherChoiceActive(false)
    setIsLoading(true)

    try {
      // バックエンドのAI機能を活用
      const chatRequest: ChatRequest = {
        user_id: userId,
        session_id: sessionId,
        message: messageText,
        industry_id: 1, // IT業界
        job_category_id: jobCategoryId,
      }

      const response: ChatResponse = await sendChatMessage(chatRequest)

      if (typeof response.job_category_id === 'number' && response.job_category_id > 0) {
        setJobCategoryId(response.job_category_id)
        writeStoredJobCategoryId(sessionId, response.job_category_id)
      }

      const assistantMessage: Message = {
        id: makeMessageId(),
        role: 'assistant',
        content: response.response || 'エラーが発生しました',
        timestamp: new Date(),
      }

      // バリデーションエラーかどうかをチェック
      const isValidationError =
        response.response?.includes('書かれた内容にはお答えできません') ||
        response.response?.includes('質問に回答してください') ||
        response.response?.includes('質問と関係のない内容が3回続いた')

      // セッション終了チェック
      const isTerminated =
        response.is_terminated === true ||
        response.response?.includes('チャットを終了させていただきます')

      setMessages((prev) => {
        const newMessages = [...prev, assistantMessage]

        // セッション終了の場合 - 専用モーダルを表示
        if (isTerminated) {
          console.log('[MUI Chat] Session terminated due to invalid answers')
          setAnalysisComplete(true)
          setShowTerminationModal(true) // 終了専用モーダル
          return newMessages
        }

        // バリデーションエラーの場合は質問カウントを進めない
        if (!isValidationError) {
          // 質問カウントの更新
          const newCount = response.answered_questions ?? questionCount + 1
          setQuestionCount(newCount)
          const newTotalQuestions = response.total_questions ?? 15
          setTotalQuestions(newTotalQuestions)

          // totalQuestionsをsessionStorageに保存
          sessionStorage.setItem('totalQuestions', String(newTotalQuestions))
          if (response.all_phases) {
            sessionStorage.setItem('phaseProgress', JSON.stringify(response.all_phases))
            setPhaseProgresses(response.all_phases)
          }

          // 進捗状況を親コンポーネントに通知（非同期で実行）
          setTimeout(() => {
            window.dispatchEvent(
              new CustomEvent('chatProgress', {
                detail: {
                  messageCount: newMessages.length,
                  questionCount: newCount,
                  totalQuestions: newTotalQuestions,
                  phases: response.all_phases ?? null,
                },
              }),
            )
          }, 0)

          // **重要: バックエンドのis_completeのみを信頼**
          console.log(
            '[MUI Chat] is_complete:',
            response.is_complete,
            'type:',
            typeof response.is_complete,
          )
          console.log(
            '[MUI Chat] evaluated_categories:',
            response.evaluated_categories,
            'total:',
            response.total_categories,
          )

          const allCompleted =
            response.all_phases?.every((phase) => {
              const required = phase.max_questions > 0 ? phase.max_questions : phase.min_questions
              return required > 0 && phase.valid_answers >= required
            }) ?? false

          const completionText =
            response.response?.includes('分析が完了しました') ||
            response.response?.includes('最適な企業をマッチング')
          if (response.is_complete === true && !completionText) {
            const completionMessage: Message = {
              id: makeMessageId(),
              role: 'assistant',
              content:
                '分析が完了しました！あなたに最適な企業をマッチングしました。「結果を見る」ボタンから詳細をご確認ください。',
              timestamp: new Date(),
            }
            newMessages.push(completionMessage)
          }

          if (response.is_complete === true) {
            console.log('[MUI Chat] AI分析完了 - モーダルを表示します')
            console.log('[MUI Chat] All phases completed:', allCompleted)
            setTimeout(() => {
              setAnalysisComplete(true)
              setAllPhasesCompleted(allCompleted)
              setShowCompletionModal(true)
            }, 300)
          } else {
            console.log(`[MUI Chat] 質問継続中 (${newCount}/${response.total_questions ?? 15})`)
            // 明示的にfalseを設定
            setAnalysisComplete(false)
            setAllPhasesCompleted(false)
          }
        } else {
          // バリデーションエラーの場合は質問カウントを進めないが、完了状態はリセット
          console.log('[MUI Chat] Validation error detected, not updating question count')
          // バリデーションエラー後も質問を継続できるように、完了状態を解除
          setAnalysisComplete(false)
          setAllPhasesCompleted(false)
        }

        return newMessages
      })
    } catch (error) {
      console.error('[MUI Chat] Backend error:', error)

      // "all phases completed"エラーの場合は分析完了として扱う
      const errorMessage = (error as Error).message
      if (errorMessage.includes('all phases completed')) {
        console.log('[MUI Chat] All phases completed - showing completion modal')
        setAnalysisComplete(true)
        setAllPhasesCompleted(true)
        setShowCompletionModal(true)

        // 完了メッセージを表示
        const completionMessage: Message = {
          id: makeMessageId(),
          role: 'assistant',
          content:
            '分析が完了しました！あなたに最適な企業をマッチングしました。「結果を見る」ボタンから詳細をご確認ください。',
          timestamp: new Date(),
        }
        setMessages((prev) => [...prev, completionMessage])
      } else if (error instanceof UserFacingApiError && error.gateway) {
        router.replace(gatewayErrorPath(error.status))
      } else {
        const errorMsg: Message = {
          id: makeMessageId(),
          role: 'assistant',
          content: '送信に失敗しました。しばらくしてから再試行してください。',
          timestamp: new Date(),
        }
        setMessages((prev) => [...prev, errorMsg])
      }
    } finally {
      setIsLoading(false)
    }
  }

  const handleReset = () => {
    // すべての状態をクリア
    setMessages([])
    setAnalysisComplete(false)
    setQuestionCount(0)
    setTotalQuestions(15)

    // セッションIDも新しく生成
    const newSessionId = `session_${Date.now()}_${Math.random().toString(36).substring(7)}`
    setSessionId(newSessionId)
    sessionStorage.setItem('chatSessionId', newSessionId)
    setJobCategoryId(0)
    writeStoredJobCategoryId(newSessionId, 0)

    // 初回メッセージを再設定
    const initialMessage: Message = {
      id: '0',
      role: 'assistant',
      content: RESET_GREETING,
      timestamp: new Date(),
    }
    setMessages([initialMessage])
    localStorage.setItem('chatMessages', JSON.stringify([initialMessage]))

    // 進捗状況を親コンポーネントに通知（非同期で実行）
    setTimeout(() => {
      window.dispatchEvent(
        new CustomEvent('chatProgress', {
          detail: { messageCount: 1, questionCount: 0, totalQuestions: 15 },
        }),
      )
    }, 0)
  }

  const handleEndChat = () => {
    setShowEndChatModal(true)
  }

  const handleConfirmEndChat = () => {
    clearChatSessionOnEnd({ sessionStorage, localStorage })

    // ページをリロードして新しいセッションを開始
    window.location.reload()
  }

  const handleCancelEndChat = () => {
    setShowEndChatModal(false)
  }

  const handleViewResults = () => {
    setShowCompletionModal(false)
    const context =
      userId && sessionId
        ? { userId: String(userId), sessionId }
        : getResultsSessionContext()
    if (context) {
      router.push(buildResultsPath(context))
      return
    }
    router.push('/')
  }

  const handleContinueChat = () => {
    console.log('[MUI Chat] Continuing chat after completion')
    console.log('[MUI Chat] Before reset - analysisComplete:', analysisComplete)
    setShowCompletionModal(false)
    setAnalysisComplete(false)
    console.log('[MUI Chat] After reset - modal closed, analysisComplete set to false')
    // 入力フィールドを有効化するためにフォーカス
    setTimeout(() => {
      inputRef.current?.focus()
    }, 100)
  }

  const lastAssistantMessage = findLastAssistantQuestionMessage(messages)
  const choiceOptions: ChoiceOption[] = lastAssistantMessage
    ? extractChoices(lastAssistantMessage.content)
    : []
  const showChoiceButtons = choiceOptions.length >= 2 && !analysisComplete
  const inputPlaceholder = otherChoiceActive
    ? 'その他の内容を入力...'
    : showChoiceButtons
      ? '選択肢と同じ内容を入力しても送信できます'
      : 'メッセージを入力...'

  useEffect(() => {
    if (!showChoiceButtons) {
      setOtherChoiceActive(false)
    }
  }, [showChoiceButtons])

  const handleOtherChoice = () => {
    setOtherChoiceActive(true)
    setInput('')
    setTimeout(() => inputRef.current?.focus(), 0)
  }

  return {
    mounted,
    messages,
    input,
    setInput,
    isLoading,
    analysisComplete,
    allPhasesCompleted,
    questionCount,
    totalQuestions,
    showCompletionModal,
    setShowCompletionModal,
    showEndChatModal,
    showTerminationModal,
    otherChoiceActive,
    historyLoadError,
    historyRetrying,
    messagesEndRef,
    messagesAreaRef,
    inputRef,
    progressTotals,
    choiceOptions,
    showChoiceButtons,
    inputPlaceholder,
    handleSend,
    handleReset,
    handleEndChat,
    handleConfirmEndChat,
    handleCancelEndChat,
    handleViewResults,
    handleContinueChat,
    handleRetryHistoryLoad,
    handleOtherChoice,
  }
}
