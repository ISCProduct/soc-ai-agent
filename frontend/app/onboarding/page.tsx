'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  IconButton,
  Stack,
  Step,
  StepLabel,
  Stepper,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import ChatIcon from '@mui/icons-material/Chat'
import BusinessIcon from '@mui/icons-material/Business'
import MicIcon from '@mui/icons-material/Mic'
import ArrowForwardIcon from '@mui/icons-material/ArrowForward'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import { getResultsPathOrChat, getResultsSessionContext } from '@/lib/results-navigation'
import { authService } from '@/lib/auth'
import { resolveActiveStep, resolveOnboardingSteps, type OnboardingStepFlags } from './steps'

const STEPS = [
  {
    icon: <ChatIcon sx={{ fontSize: 32, color: 'primary.main' }} />,
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
  const [completedSteps, setCompletedSteps] = useState<OnboardingStepFlags>([false, false, false])

  useEffect(() => {
    let ignore = false
    const context = getResultsSessionContext()
    const hasChatSession = !!context
    const hasInterview = !!localStorage.getItem('interview_session_id')

    const applyCompleted = (hasMatchingResults: boolean) => {
      if (ignore) return
      const completed = resolveOnboardingSteps({ hasChatSession, hasMatchingResults, hasInterview })
      setCompletedSteps(completed)
      setActiveStep(resolveActiveStep(completed))
    }

    if (!context) {
      applyCompleted(false)
      return
    }

    // マッチング結果（recommendations）が1件以上あれば閲覧可能とみなす（#1015）
    fetch(`/api/chat/recommendations?session_id=${encodeURIComponent(context.sessionId)}&limit=1`, {
      headers: authService.getUserFetchHeaders(),
    })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => applyCompleted(Array.isArray(data?.recommendations) && data.recommendations.length > 0))
      .catch(() => applyCompleted(false))

    return () => {
      ignore = true
    }
  }, [])

  const handleStart = (path: string) => {
    localStorage.setItem('onboarding_completed', 'true')
    if (path === '/results') {
      router.push(getResultsPathOrChat())
      return
    }
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
        <Box sx={{ mb: 2 }}>
          <IconButton component={Link} href="/"><ArrowBackIcon /></IconButton>
        </Box>
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
                  border: '2px solid',
                  borderColor: isDone ? 'success.main' : isActive ? 'primary.main' : 'divider',
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
                        ? { bgcolor: 'primary.main', '&:hover': { bgcolor: 'primary.dark' }, color: 'primary.contrastText' }
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
