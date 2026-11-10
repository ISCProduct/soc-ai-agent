'use client'

import { useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { useRequirePlatformAdmin } from '@/lib/admin/use-require-platform-admin'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'

export default function PageContent() {
  const platformReady = useRequirePlatformAdmin()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  if (!platformReady) return null

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
    <PageContainer
      maxWidth={ADMIN_PAGE_WIDTH.form}
      sx={{
        // #1766: 帯（管理シェルの AppBar）は md 未満で全幅。
        // 本文を sm から maxWidth 720 で中央寄せし、横余白も sm で増やすと、
        // 768px だけ見出しが帯より 32px 右へずれる。
        // 帯が消える md 以上でだけ 720px に収め、それ未満は帯と同じ横余白にする。
        maxWidth: { xs: '100%', md: ADMIN_PAGE_WIDTH.form },
        p: { xs: 2, sm: 2, md: 4 },
      }}
    >
      <AdminPageHeader
        title="集合知サマリー管理"
        description="ユーザー行動データをもとに全企業の集合知サマリーを再集計します。"
        backHref="/admin"
      />

      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      {success && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess('')}>{success}</Alert>}

      <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
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
    </PageContainer>
  )
}
