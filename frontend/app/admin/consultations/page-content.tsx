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

/** 返信・既読のあとの取り直しで辿るページの上限（届かなければ、取り直した分から読み直す） */
const MAX_REFRESH_PAGES = 5

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
  // 先頭の取り直しどうしの世代。重なったときは最後に始めたものだけを使う
  const refreshSeq = useRef(0)
  // 取り直しの最中に、今の一覧（2ページ目以降も含む）を読むため
  const itemsRef = useRef<ConsultationInboxItem[]>([])
  const nextBeforeIdRef = useRef<number | undefined>(undefined)
  useEffect(() => {
    itemsRef.current = items
    nextBeforeIdRef.current = nextBeforeId
  }, [items, nextBeforeId])

  const load = useCallback(async () => {
    const seq = ++requestSeq.current
    setLoading(true)
    setError('')
    setRefreshError('')
    // 前の学校の「続き」の位置を、新しい学校の一覧に送らない
    setNextBeforeId(undefined)
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
    if (!nextBeforeId || loading) return
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

  // スレッドで返信・既読にしたあとは、先頭から取り直して差し替える。一覧ごと読み直すと、
  // 「さらに読み込む」で足した2ページ目以降が消えるため。新しい発言は ID が大きいので先頭に来る。
  // 取り直した範囲と、読み込み済みの残りの間に隙間が出ないよう、読み込み済みの先頭に届くまでページを辿る
  // （届かなければ残りを捨て、取り直した分から続きを読む）。
  const refreshHead = useCallback(async (openedId?: number) => {
    const seq = requestSeq.current
    const rseq = ++refreshSeq.current
    // load の開始・別の取り直しの開始のどちらかがあれば、この応答は使わない
    const stale = () => seq !== requestSeq.current || rseq !== refreshSeq.current
    try {
      const fetched: ConsultationInboxItem[] = []
      let cursor: number | undefined
      let reached = false
      for (let i = 0; i < MAX_REFRESH_PAGES; i++) {
        const page = await fetchTeacherInbox(schoolId, cursor)
        if (stale()) return
        fetched.push(...page.items.filter((it) => !fetched.some((f) => f.guidance_id === it.guidance_id)))
        cursor = page.next_before_id
        // 読み込み済みの一覧は、その先頭から続きの位置まで隙間なく取ってある。取り直した範囲がその先頭
        // （前に取ったときの ID）まで届けば、そこから下は読み込み済みの分でつながる
        const loadedTop = itemsRef.current.reduce((max, p) => Math.max(max, p.last_message_id), 0)
        const tail = fetched.at(-1)?.last_message_id ?? 0
        if (!cursor || loadedTop === 0 || tail <= loadedTop) {
          reached = true
          break
        }
      }
      const tail = fetched.at(-1)?.last_message_id ?? 0
      // 残りのうち、取り直した範囲より古い（ID の小さい）ものだけを後ろに残す
      const rest = reached && cursor
        ? itemsRef.current.filter((p) => !fetched.some((f) => f.guidance_id === p.guidance_id) && p.last_message_id < tail)
        : []
      const next = rest.length > 0 ? nextBeforeIdRef.current : cursor
      // 開いていた相談が取り直した範囲に無ければ（2ページ目以降）、画面に出した発言は既読にしたので未読を消す。
      // あとから届いた発言は ID が大きいので、次の取り直しで先頭に来る
      const merged = [...fetched, ...rest].map((p) =>
        openedId !== undefined && p.guidance_id === openedId && !fetched.some((f) => f.guidance_id === openedId) ? { ...p, unread: 0 } : p,
      )
      setItems(merged)
      setNextBeforeId(next)
      setRefreshError('')
    } catch {
      // スレッドの操作は終わっているので、スレッドはそのまま。一覧の未読表示が古いことだけを知らせ、取り直せるようにする
      if (!stale()) setRefreshError('一覧を最新にできませんでした。未読の表示が古い可能性があります')
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
  const sendOpen = useCallback((body: string, seenIds: number[]) => replyAsTeacher(openId ?? 0, body, seenIds), [openId])
  const onThreadChanged = useCallback(() => void refreshHead(openId ?? undefined), [refreshHead, openId])

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
        <Alert severity="warning" sx={{ mb: 2 }} action={<Button color="inherit" size="small" onClick={() => void refreshHead(openId ?? undefined)}>再試行</Button>}>{refreshError}</Alert>
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
          <Button onClick={() => void loadMore()} disabled={loadingMore || loading}>{loadingMore ? '読み込み中…' : 'さらに読み込む'}</Button>
        </Box>
      )}

      <Dialog open={openId !== null} onClose={() => setOpenId(null)} fullWidth maxWidth="sm" aria-labelledby="admin-consultation-dialog-title">
        <DialogTitle id="admin-consultation-dialog-title" sx={{ pr: 6 }}>
          相談のやり取り
          <IconButton aria-label="閉じる" onClick={() => setOpenId(null)} sx={{ position: 'absolute', right: 8, top: 8 }}>
            <CloseIcon />
          </IconButton>
        </DialogTitle>
        <DialogContent dividers>
          {openId !== null && <ConsultationThread key={openId} me="teacher" load={loadOpen} send={sendOpen} onChanged={onThreadChanged} />}
        </DialogContent>
      </Dialog>
    </PageContainer>
  )
}
