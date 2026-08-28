'use client'

import { FormEvent, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Alert, Button, Stack, TextField, Typography } from '@mui/material'
import { companyAuthService } from '@/lib/company-auth'

export function CompanyPortalLoginContent() {
  const router = useRouter()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await companyAuthService.login(email, password)
      router.push('/company-portal')
    } catch {
      setError('メールアドレスまたはパスワードが正しくありません')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Stack spacing={3} component="form" onSubmit={handleSubmit}>
      <Typography variant="h4">企業ポータル ログイン</Typography>
      {error && <Alert severity="error">{error}</Alert>}
      <TextField
        label="メールアドレス"
        type="email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
        fullWidth
      />
      <TextField
        label="パスワード"
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
        fullWidth
      />
      <Button type="submit" variant="contained" disabled={loading}>
        ログイン
      </Button>
    </Stack>
  )
}
