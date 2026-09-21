"use client"

import { useState } from "react"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Label } from "@/components/ui/label"
import { Progress } from "@/components/ui/progress"
import { CheckCircle2, ArrowRight, ArrowLeft, Briefcase, Heart, DollarSign, TrendingUp } from "lucide-react"
import { CompanyResults } from "@/components/company-results"

type Answer = {
  category: string
  value: string

}

const questions = [
  {
    id: "job-type",
    category: "職種",
    icon: Briefcase,
    question: "どのような職種に興味がありますか？",
    options: [
      { value: "engineer", label: "エンジニア・技術職" },
      { value: "sales", label: "営業・マーケティング" },
      { value: "planning", label: "企画・プロジェクトマネージャー" },
      { value: "creative", label: "クリエイティブ・デザイン" },
      { value: "management", label: "経営・管理職" },
    ],
  },
  {
    id: "interest",
    category: "興味",
    icon: Heart,
    question: "どの分野に最も興味がありますか？",
    options: [
      { value: "tech", label: "テクノロジー・IT" },
      { value: "finance", label: "金融・コンサルティング" },
      { value: "manufacturing", label: "製造・メーカー" },
      { value: "service", label: "サービス・小売" },
      { value: "healthcare", label: "医療・ヘルスケア" },
    ],
  },
  {
    id: "compensation",
    category: "待遇",
    icon: DollarSign,
    question: "重視する待遇は何ですか？",
    options: [
      { value: "salary", label: "高い給与" },
      { value: "balance", label: "ワークライフバランス" },
      { value: "benefits", label: "充実した福利厚生" },
      { value: "remote", label: "リモートワーク可能" },
      { value: "stability", label: "安定性・大企業" },
    ],
  },
  {
    id: "future",
    category: "将来",
    icon: TrendingUp,
    question: "将来のキャリアで重視することは？",
    options: [
      { value: "growth", label: "急成長企業での経験" },
      { value: "skill", label: "スキルアップ・専門性" },
      { value: "leadership", label: "リーダーシップ・マネジメント" },
      { value: "global", label: "グローバルな環境" },
      { value: "impact", label: "社会的インパクト" },
    ],
  },
]

export function JobMatchingWizard() {
  const [currentStep, setCurrentStep] = useState(0)
  const [answers, setAnswers] = useState<Answer[]>([])
  const [selectedValue, setSelectedValue] = useState("")
  const [isComplete, setIsComplete] = useState(false)

  const progress = ((currentStep + 1) / questions.length) * 100
  const currentQuestion = questions[currentStep]
  const Icon = currentQuestion.icon

  const handleNext = () => {
    if (selectedValue) {
      const newAnswers = [...answers]
      newAnswers[currentStep] = {
        category: currentQuestion.category,
        value: selectedValue,
      }
      setAnswers(newAnswers)

      if (currentStep < questions.length - 1) {
        setCurrentStep(currentStep + 1)
        setSelectedValue(newAnswers[currentStep + 1]?.value || "")
      } else {
        setIsComplete(true)
      }
    }
  }

  const handleBack = () => {
    if (currentStep > 0) {
      setCurrentStep(currentStep - 1)
      setSelectedValue(answers[currentStep - 1]?.value || "")
    }
  }

  const handleReset = () => {
    setCurrentStep(0)
    setAnswers([])
    setSelectedValue("")
    setIsComplete(false)
  }

  if (isComplete) {
    // @ts-ignore
    return < CompanyResults answers={answers} onResetAction={handleReset} />
  }

  return (
    <div className="max-w-3xl mx-auto">
      <div className="mb-8">
        <div className="flex items-center justify-between mb-2">
          <span className="text-sm font-medium text-muted-foreground">
            質問 {currentStep + 1} / {questions.length}
          </span>
          <span className="text-sm font-medium text-primary">{Math.round(progress)}%</span>
        </div>
        <Progress value={progress} className="h-2" />
      </div>

      <Card className="border-2">
        <CardHeader>
          <div className="flex items-center gap-3 mb-2">
            <div className="p-2 rounded-lg bg-primary/10">
              <Icon className="w-6 h-6 text-primary" />
            </div>
            <span className="text-sm font-semibold text-primary uppercase tracking-wide">
              {currentQuestion.category}
            </span>
          </div>
          <CardTitle className="text-2xl text-balance">{currentQuestion.question}</CardTitle>
          <CardDescription>最も当てはまるものを選択してください</CardDescription>
        </CardHeader>
        <CardContent>
          <RadioGroup value={selectedValue} onValueChange={setSelectedValue} className="space-y-3">
            {currentQuestion.options.map((option) => (
              <div
                key={option.value}
                className={`flex items-center space-x-3 p-4 rounded-lg border-2 transition-all cursor-pointer hover:border-primary/50 ${
                  selectedValue === option.value ? "border-primary bg-primary/5" : "border-border bg-card"
                }`}
                onClick={() => setSelectedValue(option.value)}
              >
                <RadioGroupItem value={option.value} id={option.value} />
                <Label htmlFor={option.value} className="flex-1 cursor-pointer font-medium text-card-foreground">
                  {option.label}
                </Label>
                {selectedValue === option.value && <CheckCircle2 className="w-5 h-5 text-primary" />}
              </div>
            ))}
          </RadioGroup>

          <div className="flex gap-3 mt-8">
            <Button
              variant="outline"
              onClick={handleBack}
              disabled={currentStep === 0}
              className="flex-1 bg-transparent"
            >
              <ArrowLeft className="w-4 h-4 mr-2" />
              戻る
            </Button>
            <Button onClick={handleNext} disabled={!selectedValue} className="flex-1">
              {currentStep === questions.length - 1 ? "結果を見る" : "次へ"}
              <ArrowRight className="w-4 h-4 ml-2" />
            </Button>
          </div>

        </CardContent>
      </Card>
    </div>
  )
}
