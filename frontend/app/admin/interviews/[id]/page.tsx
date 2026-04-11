'use client'

import { useEffect, useState } from 'react'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'

type InterviewVideo = {
  id: number
  session_id: number
  user_id: number
  drive_file_id: string
  drive_file_url: string
  file_name: string
  file_size_bytes: number
  mime_type: string
  status: string
  error_message?: string
  uploaded_at?: string
  created_at: string
}

const VIDEO_STATUS: Record<string, { label: string; color: 'default' | 'primary' | 'warning' | 'success' | 'error' }> = {
  pending: { label: '待機中', color: 'default' },
  uploading: { label: 'アップロード中', color: 'primary' },
  done: { label: '完了', color: 'success' },
  error: { label: 'エラー', color: 'error' },
}

export default function AdminInterviewDetailPage() {
  const params = useParams()
  const sessionId = params.id as string

  const [videos, setVideos] = useState<InterviewVideo[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [playingURL, setPlayingURL] = useState<string | null>(null)
  const [urlLoading, setUrlLoading] = useState<number | null>(null)
  const [urlError, setUrlError] = useState('')

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  useEffect(() => {
    const fetchVideos = async () => {
      setLoading(true)
      setError('')
      const response = await fetch(`/api/admin/interviews/${sessionId}/videos`, {
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await response.json()
      setLoading(false)
      if (!response.ok) {
        setError(data?.error || '動画一覧の取得に失敗しました')
        return
      }
      setVideos(data?.videos || [])
    }
    fetchVideos()
  }, [sessionId])

  const handlePlayVideo = async (video: InterviewVideo) => {
    if (video.status !== 'done') return
    setUrlLoading(video.id)
    setUrlError('')
    setPlayingURL(null)
    const response = await fetch(`/api/admin/interviews/${sessionId}/videos/${video.id}/url`, {
      headers: authService.getAdminFetchHeaders(),
    })
    const data = await response.json()
    setUrlLoading(null)
    if (!response.ok) {
      setUrlError(data?.error || 'URLの取得に失敗しました')
      return
    }
    setPlayingURL(data.url)
  }

  const formatBytes = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  }

  return (
    <Box sx={{ p: 4, maxWidth: 1100, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" spacing={2} sx={{ mb: 1 }}>
        <Button variant="text" component={Link} href="/admin/interviews" size="small">
          ← 一覧へ戻る
        </Button>
      </Stack>
      <Typography variant="h4" fontWeight="bold" gutterBottom>
        面接セッション #{sessionId} — 動画一覧
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}
      {urlError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {urlError}
        </Alert>
      )}

      {playingURL && (
        <Card sx={{ mb: 3, border: '1px solid', borderColor: 'primary.main' }}>
          <CardContent>
            <Typography variant="h6" gutterBottom>
              動画プレイヤー
            </Typography>
            <Box
              component="video"
              src={playingURL}
              controls
              sx={{ width: '100%', maxHeight: 480, borderRadius: 1, bgcolor: 'black' }}
            />
            <Button
              size="small"
              variant="text"
              sx={{ mt: 1 }}
              onClick={() => setPlayingURL(null)}
            >
              閉じる
            </Button>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            録画動画
          </Typography>
          <Divider sx={{ mb: 2 }} />
          {loading ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
              <CircularProgress />
            </Box>
          ) : videos.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              このセッションに動画はありません。
            </Typography>
          ) : (
            <TableContainer>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>ID</TableCell>
                    <TableCell>ファイル名</TableCell>
                    <TableCell>サイズ</TableCell>
                    <TableCell>ステータス</TableCell>
                    <TableCell>アップロード日時</TableCell>
                    <TableCell align="right">操作</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {videos.map((video) => {
                    const st = VIDEO_STATUS[video.status] ?? { label: video.status, color: 'default' as const }
                    return (
                      <TableRow key={video.id}>
                        <TableCell>{video.id}</TableCell>
                        <TableCell>
                          <Typography variant="body2" sx={{ wordBreak: 'break-all' }}>
                            {video.file_name}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            {video.mime_type}
                          </Typography>
                        </TableCell>
                        <TableCell>{formatBytes(video.file_size_bytes)}</TableCell>
                        <TableCell>
                          <Chip label={st.label} color={st.color} size="small" />
                          {video.error_message && (
                            <Typography variant="caption" color="error" display="block">
                              {video.error_message}
                            </Typography>
                          )}
                        </TableCell>
                        <TableCell>
                          {video.uploaded_at
                            ? new Date(video.uploaded_at).toLocaleString('ja-JP')
                            : '—'}
                        </TableCell>
                        <TableCell align="right">
                          <Stack direction="row" spacing={1} justifyContent="flex-end">
                            <Button
                              size="small"
                              variant="contained"
                              disabled={video.status !== 'done' || urlLoading === video.id}
                              onClick={() => handlePlayVideo(video)}
                            >
                              {urlLoading === video.id ? (
                                <CircularProgress size={16} />
                              ) : (
                                '再生'
                              )}
                            </Button>
                            {video.drive_file_url && video.status === 'done' && (
                              <Button
                                size="small"
                                variant="outlined"
                                component="a"
                                href={video.drive_file_url}
                                target="_blank"
                                rel="noopener noreferrer"
                              >
                                DL
                              </Button>
                            )}
                          </Stack>
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </CardContent>
      </Card>
    </Box>
  )
}
