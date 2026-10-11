'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material'
import { PageContainer } from '@/components/admin/PageContainer'
import { PageLoading } from '@/components/common/PageLoading'
import { MarkLabel } from '@/components/company-portal/MarkLabel'
import { PipelineBand, pipelineTotal } from '@/components/company-portal/PipelineBand'
import { companyAuthService } from '@/lib/company/auth'
import {
  ALLOWED_TRANSITIONS,
  companyApplicationService,
  statusLabel,
  type ApplicationListItem,
} from '@/lib/company/applications'
import { applicationMark } from '@/lib/company/marks'

const PAGE_SIZE = 30

function formatDate(value?: string): string {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleDateString('ja-JP')
}

export default function CompanyPortalApplicationsPage() {
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [isOwner, setIsOwner] = useState(false)
  const [items, setItems] = useState<ApplicationListItem[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [status, setStatus] = useState('')
  const [counts, setCounts] = useState<Record<string, number>>({})
  const [error, setError] = useState('')
  const [updatingId, setUpdatingId] = useState<number | null>(null)

  const loadCounts = useCallback(async () => {
    try {
      const dashboard = await companyApplicationService.fetchDashboard()
      setCounts(dashboard.status_counts ?? {})
    } catch {
      // 帯が取れなくても一覧は使える。
    }
  }, [])

  const load = useCallback(async (nextOffset: number, nextStatus: string) => {
    try {
      const res = await companyApplicationService.list({
        status: nextStatus || undefined,
        limit: PAGE_SIZE,
        offset: nextOffset,
      })
      setItems(res.applications ?? [])
      setTotal(res.total)
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : '応募者を取得できませんでした')
    }
  }, [])

  useEffect(() => {
    const stored = companyAuthService.getStoredUser()
    if (!stored) {
      router.replace('/company-portal/sign-in')
      return
    }
    const initial = new URLSearchParams(window.location.search).get('status') ?? ''
    setStatus(initial)
    companyAuthService
      .fetchMe()
      .then((me) => {
        // 選考ステータスの変更は owner のみ。サーバー側でも弾くが、
        // 操作できないものを見せない。
        setIsOwner(me.role === 'owner')
        return Promise.all([load(0, initial), loadCounts()])
      })
      .catch(() => {
        companyAuthService.logout()
        router.replace('/company-portal/sign-in')
      })
      .finally(() => setLoading(false))
  }, [router, load, loadCounts])

  const onChangeStatus = async (id: number, next: string) => {
    setUpdatingId(id)
    try {
      await companyApplicationService.updateStatus(id, next)
      await Promise.all([load(offset, status), loadCounts()])
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : '更新できませんでした')
    } finally {
      setUpdatingId(null)
    }
  }

  if (loading) {
    return <PageLoading message="応募者を読み込んでいます..." />
  }

  return (
    <PageContainer maxWidth={1080}>
      <Typography variant="h4" component="h1" fontWeight="bold" sx={{ mb: 1 }}>
        応募者
      </Typography>
      <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
        段階を選ぶと、その状態で止まっている応募だけを表示します。
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {(pipelineTotal(counts) > 0 || status) && (
        <PipelineBand
          counts={counts}
          selected={status}
          onSelect={(next) => {
            setStatus(next)
            setOffset(0)
            void load(0, next)
          }}
        />
      )}

      {items.length === 0 && status ? (
        <Typography>この段階の応募はありません。別の段階を選ぶと一覧が切り替わります。</Typography>
      ) : items.length === 0 ? (
        <Paper
          elevation={0}
          sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '4px', p: 4 }}
        >
          <Typography variant="h6" gutterBottom>
            応募はまだありません
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            公開中の求人がないと応募は発生しません。求人を用意するか、学生を探してみてください。
          </Typography>
          <Stack direction="row" spacing={1}>
            <Button variant="contained" onClick={() => router.push('/company-portal/jobs')}>
              求人を作る
            </Button>
            <Button variant="outlined" onClick={() => router.push('/company-portal/students')}>
              学生を探す
            </Button>
          </Stack>
        </Paper>
      ) : (
        <Paper
          elevation={0}
          sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '4px', overflowX: 'auto' }}
        >
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>応募者</TableCell>
                <TableCell>ステータス</TableCell>
                <TableCell>応募日</TableCell>
                <TableCell>最終更新</TableCell>
                {isOwner && <TableCell>次の操作</TableCell>}
              </TableRow>
            </TableHead>
            <TableBody>
              {items.map((item) => {
                const nexts = ALLOWED_TRANSITIONS[item.status] ?? []
                return (
                  <TableRow key={item.id} hover>
                    <TableCell>
                      {item.student_name ? (
                        <Button
                          size="small"
                          onClick={() => router.push(`/company-portal/students/${item.user_id}`)}
                        >
                          {item.student_name}
                        </Button>
                      ) : (
                        <Typography variant="body2" color="text.secondary">
                          非公開の学生
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <MarkLabel {...applicationMark(item.status)} />
                    </TableCell>
                    <TableCell>{formatDate(item.applied_at)}</TableCell>
                    <TableCell>{formatDate(item.status_updated_at)}</TableCell>
                    {isOwner && (
                      <TableCell>
                        {nexts.length === 0 ? (
                          <Typography variant="caption" color="text.secondary">
                            —
                          </Typography>
                        ) : (
                          <Stack direction="row" spacing={1}>
                            {nexts.map((next) => (
                              <Button
                                key={next}
                                size="small"
                                variant="outlined"
                                disabled={updatingId === item.id}
                                onClick={() => void onChangeStatus(item.id, next)}
                              >
                                {statusLabel(next)}
                              </Button>
                            ))}
                          </Stack>
                        )}
                      </TableCell>
                    )}
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </Paper>
      )}

      {total > PAGE_SIZE && (
        <Box sx={{ mt: 2, display: 'flex', gap: 1, alignItems: 'center' }}>
          <Button
            size="small"
            disabled={offset === 0}
            onClick={() => {
              const next = Math.max(offset - PAGE_SIZE, 0)
              setOffset(next)
              void load(next, status)
            }}
          >
            前へ
          </Button>
          <Typography variant="body2" color="text.secondary">
            {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} / {total}
          </Typography>
          <Button
            size="small"
            disabled={offset + PAGE_SIZE >= total}
            onClick={() => {
              const next = offset + PAGE_SIZE
              setOffset(next)
              void load(next, status)
            }}
          >
            次へ
          </Button>
        </Box>
      )}
    </PageContainer>
  )
}
