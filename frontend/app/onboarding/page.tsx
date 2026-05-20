'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Stack,
  Step,
  StepLabel,
  Stepper,
  Typography,
} from '@mui/material'
import ChatIcon from '@mui/icons-material/Chat'
import BusinessIcon from '@mui/icons-material/Business'
import MicIcon from '@mui/icons-material/Mic'
import ArrowForwardIcon from '@mui/icons-material/ArrowForward'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'

const STEPS = [
  {
    icon: <ChatIcon sx={{ fontSize: 32, color: '#ec5b13' }} />,
    title: '自己分析チャット',
    description: 'AIとの会話を通じて、あなたの強み・志向・経験を整理します。まずここから始めましょう。',
    action: 'チャットを始める',
    path: '/',
    tag: '最初にやること',
  },
  {
    icon: <BusinessIcon sx={{ fontSize: 32, color: '#1976d2' }} />,
    title: '企業マッチング',
    description: '自己分析が完了すると、あなたに合った企業を自動でリストアップします。',
    action: 'マッチング結果を見る',
    path: '/results',
    tag: '自己分析後',
  },
  {
    icon: <MicIcon sx={{ fontSize: 32, color: '#388e3c' }} />,
    title: '面接練習',
    description: 'マッチングした企業を想定したAI面接練習で、実践力を高めましょう。',
    action: '面接練習を始める',
    path: '/interview',
    tag: 'マッチング後',
  },
]

export default function OnboardingPage() {
  const router = useRouter()
  const [activeStep, setActiveStep] = useState(0)
  const [completedSteps, setCompletedSteps] = useState<boolean[]>([false, false, false])

  useEffect(() => {
    const hasChatSession = !!(localStorage.getItem('chat_session_id') || localStorage.getItem('currentSessionId'))
    const hasResults = hasChatSession // マッチング結果はチャット完了後に閲覧可能
    const hasInterview = !!localStorage.getItem('interview_session_id')
    const completed = [hasChatSession, hasResults, hasInterview]
    setCompletedSteps(completed)
    // 最初の未完了ステップをアクティブに設定
    const nextStep = completed.findIndex((c) => !c)
    setActiveStep(nextStep === -1 ? STEPS.length - 1 : nextStep)
  }, [])

  const handleStart = (path: string) => {
    localStorage.setItem('onboarding_completed', 'true')
    router.push(path)
  }

  return (
    <Box
      sx={{
        minHeight: '100vh',
        bgcolor: '#f5f5f5',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        px: 2,
        py: 6,
      }}
    >
      <Box sx={{ maxWidth: 720, width: '100%' }}>
        {/* ウェルカムメッセージ */}
        <Box sx={{ textAlign: 'center', mb: 5 }}>
          <Typography variant="h4" sx={{ fontWeight: 700, mb: 1, fontSize: { xs: '1.4rem', sm: '2.125rem' } }}>
            ようこそ！まずはここから始めましょう
          </Typography>
          <Typography sx={{ color: 'text.secondary', fontSize: 16 }}>
            3つのステップで就活を効率的に進められます。まず自己分析チャットから始めてください。
          </Typography>
        </Box>

        {/* ステッパー */}
        <Stepper activeStep={activeStep} alternativeLabel sx={{ mb: 4 }}>
          {STEPS.map((step) => (
            <Step key={step.title}>
              <StepLabel>{step.title}</StepLabel>
            </Step>
          ))}
        </Stepper>

        {/* ステップカード */}
        <Stack spacing={2} sx={{ mb: 5 }}>
          {STEPS.map((step, idx) => {
            const isActive = idx === activeStep
            const isDone = completedSteps[idx]
            return (
              <Card
                key={step.title}
                variant="outlined"
                sx={{
                  border: isDone ? '2px solid #388e3c' : isActive ? '2px solid #ec5b13' : '1px solid #e0e0e0',
                  bgcolor: isDone ? '#f1f8f1' : isActive ? '#fff8f5' : '#fff',
                }}
              >
                <CardContent sx={{ display: 'flex', alignItems: 'flex-start', gap: 2, py: 2.5, flexWrap: { xs: 'wrap', sm: 'nowrap' } }}>
                  <Box sx={{ mt: 0.5, flexShrink: 0 }}>
                    {isDone ? <CheckCircleIcon sx={{ fontSize: 32, color: '#388e3c' }} /> : step.icon}
                  </Box>
                  <Box sx={{ flex: 1 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                      <Typography sx={{ fontWeight: 700, fontSize: 16 }}>{step.title}</Typography>
                      {isDone ? (
                        <Chip label="完了" size="small" color="success" sx={{ fontSize: 11, height: 20 }} />
                      ) : (
                        <Chip label={step.tag} size="small" sx={{ fontSize: 11, height: 20 }} />
                      )}
                    </Box>
                    <Typography sx={{ color: 'text.secondary', fontSize: 14 }}>{step.description}</Typography>
                  </Box>
                  <Button
                    variant={isDone ? 'outlined' : isActive ? 'contained' : 'outlined'}
                    endIcon={<ArrowForwardIcon />}
                    onClick={() => handleStart(step.path)}
                    sx={{
                      flexShrink: 0,
                      width: { xs: '100%', sm: 'auto' },
                      ...(isActive && !isDone
                        ? { bgcolor: '#ec5b13', '&:hover': { bgcolor: '#c44d0e' }, color: '#fff' }
                        : {}),
                      textTransform: 'none',
                      borderRadius: 9999,
                    }}
                  >
                    {isDone ? 'もう一度' : step.action}
                  </Button>
                </CardContent>
              </Card>
            )
          })}
        </Stack>

        <Box sx={{ textAlign: 'center' }}>
          <Button variant="text" onClick={() => handleStart('/')} sx={{ color: 'text.secondary', textTransform: 'none' }}>
            スキップしてチャット画面へ
          </Button>
        </Box>
      </Box>
    </Box>
  )
}
