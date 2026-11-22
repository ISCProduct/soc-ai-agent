'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Alert,
  Badge,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Stack,
  Typography,
} from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { SchoolFilterSelect } from '@/components/admin/SchoolFilterSelect'
import { EmptyState } from '@/components/common/EmptyState'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { ConsultationThread } from '@/components/consultation/ConsultationThread'
import { fetchTeacherInbox, fetchTeacherThread, replyAsTeacher, type ConsultationInboxItem } from '@/lib/consultation'

function formatTime(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleString('ja-JP', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

type ScopeState = 'pending' | 'ready' | 'failed'

/**
 * 学生からの相談の受信一覧（#2028）。担当校の学生の、案内への返信と質問を新しい順に出す。
 * 未読（まだ読んでいない学生の発言）がある相談に印を付ける。
 *
 * 担当校の解決（SchoolFilterSelect）が終わるまでは一覧を取らない（school_id 無しで弾かれるため）。
 * 解決に失敗したらエラーと再試行を出し、取得に成功して 0 件のときだけ「まだありません」を出す。
 */
export default function AdminConsultationsPage() {
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)
  const [scope, setScope] = useState<ScopeState>('pending')
  const [scopeError, setScopeError] = useState('')
  const [scopeKey, setScopeKey] = useState(0)
  const [items, setItems] = useState<ConsultationInboxItem[]>([])
  const [nextBeforeId, setNextBeforeId] = useState<number | undefined>(undefined)
  const [loaded, setLoaded] = useState(false)
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState('')
  // 返信・既読のあとの一覧の取り直しに失敗した（スレッドの操作は成功している）
  const [refreshError, setRefreshError] = useState('')
  const [openId, setOpenId] = useState<number | null>(null)

  // 学校を切り替えたあとに、前の学校の遅い応答で一覧を上書きしない
  const requestSeq = useRef(0)

  const load = useCallback(async () => {
    const seq = ++requestSeq.current
    setLoading(true)
    setError('')
    setRefreshError('')
    try {
      const page = await fetchTeacherInbox(schoolId)
      if (seq !== requestSeq.current) return
      setItems(page.items)
      setNextBeforeId(page.next_before_id)
      setLoaded(true)
    } catch (e) {
      if (seq !== requestSeq.current) return
      // 取得の失敗を、0 件の一覧に見せない
      setItems([])
      setNextBeforeId(undefined)
      setLoaded(false)
      setError(e instanceof Error && e.message ? e.message : '相談の一覧を読み込めませんでした')
    } finally {
      if (seq === requestSeq.current) setLoading(false)
    }
  }, [schoolId])

  useEffect(() => {
    if (scope === 'ready') void load()
  }, [scope, load])

  const loadMore = async () => {
    if (!nextBeforeId) return
    const seq = requestSeq.current
    setLoadingMore(true)
    try {
      const page = await fetchTeacherInbox(schoolId, nextBeforeId)
      if (seq !== requestSeq.current) return
      setItems((prev) => [...prev, ...page.items.filter((it) => !prev.some((p) => p.guidance_id === it.guidance_id))])
      setNextBeforeId(page.next_before_id)
    } catch {
      if (seq === requestSeq.current) setError('続きを読み込めませんでした。もう一度お試しください')
    } finally {
      setLoadingMore(false)
    }
  }

  // スレッドで返信・既読にしたあとは、先頭のページだけ取り直して差し替える。
  // 一覧ごと読み直すと、「さらに読み込む」で足した2ページ目以降が消えるため。
  const refreshHead = useCallback(async () => {
    const seq = requestSeq.current
    try {
      const page = await fetchTeacherInbox(schoolId)
      if (seq !== requestSeq.current) return
      setItems((prev) => {
        const rest = prev.filter((p) => !page.items.some((it) => it.guidance_id === p.guidance_id))
        // 先頭のページより後ろを読んでいなければ、続きの位置は取り直した結果に合わせる
        if (prev.length <= page.items.length) setNextBeforeId(page.next_before_id)
        return [...page.items, ...rest]
      })
      if (seq === requestSeq.current) setRefreshError('')
    } catch {
      // スレッドの操作は終わっているので、スレッドはそのまま。一覧の未読表示が古いことだけを知らせ、取り直せるようにする
      if (seq === requestSeq.current) setRefreshError('一覧を最新にできませんでした。未読の表示が古い可能性があります')
    }
  }, [schoolId])

  const handleScopeResolved = useCallback((failure?: string) => {
    if (failure) {
      setScope('failed')
      setScopeError(failure)
    } else {
      setScope('ready')
      setScopeError('')
    }
  }, [])

  const retryScope = () => {
    setScope('pending')
    setScopeError('')
    setScopeKey((k) => k + 1)
  }

  const loadOpen = useCallback(() => fetchTeacherThread(openId ?? 0), [openId])
  const sendOpen = useCallback((body: string, lastSeenId: number | undefined) => replyAsTeacher(openId ?? 0, body, lastSeenId), [openId])

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader title="学生からの相談" backHref="/admin" />
      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mb: 2, alignItems: { md: 'center' } }}>
        <Typography variant="body2" color="text.secondary" sx={{ flex: 1 }}>
          担当校の学生からの、案内への返信と質問です。未読がある相談に印が付きます。
        </Typography>
        <SchoolFilterSelect key={scopeKey} value={schoolId} onChange={setSchoolId} onResolved={handleScopeResolved} />
      </Stack>

      {scope === 'failed' && (
        <Alert severity="error" sx={{ mb: 2 }} action={<Button color="inherit" size="small" onClick={retryScope}>再試行</Button>}>
          担当校の情報を読み込めませんでした。{scopeError}
        </Alert>
      )}
      {error && <Alert severity="error" sx={{ mb: 2 }} action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}>{error}</Alert>}
      {refreshError && !error && (
        <Alert severity="warning" sx={{ mb: 2 }} action={<Button color="inherit" size="small" onClick={() => void refreshHead()}>再試行</Button>}>{refreshError}</Alert>
      )}
      {(scope === 'pending' || (loading && items.length === 0)) && scope !== 'failed' && <ListSkeleton rows={3} />}
      {scope === 'ready' && loaded && !loading && !error && items.length === 0 && (
        <EmptyState title="相談はまだありません" description="学生が案内に返信したり質問したりすると、ここに表示されます。" />
      )}

      <Stack component="ul" spacing={1} sx={{ listStyle: 'none', p: 0, m: 0 }}>
        {items.map((it) => (
          <Box component="li" key={it.guidance_id}>
            <Card variant="outlined">
              <CardActionArea onClick={() => setOpenId(it.guidance_id)} aria-label={`${it.student_name}さんの相談を開く${it.unread > 0 ? `（未読 ${it.unread} 件）` : ''}`}>
                <CardContent>
                  <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center' }}>
                    <Badge color="error" badgeContent={it.unread} invisible={it.unread === 0}>
                      <Typography fontWeight={700}>{it.student_name}</Typography>
                    </Badge>
                    <Typography variant="caption" color="primary">{it.kind === 'question' ? '質問' : '案内への返信'}</Typography>
                    <Typography variant="caption" color="text.secondary" sx={{ ml: 'auto' }}>{formatTime(it.last_message_at)}</Typography>
                  </Stack>
                  <Typography variant="body2" color="text.secondary" noWrap>{it.first_message}</Typography>
                </CardContent>
              </CardActionArea>
            </Card>
          </Box>
        ))}
      </Stack>
      {nextBeforeId && (
        <Box sx={{ textAlign: 'center', mt: 2 }}>
          <Button onClick={() => void loadMore()} disabled={loadingMore}>{loadingMore ? '読み込み中…' : 'さらに読み込む'}</Button>
        </Box>
      )}

      <Dialog open={openId !== null} onClose={() => setOpenId(null)} fullWidth maxWidth="sm">
        <DialogTitle sx={{ pr: 6 }}>
          相談のやり取り
          <IconButton aria-label="閉じる" onClick={() => setOpenId(null)} sx={{ position: 'absolute', right: 8, top: 8 }}>
            <CloseIcon />
          </IconButton>
        </DialogTitle>
        <DialogContent dividers>
          {openId !== null && <ConsultationThread me="teacher" load={loadOpen} send={sendOpen} onChanged={() => void refreshHead()} />}
        </DialogContent>
      </Dialog>
    </PageContainer>
  )
}
