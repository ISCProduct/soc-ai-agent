'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  IconButton,
  Stack,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { authService } from '@/lib/auth'

export default function AdminCollectiveInsightsPage() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const handleRebuild = async () => {
    setLoading(true)
    setError('')
    setSuccess('')
    try {
      const headers = authService.getAdminFetchHeaders()
      const res = await fetch('/api/admin/collective-insights/rebuild-summaries', {
        method: 'POST',
        headers,
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || 'サマリー再構築に失敗しました')
        return
      }
      setSuccess('全企業の集合知サマリーを再構築しました')
    } catch {
      setError('通信エラーが発生しました')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Box sx={{ p: 4, maxWidth: 720, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
        <IconButton component={Link} href="/admin"><ArrowBackIcon /></IconButton>
        <Typography variant="h4" fontWeight="bold">
          集合知サマリー管理
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        ユーザー行動データをもとに全企業の集合知サマリーを再集計します。
      </Typography>

      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      {success && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess('')}>{success}</Alert>}

      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom>サマリー再構築</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            全企業の行動サマリーをバッチ処理で再集計します。データ整合性の確保やプロファイル更新後に実行してください。
            処理はバックグラウンドで実行されます。
          </Typography>
          <Button
            variant="contained"
            color="primary"
            onClick={handleRebuild}
            disabled={loading}
            startIcon={loading ? <CircularProgress size={16} /> : undefined}
          >
            {loading ? '再構築中...' : 'サマリーを再構築'}
          </Button>
        </CardContent>
      </Card>
    </Box>
  )
}
