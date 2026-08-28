'use client'

import { FormEvent, Suspense, useMemo, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Alert, Button, CircularProgress, Stack, TextField, Typography } from '@mui/material'
import { companyAuthService } from '@/lib/company-auth'

function SetupContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const token = useMemo(() => searchParams.get('token') || '', [searchParams])
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!token) {
      setError('招待トークンが見つかりません')
      return
    }
    setError('')
    setLoading(true)
    try {
      await companyAuthService.acceptInvite(token, password, name || undefined)
      router.push('/company-portal')
    } catch {
      setError('招待の受諾に失敗しました。リンクの有効期限を確認してください')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Stack spacing={3} component="form" onSubmit={handleSubmit}>
      <Typography variant="h4">アカウント有効化</Typography>
      {!token && <Alert severity="error">招待リンクが無効です</Alert>}
      {error && <Alert severity="error">{error}</Alert>}
      <TextField
        label="お名前（任意）"
        value={name}
        onChange={(e) => setName(e.target.value)}
        fullWidth
      />
      <TextField
        label="パスワード（8文字以上）"
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
        fullWidth
      />
      <Button type="submit" variant="contained" disabled={loading || !token}>
        有効化してログイン
      </Button>
    </Stack>
  )
}

export default function CompanyPortalSetupPage() {
  return (
    <Suspense fallback={<CircularProgress />}>
      <SetupContent />
    </Suspense>
  )
}
