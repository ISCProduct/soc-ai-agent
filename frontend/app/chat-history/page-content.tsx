'use client'

import { useState, useEffect, useCallback } from 'react'
import { Box, Container, Typography, Paper, List, ListItem, ListItemButton, ListItemText, Divider, CircularProgress, Button, Stack } from '@mui/material'
import { useRouter } from 'next/navigation'
import { authService } from '@/lib/auth'
import ChatIcon from '@mui/icons-material/Chat'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { parseChatSessionsResponse, type ChatSessionSummary } from './parseSessions'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'

export default function PageContent() {
  const [sessions, setSessions] = useState<ChatSessionSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [authError, setAuthError] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const router = useRouter()

  const fetchSessions = useCallback(async () => {
    setLoading(true)
    setLoadError(null)
    setAuthError(false)
    try {
      const response = await fetch('/api/chat/sessions', {
        headers: authService.getUserFetchHeaders(),
      })
      if (response.status === 401 || response.status === 403) {
        setAuthError(true)
        setSessions([])
        return
      }
      if (!response.ok) {
        setLoadError('チャット履歴の取得に失敗しました。')
        setSessions([])
        return
      }
      const raw = await response.json()
      setSessions(parseChatSessionsResponse(raw))
    } catch {
      setLoadError('チャット履歴の取得に失敗しました。')
      setSessions([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user) {
      router.push('/login')
      return
    }
    void fetchSessions()
  }, [router, fetchSessions])

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    return date.toLocaleString('ja-JP', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      timeZone: 'Asia/Tokyo'
    })
  }

  const handleSessionClick = (sessionId: string) => {
    localStorage.setItem('currentSessionId', sessionId)
    router.push('/')
  }

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <CircularProgress />
      </Box>
    )
  }

  if (authError) {
    return (
      <Container maxWidth="md" sx={{ py: 4 }}>
        <Paper sx={{ p: 4, textAlign: 'center' }}>
          <Typography variant="h6" color="error" gutterBottom>
            認証エラー
          </Typography>
          <Typography color="text.secondary" sx={{ mb: 2 }}>
            セッションが期限切れです。再度ログインしてください。
          </Typography>
          <Button variant="contained" onClick={() => { authService.logout(); router.push('/login') }}>
            ログインページへ
          </Button>
        </Paper>
      </Container>
    )
  }

  return (
    <Container maxWidth="md" sx={{ py: 4 }}>
      <Button
        startIcon={<ArrowBackIcon />}
        onClick={() => router.push('/')}
        sx={{ mb: 2 }}
      >
        戻る
      </Button>

      <Typography variant="h4" gutterBottom>
        チャット履歴
      </Typography>

      {loadError ? (
        <Paper sx={{ p: 4, textAlign: 'center' }}>
          <Stack spacing={2} alignItems="center">
            <Typography color="error">{loadError}</Typography>
            <Typography variant="body2" color="text.secondary">
              空の履歴ではなく、読み込みエラーです。時間をおいて再試行してください。
            </Typography>
            <Button variant="contained" onClick={() => { void fetchSessions() }}>
              再試行
            </Button>
          </Stack>
        </Paper>
      ) : sessions.length === 0 ? (
        <Paper sx={{ p: 4, textAlign: 'center' }}>
          <Typography color="text.secondary">
            チャット履歴がありません
          </Typography>
        </Paper>
      ) : (
        <Paper>
          <List>
            {sessions.map((session, index) => (
              <Box key={session.session_id}>
                {index > 0 && <Divider />}
                <ListItem disablePadding>
                  <ListItemButton onClick={() => handleSessionClick(session.session_id)}>
                    <ChatIcon sx={{ mr: 2, color: 'primary.main' }} />
                    <ListItemText
                      primary={`セッション: ${session.session_id.substring(0, 8)}...`}
                      secondary={
                        <>
                          <Typography component="span" variant="body2" color="text.primary">
                            メッセージ数: {session.message_count}
                          </Typography>
                          <br />
                          開始: {formatDate(session.started_at)}
                          <br />
                          最終更新: {formatDate(session.last_message_at)}
                        </>
                      }
                    />
                  </ListItemButton>
                </ListItem>
              </Box>
            ))}
          </List>
        </Paper>
      )}
      <BottomNavSpacer />
    </Container>
  )
}
