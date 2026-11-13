'use client'

import { useCallback, useEffect, useState } from 'react'
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

/**
 * 学生からの相談の受信一覧（#2028）。担当校の学生の、案内への返信と質問を新しい順に出す。
 * 未読（まだ読んでいない学生の発言）がある相談に印を付ける。
 */
export default function AdminConsultationsPage() {
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)
  const [items, setItems] = useState<ConsultationInboxItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [openId, setOpenId] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setItems(await fetchTeacherInbox(schoolId))
    } catch (e) {
      const message = e instanceof Error ? e.message : ''
      // 担当校の先生は、学校の選択が決まるまで school_id 無しで弾かれる。エラーとして見せない
      if (schoolId === undefined && /school_id/.test(message)) {
        setItems([])
      } else {
        setError(message || '相談の一覧を読み込めませんでした')
      }
    } finally {
      setLoading(false)
    }
  }, [schoolId])

  useEffect(() => {
    void load()
  }, [load])

  const loadOpen = useCallback(() => fetchTeacherThread(openId ?? 0), [openId])
  const sendOpen = useCallback((body: string) => replyAsTeacher(openId ?? 0, body), [openId])

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader title="学生からの相談" backHref="/admin" />
      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mb: 2, alignItems: { md: 'center' } }}>
        <Typography variant="body2" color="text.secondary" sx={{ flex: 1 }}>
          担当校の学生からの、案内への返信と質問です。未読がある相談に印が付きます。
        </Typography>
        <SchoolFilterSelect value={schoolId} onChange={setSchoolId} />
      </Stack>

      {error && <Alert severity="error" sx={{ mb: 2 }} action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}>{error}</Alert>}
      {loading && items.length === 0 && <ListSkeleton rows={3} />}
      {!loading && !error && items.length === 0 && (
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

      <Dialog open={openId !== null} onClose={() => setOpenId(null)} fullWidth maxWidth="sm">
        <DialogTitle sx={{ pr: 6 }}>
          相談のやり取り
          <IconButton aria-label="閉じる" onClick={() => setOpenId(null)} sx={{ position: 'absolute', right: 8, top: 8 }}>
            <CloseIcon />
          </IconButton>
        </DialogTitle>
        <DialogContent dividers>
          {openId !== null && <ConsultationThread me="teacher" load={loadOpen} send={sendOpen} onChanged={() => void load()} />}
        </DialogContent>
      </Dialog>
    </PageContainer>
  )
}
