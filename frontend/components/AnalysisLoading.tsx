"use client"

import { useCallback, useEffect, useState } from "react"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { CheckCircle2, Loader2, ArrowRight, AlertCircle } from "lucide-react"
import { authService } from "@/lib/auth"

type AnalysisLoadingProps = {
  onCompleteAction: () => void
}

const analysisSteps = [
  { id: 1, name: "回答データの集計", duration: 1000 },
  { id: 2, name: "適性スコアの算出", duration: 1500 },
  { id: 3, name: "企業データベースの検索", duration: 2000 },
  { id: 4, name: "マッチング分析", duration: 1500 },
  { id: 5, name: "最適企業の選定", duration: 1000 },
]

export function AnalysisLoading({ onCompleteAction }: AnalysisLoadingProps) {
  const [currentStep, setCurrentStep] = useState(0)
  const [completedSteps, setCompletedSteps] = useState<number[]>([])
  const [isFullyComplete, setIsFullyComplete] = useState(false)
  // ponytail: バックエンドの実処理成否を軽量GETで確認するだけの簡易チェック。結果データそのものはCompanyResults側で再取得する。
  const [apiState, setApiState] = useState<"pending" | "success" | "error">("pending")

  const checkAnalysisReady = useCallback(async () => {
    setApiState("pending")
    try {
      const sessionId = typeof window !== "undefined" ? localStorage.getItem("chat_session_id") : null
      if (!sessionId) {
        setApiState("error")
        return
      }
      const res = await fetch(`/api/chat/recommendations?session_id=${sessionId}&limit=1`, {
        headers: authService.getUserFetchHeaders(),
      })
      setApiState(res.ok ? "success" : "error")
    } catch {
      setApiState("error")
    }
  }, [])

  useEffect(() => {
    checkAnalysisReady()
  }, [checkAnalysisReady])

  useEffect(() => {
    if (currentStep >= analysisSteps.length) {
      if (apiState === "success") {
        setTimeout(() => {
          setIsFullyComplete(true)
        }, 500)
      }
      return
    }

    const step = analysisSteps[currentStep]
    const timer = setTimeout(() => {
      setCompletedSteps((prev) => [...prev, step.id])
      setCurrentStep((prev) => prev + 1)
    }, step.duration)

    return () => clearTimeout(timer)
  }, [currentStep, apiState])

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-background to-muted/20 p-4">
      <Card className="w-full max-w-2xl p-8 space-y-6">
        <div className="text-center space-y-2">
          <h2 className="text-3xl font-bold text-foreground">適性分析完了</h2>
          <p className="text-muted-foreground">
            あなたに最適な企業を選定しています...
          </p>
        </div>

        <div className="space-y-4">
          {analysisSteps.map((step, index) => {
            const isCompleted = completedSteps.includes(step.id)
            const isCurrent = currentStep === index
            const isPending = currentStep < index

            return (
              <div
                key={step.id}
                className={`flex items-center gap-4 p-4 rounded-lg transition-all duration-300 ${
                  isCurrent
                    ? "bg-primary/10 border-2 border-primary"
                    : isCompleted
                    ? "bg-muted/50"
                    : "bg-muted/20"
                }`}
              >
                <div className="flex-shrink-0">
                  {isCompleted ? (
                    <CheckCircle2 className="w-6 h-6 text-primary" />
                  ) : isCurrent ? (
                    <Loader2 className="w-6 h-6 text-primary animate-spin" />
                  ) : (
                    <div className="w-6 h-6 rounded-full border-2 border-muted-foreground/30" />
                  )}
                </div>

                <div className="flex-1">
                  <p
                    className={`font-medium ${
                      isCurrent
                        ? "text-primary"
                        : isCompleted
                        ? "text-foreground"
                        : "text-muted-foreground"
                    }`}
                  >
                    {step.name}
                  </p>
                </div>

                {isCurrent && (
                  <div className="flex-shrink-0">
                    <div className="text-xs text-primary font-medium">処理中...</div>
                  </div>
                )}

                {isCompleted && (
                  <div className="flex-shrink-0">
                    <div className="text-xs text-muted-foreground">完了</div>
                  </div>
                )}
              </div>
            )
          })}
        </div>

        <div className="space-y-2">
          <div className="flex justify-between text-sm text-muted-foreground">
            <span>進捗状況</span>
            <span>
              {completedSteps.length} / {analysisSteps.length}
            </span>
          </div>
          <div className="w-full bg-muted rounded-full h-2 overflow-hidden">
            <div
              className="bg-primary h-full transition-all duration-500 ease-out"
              style={{
                width: `${(completedSteps.length / analysisSteps.length) * 100}%`,
              }}
            />
          </div>
        </div>

        {currentStep >= analysisSteps.length && apiState === "error" && (
          <div className="pt-4 border-t">
            <div className="text-center space-y-4">
              <div className="flex items-center justify-center gap-2 text-destructive">
                <AlertCircle className="w-6 h-6" />
                <p className="text-lg font-semibold">分析結果の取得に失敗しました</p>
              </div>
              <Button size="lg" variant="outline" onClick={checkAnalysisReady} className="w-full">
                再試行する
              </Button>
            </div>
          </div>
        )}

        {isFullyComplete && (
          <div className="pt-4 border-t">
            <div className="text-center space-y-4">
              <div className="flex items-center justify-center gap-2 text-green-600">
                <CheckCircle2 className="w-6 h-6" />
                <p className="text-lg font-semibold">分析が完了しました！</p>
              </div>
              <Button
                size="lg"
                onClick={onCompleteAction}
                className="w-full"
              >
                結果を見る
                <ArrowRight className="w-4 h-4 ml-2" />
              </Button>
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
