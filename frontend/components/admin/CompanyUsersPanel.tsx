'use client'

import { FormEvent, useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Button,
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

type CompanyUserItem = {
  id: number
  email: string
  name: string
  role: string
  password_set: boolean
  invite_pending: boolean
}

export function CompanyUsersPanel({ companyId }: { companyId: string }) {
  const [items, setItems] = useState<CompanyUserItem[]>([])
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [role, setRole] = useState('member')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [loading, setLoading] = useState(false)

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
          </TableRow>
        </TableHead>
        <TableBody>
          {items.map((u) => (
            <TableRow key={u.id}>
              <TableCell>{u.name}</TableCell>
              <TableCell>{u.email}</TableCell>
              <TableCell>{u.role}</TableCell>
              <TableCell>{u.password_set ? '有効' : '招待中'}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Stack>
  )
}
