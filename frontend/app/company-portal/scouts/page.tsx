'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Alert, Box, Button, Chip, Paper, Stack, Typography } from '@mui/material'
import { PageContainer } from '@/components/admin/PageContainer'
import { PageLoading } from '@/components/common/PageLoading'
import { companyAuthService } from '@/lib/company/auth'
import { SCOUT_STATUS_LABEL, companyScoutService, type CompanyScout } from '@/lib/company/scouts'

const PAGE_SIZE = 30

function formatWhen(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString('ja-JP')
}

export default function CompanyPortalScoutsPage() {
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [items, setItems] = useState<CompanyScout[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [error, setError] = useState('')
  const loadSeq = useRef(0)

  const load = useCallback(async (nextOffset: number) => {
    const seq = ++loadSeq.current
    try {
      const res = await companyScoutService.listScouts({ limit: PAGE_SIZE, offset: nextOffset })
      if (seq !== loadSeq.current) return
      setItems(res.items)
      setTotal(res.total)
      setError('')
    } catch (e) {
      if (seq !== loadSeq.current) return
      setError(e instanceof Error ? e.message : '送信履歴を取得できませんでした')
    }
  }, [])

  useEffect(() => {
    if (!companyAuthService.getStoredUser()) {
      router.replace('/company-portal/sign-in')
      return
    }
    load(0).finally(() => setLoading(false))
  }, [router, load])

  if (loading) return <PageLoading message="送信履歴を読み込んでいます..." />

  return (
    <PageContainer maxWidth={800}>
      <Stack direction="row" spacing={1} sx={{ mb: 2 }} flexWrap="wrap">
        <Button onClick={() => router.push('/company-portal')}>← ダッシュボードへ</Button>
        <Button variant="outlined" onClick={() => router.push('/company-portal/scout-templates')}>
          テンプレート管理
        </Button>
        <Button variant="outlined" onClick={() => router.push('/company-portal/students')}>
          学生を探す
        </Button>
      </Stack>

      <Typography variant="h4" fontWeight="bold" gutterBottom>
        スカウト送信履歴
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        自社から送ったスカウトの状態を確認できます。
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} action={<Button onClick={() => void load(offset)}>再読み込み</Button>}>
          {error}
        </Alert>
      )}

      {items.length === 0 && total === 0 ? (
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Typography fontWeight="bold" gutterBottom>
            まだ送信したスカウトはありません
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            学生詳細の「スカウトする」から送れます。
          </Typography>
          <Button variant="contained" onClick={() => router.push('/company-portal/students')}>
            学生を探す
          </Button>
        </Paper>
      ) : (
        <Stack spacing={1.5}>
          {items.map((s) => (
            <Paper key={s.id} variant="outlined" sx={{ p: 2 }}>
              <Stack direction="row" justifyContent="space-between" alignItems="flex-start" gap={1} flexWrap="wrap">
                <Typography fontWeight="bold">
                  {s.student_name ? `${s.student_name}さん` : `学生 #${s.user_id}`}
                </Typography>
                <Chip size="small" label={SCOUT_STATUS_LABEL[s.status]} />
              </Stack>
              <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
                {formatWhen(s.created_at)}
              </Typography>
              <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>
                {s.message}
              </Typography>
              <Button
                size="small"
                sx={{ mt: 1 }}
                onClick={() => router.push(`/company-portal/students/${s.user_id}`)}
              >
                学生詳細を見る
              </Button>
            </Paper>
          ))}
        </Stack>
      )}

      {total > PAGE_SIZE && (
        <Box sx={{ mt: 2, display: 'flex', gap: 1, alignItems: 'center' }}>
          <Button
            size="small"
            disabled={offset === 0}
            onClick={() => {
              const next = Math.max(offset - PAGE_SIZE, 0)
              setOffset(next)
              void load(next)
            }}
          >
            前へ
          </Button>
          <Typography variant="body2" color="text.secondary">
            {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} / {total}件
          </Typography>
          <Button
            size="small"
            disabled={offset + PAGE_SIZE >= total}
            onClick={() => {
              const next = offset + PAGE_SIZE
              setOffset(next)
              void load(next)
            }}
          >
            次へ
          </Button>
        </Box>
      )}
    </PageContainer>
  )
}
