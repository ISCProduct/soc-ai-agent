"use client"

import { useState, useRef, useEffect } from "react"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Progress } from "@/components/ui/progress"
import { Send, Bot, User } from "lucide-react"
import { CompanyResults } from "@/components/company-results"
import { AnalysisLoading } from "@/components/analysis-loading"
import { sendChatMessage, getChatHistory, getUserScores, type ChatResponse, type ChatScore } from "@/lib/api"

type Message = {
  id: string
  role: "agent" | "user"
  content: string
}

type UserScore = {
  category: string
  score: number
  reason: string
}

type ChoiceOption = {
  number: number
  text: string
}

// メッセージから選択肢を抽出する関数
function extractChoices(content: string): { choices: ChoiceOption[], mainText: string } {
  const lines = content.split('\n')
  const choices: ChoiceOption[] = []
  const mainTextLines: string[] = []
  
  for (const line of lines) {
    // "1. 〜" "2. 〜" のようなパターンを検出
    const match = line.match(/^(\d+)\.\s*(.+)$/)
    if (match) {
      const number = parseInt(match[1])
      const text = match[2].trim()
      choices.push({ number, text })
    } else if (line.trim()) {
      mainTextLines.push(line)
    }
  }
  
  return {
    choices: choices.length >= 3 ? choices : [], // 3つ以上の選択肢がある場合のみ
    mainText: mainTextLines.join('\n')
  }
}

export function JobAgentChat() {
  // セッションIDを最初に初期化（他のstateより先に）
  const [sessionId, setSessionId] = useState<string>(() => {
    if (typeof window !== 'undefined') {
      let storedSessionId = localStorage.getItem('chat_session_id')
      if (!storedSessionId) {
        storedSessionId = `session_${Date.now()}_${Math.random().toString(36).slice(2, 11)}`
        localStorage.setItem('chat_session_id', storedSessionId)
      }
      console.log('[Frontend] Session ID initialized:', storedSessionId)
      return storedSessionId
    }
    return ''
  })
  
  const [userId] = useState(1)
  const [industryId] = useState(1)
  const [jobCategoryId] = useState(0)
  const [isLoadingFromBackend, setIsLoadingFromBackend] = useState(false)
  const [isInitializing, setIsInitializing] = useState(true)

  const [messages, setMessages] = useState<Message[]>(() => {
    // 初期表示用にLocalStorageからキャッシュを読み込む（バックエンドから取得するまでの一時表示）
    if (typeof window !== 'undefined' && sessionId) {
      const cached = localStorage.getItem(`chat_cache_${sessionId}`)
      if (cached) {
        try {
          console.log('[Frontend] Loading cached messages for session:', sessionId)
          return JSON.parse(cached)
        } catch (e) {
          console.error('Failed to parse cached messages:', e)
        }
      }
    }
    return []
  })
  const [inputValue, setInputValue] = useState("")
  const [isComplete, setIsComplete] = useState(false)
  const [isAnalyzing, setIsAnalyzing] = useState(false)
  const [isTyping, setIsTyping] = useState(false)
  const [userScores, setUserScores] = useState<UserScore[]>([])
  const [progress, setProgress] = useState({ questions: 0, total: 15, categories: 0, totalCategories: 10 })
  const [showCustomInput, setShowCustomInput] = useState(false) // カスタム入力モード
  const messagesEndRef = useRef<HTMLDivElement>(null)

  // メッセージが更新されたらキャッシュに保存
  useEffect(() => {
    if (typeof window !== 'undefined' && messages.length > 0) {
      localStorage.setItem(`chat_cache_${sessionId}`, JSON.stringify(messages))
    }
  }, [messages, sessionId])

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }

  useEffect(() => {
    scrollToBottom()
  }, [messages, isTyping])

  // 初回ロード時にバックエンドからチャット履歴を読み込む
  useEffect(() => {
    if (sessionId) {
      initializeChat()
    }
  }, [sessionId])

  const initializeChat = async () => {
    if (!sessionId) {
      console.log('[Frontend] Session ID not ready yet')
      return
    }
    
    try {
      console.log('[Frontend] Initializing chat with sessionId:', sessionId)
      
      // バックエンドからチャット履歴を取得
      const history = await getChatHistory(sessionId)
      console.log('[Frontend] Chat history loaded:', history.length, 'messages')
      
      if (history.length > 0) {
        // 既存の履歴がある場合は復元
        const loadedMessages: Message[] = history.map((msg) => ({
          id: msg.id.toString(),
          role: msg.role === "assistant" ? "agent" : "user",
          content: msg.content,
        }))
        setMessages(loadedMessages)
        console.log('[Frontend] Messages restored from backend')
        
        // バックエンドからスコアと進捗を取得
        try {
          const scoresData = await getUserScores(userId, sessionId)
          console.log('[Frontend] Scores loaded:', scoresData)
          if (scoresData && scoresData.length > 0) {
            const scores: UserScore[] = scoresData.map((s: ChatScore) => ({
              category: s.weight_category || s.category,
              score: s.score || 0,
              reason: s.reason || ''
            }))
            setUserScores(scores)
            
            // 進捗状況を計算（スコアの数から）
            setProgress({
              questions: history.length,
              total: 15,
              categories: scores.length,
              totalCategories: 10,
            })
          }
        } catch (error) {
          console.error("Failed to load scores:", error)
        }
        
        // 完了状態をチェック（診断完了の特別なメッセージを確認）
        const lastMessage = history[history.length - 1]
        if (lastMessage.role === "assistant" && 
            (lastMessage.content.includes("分析が完了しました") || 
             lastMessage.content.includes("診断が完了しました"))) {
          setIsComplete(true)
        }
      } else {
        console.log('[Frontend] No history found, starting new session')
        // 新規セッション：AIに最初の質問を生成させる
        setIsTyping(true)
        const response = await sendChatMessage({
          user_id: userId,
          session_id: sessionId,
          message: "START_SESSION",
          industry_id: industryId,
          job_category_id: jobCategoryId,
        })
        
        setMessages([
          {
            id: "1",
            role: "agent",
            content: response.response,
          },
        ])
        setIsTyping(false)
      }
    } catch (error) {
      console.error("Failed to initialize chat:", error)
      // エラー時はデフォルトメッセージ
      setMessages([
        {
          id: "1",
          role: "agent",
          content: "こんにちは！IT業界専門のキャリアエージェントです。あなたに最適な企業を見つけるため、いくつか質問させてください。まず、どのような職種に興味がありますか？",
        },
      ])
    } finally {
      setIsInitializing(false)
    }
  }

  const handleSend = async (message?: string) => {
    const textToSend = message || inputValue.trim()
    if (!textToSend || !sessionId) return

    // ユーザーメッセージを追加
    const newUserMessage = {
      id: Date.now().toString(),
      role: "user" as const,
      content: textToSend,
    }
    
    setMessages((prev) => [...prev, newUserMessage])
    setInputValue("")

    // バックエンドに送信
    setIsLoadingFromBackend(true)
    setIsTyping(true)

    try {
      console.log("[Frontend] Sending message to backend:", { sessionId, userId, message: textToSend })

      // 過去のチャット履歴を準備（最新のユーザーメッセージを含む）
      const chatHistory = [...messages, newUserMessage].map(msg => ({
        role: msg.role === "agent" ? "assistant" as const : "user" as const,
        content: msg.content
      }))

      const response: ChatResponse = await sendChatMessage({
        user_id: userId,
        session_id: sessionId,
        message: textToSend,
        industry_id: industryId,
        job_category_id: jobCategoryId,
        chat_history: chatHistory,
      })

      console.log("[Frontend] Received response from backend:", response)

      // AIの応答を追加
      setTimeout(() => {
        setMessages((prev) => [
          ...prev,
          {
            id: Date.now().toString(),
            role: "agent",
            content: response.response,
          },
        ])
        setIsTyping(false)
        setIsLoadingFromBackend(false)

        // 進捗状況を更新
        setProgress({
          questions: response.answered_questions || 0,
          total: response.total_questions || 20,
          categories: response.evaluated_categories || 0,
          totalCategories: response.total_categories || 10,
        })

        // 質問終了判定
        if (response.is_complete) {
          console.log("[Frontend] Analysis complete. Starting loading phase...")
          setTimeout(() => {
            setIsAnalyzing(true)
          }, 1000)
        }
      }, 500)

      if (response.current_scores && response.current_scores.length > 0) {
        console.log("[Frontend] Current scores:", response.current_scores)
        // スコアを保存
        const scores: UserScore[] = response.current_scores.map((s) => ({
          category: s.weight_category || s.category,
          score: s.score || 0,
          reason: s.reason || ''
        }))
        setUserScores(scores)
      }
    } catch (error) {
      console.error("[Frontend] Backend error:", error)
      setIsTyping(false)
      setIsLoadingFromBackend(false)

      setMessages((prev) => [
        ...prev,
        {
          id: Date.now().toString(),
          role: "agent",
          content: "申し訳ございません。接続エラーが発生しました。もう一度お試しください。",
        },
      ])
    }
  }

  const handleReset = () => {
    // 古いキャッシュをクリア
    if (typeof window !== 'undefined') {
      localStorage.removeItem(`chat_cache_${sessionId}`)
    }
    
    // 新しいセッションID生成
    const newSessionId = `session_${Date.now()}_${Math.random().toString(36).slice(2, 11)}`
    if (typeof window !== 'undefined') {
      localStorage.setItem('chat_session_id', newSessionId)
    }
    
    // ページをリロード（バックエンドには古いセッションが残る）
    window.location.reload()
  }

  const handleEndChat = () => {
    // セッションとキャッシュを完全にクリア
    if (typeof window !== 'undefined') {
      localStorage.removeItem(`chat_cache_${sessionId}`)
      localStorage.removeItem('chat_session_id')
    }
    
    // ページをリロードして新しいセッションを開始
    window.location.reload()
  }

  const handleAnalysisComplete = () => {
    setIsAnalyzing(false)
    setIsComplete(true)
  }

  if (isAnalyzing) {
    return <AnalysisLoading onCompleteAction={handleAnalysisComplete} />
  }

  if (isComplete) {
    return <CompanyResults userData={{ scores: userScores }} onResetAction={handleReset} />
  }

  // 最後のメッセージに選択肢があるかチェック
  const lastMessage = messages.length > 0 ? messages[messages.length - 1] : null
  const hasChoicesInLastMessage = lastMessage && lastMessage.role === "agent" 
    ? extractChoices(lastMessage.content).choices.length > 0 
    : false

  return (
      <div className="flex justify-center items-center h-screen bg-background p-4">
        <Card className="flex flex-col w-full max-w-4xl h-[90vh] border-2">
          <div className="border-b bg-muted/50 p-4">
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-3">
                <Avatar className="w-10 h-10 bg-primary">
                  <AvatarFallback>
                    <Bot className="w-5 h-5 text-primary-foreground" />
                  </AvatarFallback>
                </Avatar>
                <div>
                  <h2 className="font-bold text-foreground">IT業界キャリアエージェント</h2>
                  <p className="text-xs text-muted-foreground">
                    AI駆動で最適な企業を選定
                  </p>
                </div>
              </div>
              
              <div className="flex items-center gap-3">
                {/* 進捗状況表示 */}
                {progress.categories > 0 && (
                  <div className="flex flex-col items-end gap-1.5 min-w-[200px]">
                    <div className="flex items-center gap-2">
                      <div className="text-sm font-semibold text-primary">
                        診断進行度
                      </div>
                      <div className="text-lg font-bold text-primary">
                        {Math.round((progress.categories / progress.totalCategories) * 100)}%
                      </div>
                    </div>
                    <div className="text-xs text-muted-foreground text-right">
                      {progress.categories}/{progress.totalCategories} カテゴリ評価済み
                    </div>
                    <Progress 
                      value={(progress.categories / progress.totalCategories) * 100} 
                      className="w-full h-2.5"
                    />
                  </div>
                )}
                
                {/* チャットを終了ボタン */}
                <Button 
                  variant="outline" 
                  size="sm"
                  onClick={handleEndChat}
                  className="text-sm"
                >
                  チャットを終了
                </Button>
              </div>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-6 space-y-4">
            {isInitializing ? (
              <div className="flex items-center justify-center h-full">
                <div className="text-center space-y-2">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto"></div>
                  <p className="text-sm text-muted-foreground">チャットを準備中...</p>
                </div>
              </div>
            ) : (
              <>
                {messages.map((message, index) => {
                  const isLastMessage = index === messages.length - 1
                  const isAgentMessage = message.role === "agent"
                  const { choices, mainText } = isAgentMessage ? extractChoices(message.content) : { choices: [], mainText: message.content }
                  
                  return (
                    <div key={message.id} className={`flex gap-3 ${message.role === "user" ? "flex-row-reverse" : "flex-row"}`}>
                      <Avatar className={`w-10 h-10 ${message.role === "agent" ? "bg-primary" : "bg-accent"} flex-shrink-0`}>
                        <AvatarFallback>
                          {message.role === "agent" ? (
                              <Bot className="w-5 h-5 text-primary-foreground" />
                          ) : (
                              <User className="w-5 h-5 text-accent-foreground" />
                          )}
                        </AvatarFallback>
                      </Avatar>
                      <div
                          className={`flex flex-col gap-3 max-w-[75%] ${message.role === "user" ? "items-end" : "items-start"}`}
                      >
                        <div
                            className={`rounded-2xl px-4 py-3 ${
                                message.role === "agent" ? "bg-muted text-foreground" : "bg-primary text-primary-foreground"
                            }`}
                        >
                          <p className="text-sm leading-relaxed whitespace-pre-line">
                            {choices.length > 0 ? mainText : message.content}
                          </p>
                        </div>
                        
                        {/* 選択肢ボタン（エージェントの最後のメッセージで、選択肢がある場合のみ表示） */}
                        {isAgentMessage && isLastMessage && choices.length > 0 && !isLoadingFromBackend && (
                          <div className="flex flex-col gap-2 w-full">
                            {choices.map((choice) => (
                              <Button
                                key={choice.number}
                                variant="outline"
                                className="justify-start text-left h-auto py-3 px-4 hover:bg-primary hover:text-primary-foreground transition-colors"
                                onClick={() => {
                                  handleSend(choice.number.toString())
                                  setShowCustomInput(false) // 選択肢クリック時はカスタム入力モードをリセット
                                }}
                              >
                                <span className="font-semibold mr-2">{choice.number}.</span>
                                <span>{choice.text}</span>
                              </Button>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  )
                })}
                {isTyping && (
                    <div className="flex gap-3">
                      <Avatar className="w-10 h-10 bg-primary flex-shrink-0">
                        <AvatarFallback>
                          <Bot className="w-5 h-5 text-primary-foreground" />
                        </AvatarFallback>
                      </Avatar>
                      <div className="bg-muted rounded-2xl px-4 py-3">
                        <div className="flex gap-1">
                          <div className="w-2 h-2 bg-foreground/40 rounded-full animate-bounce [animation-delay:-0.3s]"></div>
                          <div className="w-2 h-2 bg-foreground/40 rounded-full animate-bounce [animation-delay:-0.15s]"></div>
                          <div className="w-2 h-2 bg-foreground/40 rounded-full animate-bounce"></div>
                        </div>
                      </div>
                    </div>
                )}
                <div ref={messagesEndRef} />
              </>
            )}
          </div>

          <div className="border-t p-4">
            {hasChoicesInLastMessage && !showCustomInput ? (
              // 選択肢がある場合は「その他を入力」ボタンのみ表示
              <div className="flex justify-center">
                <Button
                  variant="outline"
                  onClick={() => setShowCustomInput(true)}
                  className="w-full sm:w-auto"
                >
                  その他を入力
                </Button>
              </div>
            ) : (
              // 通常の入力フォーム
              <form
                  onSubmit={(e) => {
                    e.preventDefault()
                    handleSend()
                    setShowCustomInput(false) // 送信後はリセット
                  }}
                  className="flex gap-2"
              >
                <Input
                    value={inputValue}
                    onChange={(e) => setInputValue(e.target.value)}
                    placeholder="メッセージを入力..."
                    className="flex-1"
                    disabled={isLoadingFromBackend || isInitializing}
                    autoFocus={showCustomInput} // カスタム入力モード時は自動フォーカス
                />
                <Button type="submit" size="icon" disabled={isLoadingFromBackend || !inputValue.trim() || isInitializing}>
                  <Send className="w-4 h-4" />
                </Button>
                {/* カスタム入力モード時はキャンセルボタンを表示 */}
                {showCustomInput && (
                  <Button 
                    type="button" 
                    variant="outline" 
                    onClick={() => {
                      setShowCustomInput(false)
                      setInputValue("")
                    }}
                  >
                    キャンセル
                  </Button>
                )}
              </form>
            )}
          </div>
        </Card>
      </div>
  )
}
