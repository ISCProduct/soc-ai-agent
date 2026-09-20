'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  Paper,
  Select,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import { PageContainer } from '@/components/admin/PageContainer'
import { PageLoading } from '@/components/common/PageLoading'
import { companyAuthService } from '@/lib/company-auth'
import {
  companyProfileService,
  type CompanyMember,
  type CompanyProfile,
  type ProfileUpdate,
} from '@/lib/company-profile'

export default function CompanyPortalSettingsPage() {
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [isOwner, setIsOwner] = useState(false)
  const [myUserId, setMyUserId] = useState<number | null>(null)
  const [profile, setProfile] = useState<CompanyProfile | null>(null)
  const [form, setForm] = useState<ProfileUpdate>({})
  const [members, setMembers] = useState<CompanyMember[]>([])
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [saving, setSaving] = useState(false)

  const [inviteOpen, setInviteOpen] = useState(false)
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteName, setInviteName] = useState('')
  const [inviteRole, setInviteRole] = useState<'owner' | 'member'>('member')

  const load = useCallback(async () => {
    try {
      const [p, m] = await Promise.all([
        companyProfileService.get(),
        companyProfileService.listMembers(),
      ])
      setProfile(p)
      setForm({
        description: p.description,
        industry: p.industry,
        location: p.location,
        website_url: p.website_url,
        founded_year: p.founded_year,
        employee_count: p.employee_count,
        culture: p.culture,
        work_style: p.work_style,
        welfare_details: p.welfare_details,
        main_business: p.main_business,
      })
      setMembers(m)
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : '情報を取得できませんでした')
    }
  }, [])

  useEffect(() => {
    const stored = companyAuthService.getStoredUser()
    if (!stored) {
      router.replace('/company-portal/sign-in')
      return
    }
    companyAuthService
      .fetchMe()
      .then((me) => {
        setIsOwner(me.role === 'owner')
        setMyUserId(me.company_user_id)
        return load()
      })
      .catch(() => {
        companyAuthService.logout()
        router.replace('/company-portal/sign-in')
      })
      .finally(() => setLoading(false))
  }, [router, load])

  const saveProfile = async () => {
    setSaving(true)
    try {
      const updated = await companyProfileService.update(form)
      setProfile(updated)
      setNotice('企業情報を保存しました')
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : '保存できませんでした')
    } finally {
      setSaving(false)
    }
  }

  const invite = async () => {
    setSaving(true)
    try {
      await companyProfileService.invite(inviteEmail, inviteName, inviteRole)
      setInviteOpen(false)
      setInviteEmail('')
      setInviteName('')
      setInviteRole('member')
      await load()
      setNotice('招待メールを送信しました')
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : '招待できませんでした')
    } finally {
      setSaving(false)
    }
  }

  const toggleDisabled = async (member: CompanyMember) => {
    setSaving(true)
    try {
      await companyProfileService.setDisabled(member.id, !member.disabled)
      await load()
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : '変更できませんでした')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <PageLoading message="設定を読み込んでいます..." />
  }

  return (
    <PageContainer maxWidth={960}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
        <Typography variant="h4" fontWeight="bold">
          設定
        </Typography>
        <Button variant="outlined" onClick={() => router.push('/company-portal')}>
          ダッシュボードへ
        </Button>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        自社の情報と担当者を管理します。
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}
      {notice && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setNotice('')}>
          {notice}
        </Alert>
      )}

      <Paper
        elevation={0}
        sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px', p: 3, mb: 3 }}
      >
        <Typography variant="h6" gutterBottom>
          企業情報
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          企業名・法人番号・公開状態は変更できません。変更が必要な場合は運営にお問い合わせください。
        </Typography>

        <Stack spacing={2}>
          <Stack direction="row" spacing={2} alignItems="center">
            <Typography variant="body2" color="text.secondary" sx={{ minWidth: 100 }}>
              企業名
            </Typography>
            <Typography>{profile?.name}</Typography>
            <Chip
              size="small"
              label={profile?.data_status === 'published' ? '公開中' : '未公開'}
              color={profile?.data_status === 'published' ? 'success' : 'default'}
            />
          </Stack>

          <TextField
            label="企業概要"
            fullWidth
            multiline
            minRows={3}
            disabled={!isOwner}
            value={form.description ?? ''}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
          />
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
            <TextField
              label="業種"
              fullWidth
              disabled={!isOwner}
              value={form.industry ?? ''}
              onChange={(e) => setForm({ ...form, industry: e.target.value })}
            />
            <TextField
              label="所在地"
              fullWidth
              disabled={!isOwner}
              value={form.location ?? ''}
              onChange={(e) => setForm({ ...form, location: e.target.value })}
            />
          </Stack>
          <TextField
            label="公式サイトURL"
            fullWidth
            disabled={!isOwner}
            value={form.website_url ?? ''}
            onChange={(e) => setForm({ ...form, website_url: e.target.value })}
          />
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
            <TextField
              label="設立年"
              type="number"
              fullWidth
              disabled={!isOwner}
              value={form.founded_year ?? 0}
              onChange={(e) => setForm({ ...form, founded_year: Number(e.target.value) })}
            />
            <TextField
              label="従業員数"
              type="number"
              fullWidth
              disabled={!isOwner}
              value={form.employee_count ?? 0}
              onChange={(e) => setForm({ ...form, employee_count: Number(e.target.value) })}
            />
          </Stack>
          <TextField
            label="主要事業"
            fullWidth
            disabled={!isOwner}
            value={form.main_business ?? ''}
            onChange={(e) => setForm({ ...form, main_business: e.target.value })}
          />
          <TextField
            label="企業文化"
            fullWidth
            multiline
            minRows={2}
            disabled={!isOwner}
            value={form.culture ?? ''}
            onChange={(e) => setForm({ ...form, culture: e.target.value })}
          />
          <TextField
            label="働き方"
            fullWidth
            disabled={!isOwner}
            value={form.work_style ?? ''}
            onChange={(e) => setForm({ ...form, work_style: e.target.value })}
          />
          <TextField
            label="福利厚生"
            fullWidth
            multiline
            minRows={2}
            disabled={!isOwner}
            value={form.welfare_details ?? ''}
            onChange={(e) => setForm({ ...form, welfare_details: e.target.value })}
          />

          {isOwner ? (
            <Stack direction="row">
              <Button variant="contained" disabled={saving} onClick={() => void saveProfile()}>
                保存
              </Button>
            </Stack>
          ) : (
            <Typography variant="body2" color="text.secondary">
              企業情報の編集は管理者のみ行えます。
            </Typography>
          )}
        </Stack>
      </Paper>

      <Paper
        elevation={0}
        sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px', p: 3 }}
      >
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
          <Typography variant="h6">担当者</Typography>
          {isOwner && (
            <Button variant="contained" onClick={() => setInviteOpen(true)}>
              担当者を招待
            </Button>
          )}
        </Stack>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          担当者の削除はできません。アクセスを止める場合は無効化してください。
        </Typography>

        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>氏名</TableCell>
              <TableCell>メールアドレス</TableCell>
              <TableCell>権限</TableCell>
              <TableCell>状態</TableCell>
              {isOwner && <TableCell>操作</TableCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {members.map((m) => (
              <TableRow key={m.id} hover>
                <TableCell>{m.name || '—'}</TableCell>
                <TableCell>{m.email}</TableCell>
                <TableCell>{m.role === 'owner' ? '管理者' : '担当者'}</TableCell>
                <TableCell>
                  {m.disabled ? (
                    <Chip size="small" label="無効" />
                  ) : m.invite_pending ? (
                    <Chip size="small" color="warning" label="招待中" />
                  ) : (
                    <Chip size="small" color="success" label="有効" />
                  )}
                </TableCell>
                {isOwner && (
                  <TableCell>
                    {m.id === myUserId ? (
                      <Typography variant="caption" color="text.secondary">
                        自分
                      </Typography>
                    ) : (
                      <Button
                        size="small"
                        variant="outlined"
                        disabled={saving}
                        onClick={() => void toggleDisabled(m)}
                      >
                        {m.disabled ? '有効にする' : '無効にする'}
                      </Button>
                    )}
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Paper>

      <Dialog open={inviteOpen} onClose={() => setInviteOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>担当者を招待</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label="メールアドレス"
              type="email"
              required
              fullWidth
              value={inviteEmail}
              onChange={(e) => setInviteEmail(e.target.value)}
            />
            <TextField
              label="氏名"
              fullWidth
              value={inviteName}
              onChange={(e) => setInviteName(e.target.value)}
            />
            <Select
              value={inviteRole}
              onChange={(e) => setInviteRole(e.target.value as 'owner' | 'member')}
            >
              <MenuItem value="member">担当者（閲覧のみ）</MenuItem>
              <MenuItem value="owner">管理者（編集・公開ができる）</MenuItem>
            </Select>
            <Typography variant="caption" color="text.secondary">
              招待メールが送信されます。受信者がパスワードを設定すると利用開始できます。
            </Typography>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setInviteOpen(false)}>キャンセル</Button>
          <Button
            variant="contained"
            disabled={saving || !inviteEmail.trim()}
            onClick={() => void invite()}
          >
            招待する
          </Button>
        </DialogActions>
      </Dialog>
    </PageContainer>
  )
}
