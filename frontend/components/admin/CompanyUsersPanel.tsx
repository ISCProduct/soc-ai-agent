'use client'

import { FormEvent, useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Chip,
  MenuItem,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { adminFetchJson, toAdminErrorMessage } from '@/lib/admin-fetch'

type CompanyUserItem = {
  id: number
  email: string
  name: string
  role: string
  password_set: boolean
  invite_pending: boolean
  /** 無効化済み。退職者のアクセスを止める手段 (#1196)。行削除はタグの参照があるため不可 */
  disabled: boolean
  disabled_at: string | null
}

/** 無効 > 招待中 > 有効 の順に判定する。無効化済みを「有効」と出さないため順序が重要 (#1196) */
function statusChip(user: CompanyUserItem) {
  if (user.disabled) return <Chip size="small" color="error" variant="filled" label="無効" />
  if (!user.password_set) return <Chip size="small" color="warning" variant="outlined" label="招待中" />
  return <Chip size="small" color="success" variant="outlined" label="有効" />
}

export function CompanyUsersPanel({ companyId }: { companyId: string }) {
  const [items, setItems] = useState<CompanyUserItem[]>([])
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [role, setRole] = useState('member')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)
  // 二重送信防止。処理中の行だけボタンを無効化する
  const [pendingId, setPendingId] = useState<number | null>(null)

  const loadUsers = useCallback(() => {
    fetch(`/api/admin/companies/${companyId}/company-users`, {
      headers: authService.getAdminFetchHeaders(),
    })
      .then(async (r) => {
        if (!r.ok) throw new Error('failed')
        const data = await r.json()
        setItems((data.items as CompanyUserItem[]) || [])
      })
      .catch(() => setError('企業ユーザーの取得に失敗しました'))
  }, [companyId])

  useEffect(() => {
    loadUsers()
  }, [loadUsers])

  const handleToggleDisabled = async (user: CompanyUserItem) => {
    const disabled = !user.disabled
    if (disabled && !window.confirm(`${user.email} を無効化します。この担当者は企業ポータルにログインできなくなります。よろしいですか？`)) {
      return
    }
    setError('')
    setSuccess('')
    setPendingId(user.id)
    try {
      await adminFetchJson(
        `/api/admin/companies/${companyId}/company-users/${user.id}`,
        {
          method: 'PATCH',
          headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({ disabled }),
        },
        disabled ? '無効化に失敗しました' : '再有効化に失敗しました',
      )
      setSuccess(disabled ? '企業ユーザーを無効化しました' : '企業ユーザーを再有効化しました')
      loadUsers()
    } catch (e) {
      setError(toAdminErrorMessage(e))
    } finally {
      setPendingId(null)
    }
  }

  const handleInvite = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    setLoading(true)
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/company-users`, {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
        body: JSON.stringify({ email, name, role }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error((data as { error?: string }).error || 'invite failed')
      }
      setEmail('')
      setName('')
      setRole('member')
      setSuccess('招待メールを送信しました')
      loadUsers()
    } catch (err) {
      setError(err instanceof Error ? err.message : '招待に失敗しました')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Stack spacing={2} component="form" onSubmit={handleInvite}>
      <Typography variant="h6">企業ポータル担当者</Typography>
      {error && <Alert severity="error">{error}</Alert>}
      {success && <Alert severity="success">{success}</Alert>}
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
        <TextField label="メール" value={email} onChange={(e) => setEmail(e.target.value)} required fullWidth />
        <TextField label="氏名" value={name} onChange={(e) => setName(e.target.value)} required fullWidth />
        <TextField select label="ロール" value={role} onChange={(e) => setRole(e.target.value)} sx={{ minWidth: 140 }}>
          <MenuItem value="owner">owner</MenuItem>
          <MenuItem value="member">member</MenuItem>
        </TextField>
        <Button type="submit" variant="contained" disabled={loading}>
          招待
        </Button>
      </Stack>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>氏名</TableCell>
            <TableCell>メール</TableCell>
            <TableCell>ロール</TableCell>
            <TableCell>状態</TableCell>
            <TableCell>操作</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {items.map((u) => (
            <TableRow key={u.id}>
              <TableCell>{u.name}</TableCell>
              <TableCell>{u.email}</TableCell>
              <TableCell>{u.role}</TableCell>
              <TableCell>{statusChip(u)}</TableCell>
              <TableCell>
                <Button
                  type="button"
                  size="small"
                  color={u.disabled ? 'primary' : 'error'}
                  disabled={pendingId === u.id}
                  onClick={() => handleToggleDisabled(u)}
                >
                  {u.disabled ? '再有効化' : '無効化'}
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Stack>
  )
}
