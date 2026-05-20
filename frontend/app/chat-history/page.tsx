'use client'

import { useState, useEffect } from 'react'
import { Box, Container, Typography, Paper, List, ListItem, ListItemButton, ListItemText, Divider, CircularProgress, Button } from '@mui/material'
import { useRouter } from 'next/navigation'
import { authService } from '@/lib/auth'
import ChatIcon from '@mui/icons-material/Chat'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'

interface ChatSession {
  session_id: string
  user_id: number
  started_at: string
  last_message_at: string
  message_count: number
}

export default function ChatHistoryPage() {
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [loading, setLoading] = useState(true)
  const router = useRouter()

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user) {
      router.push('/')
      return
    }
    fetchSessions(user.user_id)
  }, [router])

  const fetchSessions = async (userId: number) => {
    try {
      const response = await fetch('/api/chat/sessions', {
        headers: authService.getUserFetchHeaders(),
      })
      if (!response.ok) {
        throw new Error('Failed to fetch sessions')
      }
      const data = await response.json()
      setSessions(data || [])
    } catch (error) {
      console.error('Error fetching sessions:', error)
      setSessions([])
    } finally {
      setLoading(false)
    }
  }

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

      {sessions.length === 0 ? (
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
    </Container>
  )
}
