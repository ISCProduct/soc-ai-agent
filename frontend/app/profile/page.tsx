'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Container,
  Divider,
  MenuItem,
  TextField,
  Typography,
  Alert,
  Snackbar,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
} from '@mui/material'
import PersonIcon from '@mui/icons-material/Person'
import SchoolIcon from '@mui/icons-material/School'
import WorkIcon from '@mui/icons-material/Work'
import EmojiEventsIcon from '@mui/icons-material/EmojiEvents'
import SaveIcon from '@mui/icons-material/Save'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { authService, User } from '@/lib/auth'
import { CERTIFICATION_OPTIONS, joinCertifications, splitCertifications } from '@/lib/profile'
import GitHubSkills from '@/components/github-skills'

export default function ProfilePage() {
  const router = useRouter()
  const [user, setUser] = useState<User | null>(null)
  const [name, setName] = useState('')
  const [targetLevel, setTargetLevel] = useState('新卒')
  const [schoolName, setSchoolName] = useState('')
  const [schoolOption, setSchoolOption] = useState('other')
  const [certificationsAcquired, setCertificationsAcquired] = useState<string[]>([])
  const [certificationsInProgress, setCertificationsInProgress] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [saved, setSaved] = useState(false)
  const [isFirstTime, setIsFirstTime] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)

  useEffect(() => {
    const storedUser = authService.getStoredUser()
    if (!storedUser) {
      router.replace('/login')
      return
    }
    setUser(storedUser)
    setName(storedUser.name || '')

    const storedSchool = storedUser.school_name || ''
    const predefinedSchools = ['学校法人岩崎学園情報科学専門学校']
    if (predefinedSchools.includes(storedSchool)) {
      setSchoolOption(storedSchool)
      setSchoolName(storedSchool)
    } else {
      setSchoolOption('other')
      setSchoolName(storedSchool)
    }

    setCertificationsAcquired(splitCertifications(storedUser.certifications_acquired))
    setCertificationsInProgress(storedUser.certifications_in_progress || '')

    if (storedUser.target_level === '新卒' || storedUser.target_level === '中途') {
      setTargetLevel(storedUser.target_level)
    }

    // 初回セットアップかどうか（target_levelが未設定なら初回）
    const firstTime = !storedUser.target_level || (storedUser.target_level !== '新卒' && storedUser.target_level !== '中途')
    setIsFirstTime(firstTime)
  }, [router])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!user) return

    setError('')
    setLoading(true)
    try {
      const finalSchoolName = schoolOption === 'other' ? schoolName : schoolOption
      const response = await authService.updateProfile(
        user.user_id,
        name,
        targetLevel,
        finalSchoolName,
        joinCertifications(certificationsAcquired),
        certificationsInProgress,
      )
      authService.saveAuth(response)
      setUser(authService.getStoredUser())
      setSaved(true)
      if (isFirstTime) {
        router.replace('/')
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '保存に失敗しました')
    } finally {
      setLoading(false)
    }
  }

  const handleDeleteAccount = async () => {
    if (!user) return
    setDeleting(true)
    try {
      const res = await fetch(`/api/auth/account?user_id=${user.user_id}`, {
        method: 'DELETE',
        headers: authService.getUserFetchHeaders(),
      })
      if (!res.ok) {
        const err = await res.text()
        setError(err || 'アカウント削除に失敗しました')
        return
      }
      authService.logout?.()
      router.replace('/login?deleted=1')
    } finally {
      setDeleting(false)
      setDeleteDialogOpen(false)
    }
  }

  if (!user) return null

  const avatarLetter = name ? name[0].toUpperCase() : user.email[0].toUpperCase()
  const isGitHubUser = user.oauth_provider === 'github'
  const isGuest = user.is_guest === true

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#f5f7fa' }}>
      {/* ページヘッダー */}
      <Box sx={{ bgcolor: 'white', borderBottom: '1px solid #e0e0e0', px: { xs: 2, sm: 3 }, py: 1.5 }}>
        <Container maxWidth="lg">
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <Typography variant="h6" fontWeight="bold" color="primary">
              プロフィール
            </Typography>
            <Button
              startIcon={<ArrowBackIcon />}
              onClick={() => router.push('/')}
              size="small"
            >
              ホームへ戻る
            </Button>
          </Box>
        </Container>
      </Box>

      <Container maxWidth="lg" sx={{ py: 4 }}>
        {/* ユーザーヘッダーカード */}
        <Card sx={{ mb: 3, borderRadius: 2 }}>
          <CardContent sx={{ p: 3 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2.5, flexWrap: 'wrap' }}>
              <Avatar
                src={user.avatar_url || undefined}
                sx={{ width: 72, height: 72, fontSize: '1.8rem', bgcolor: 'primary.main' }}
              >
                {!user.avatar_url && avatarLetter}
              </Avatar>
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', mb: 0.5 }}>
                  <Typography variant="h5" fontWeight="bold" noWrap>
                    {name || '（名前未設定）'}
                  </Typography>
                  {isGitHubUser && (
                    <Chip
                      label="GitHub連携済み"
                      size="small"
                      sx={{ bgcolor: '#24292e', color: 'white', fontSize: '0.7rem' }}
                    />
                  )}
                  {(targetLevel === '新卒' || targetLevel === '中途') && (
                    <Chip
                      label={targetLevel}
                      size="small"
                      color="primary"
                      variant="outlined"
                    />
                  )}
                </Box>
                <Typography variant="body2" color="text.secondary">
                  {user.email}
                </Typography>
                {schoolName && (
                  <Typography variant="body2" color="text.secondary" sx={{ mt: 0.25 }}>
                    {schoolOption === 'other' ? schoolName : schoolOption}
                  </Typography>
                )}
              </Box>
            </Box>
          </CardContent>
        </Card>

        {/* メインコンテンツ */}
        <Box sx={{ display: 'flex', gap: 3, alignItems: 'flex-start', flexWrap: { xs: 'wrap', md: 'nowrap' } }}>
          {/* 左カラム: プロフィール編集フォーム */}
          <Box sx={{ width: { xs: '100%', md: 420 }, flexShrink: 0 }}>
            <Card sx={{ borderRadius: 2, height: '100%' }}>
              <CardContent sx={{ p: 3 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 3 }}>
                  <PersonIcon color="primary" />
                  <Typography variant="h6" fontWeight="bold">
                    {isFirstTime ? 'はじめに情報を入力してください' : 'プロフィール編集'}
                  </Typography>
                </Box>

                {isFirstTime && (
                  <Alert severity="info" sx={{ mb: 2 }}>
                    診断の質問内容を最適化するために必要です。
                  </Alert>
                )}

                {error && (
                  <Alert severity="error" sx={{ mb: 2 }}>
                    {error}
                  </Alert>
                )}

                <Box component="form" onSubmit={handleSubmit}>
                  {/* 基本情報 */}
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1.5 }}>
                    <PersonIcon sx={{ fontSize: 16, color: 'text.secondary' }} />
                    <Typography variant="body2" fontWeight="bold" color="text.secondary">
                      基本情報
                    </Typography>
                  </Box>
                  <TextField
                    fullWidth
                    label="名前"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    required
                    size="small"
                    sx={{ mb: 2 }}
                  />
                  <TextField
                    fullWidth
                    select
                    label="区分"
                    value={targetLevel}
                    onChange={(e) => setTargetLevel(e.target.value)}
                    size="small"
                    sx={{ mb: 3 }}
                  >
                    <MenuItem value="新卒">新卒</MenuItem>
                    <MenuItem value="中途">中途</MenuItem>
                  </TextField>

                  <Divider sx={{ mb: 2 }} />

                  {/* 学校情報 */}
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1.5 }}>
                    <SchoolIcon sx={{ fontSize: 16, color: 'text.secondary' }} />
                    <Typography variant="body2" fontWeight="bold" color="text.secondary">
                      学校
                    </Typography>
                  </Box>
                  <TextField
                    fullWidth
                    select
                    label="学校名"
                    value={schoolOption}
                    onChange={(e) => setSchoolOption(e.target.value)}
                    required
                    size="small"
                    sx={{ mb: 2 }}
                  >
                    <MenuItem value="学校法人岩崎学園情報科学専門学校">
                      学校法人岩崎学園情報科学専門学校
                    </MenuItem>
                    <MenuItem value="other">その他</MenuItem>
                  </TextField>
                  {schoolOption === 'other' && (
                    <TextField
                      fullWidth
                      label="学校名（その他）"
                      value={schoolName}
                      onChange={(e) => setSchoolName(e.target.value)}
                      required
                      size="small"
                      sx={{ mb: 2 }}
                    />
                  )}

                  <Divider sx={{ mb: 2 }} />

                  {/* 資格 */}
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1.5 }}>
                    <EmojiEventsIcon sx={{ fontSize: 16, color: 'text.secondary' }} />
                    <Typography variant="body2" fontWeight="bold" color="text.secondary">
                      資格
                    </Typography>
                  </Box>
                  <TextField
                    fullWidth
                    select
                    label="取得資格"
                    value={certificationsAcquired}
                    onChange={(e) =>
                      setCertificationsAcquired(
                        typeof e.target.value === 'string' ? e.target.value.split(',') : (e.target.value as string[]),
                      )
                    }
                    SelectProps={{
                      multiple: true,
                      renderValue: (selected) => (selected as string[]).join(', '),
                    }}
                    helperText="複数選択可"
                    size="small"
                    sx={{ mb: 2 }}
                  >
                    {CERTIFICATION_OPTIONS.map((option) => (
                      <MenuItem key={option} value={option}>
                        {option}
                      </MenuItem>
                    ))}
                  </TextField>
                  <TextField
                    fullWidth
                    label="勉強中の資格"
                    value={certificationsInProgress}
                    onChange={(e) => setCertificationsInProgress(e.target.value)}
                    placeholder="例: 応用情報技術者、AWS SAA（改行区切り可）"
                    multiline
                    minRows={2}
                    size="small"
                    sx={{ mb: 3 }}
                  />

                  <Button
                    type="submit"
                    fullWidth
                    variant="contained"
                    size="large"
                    disabled={loading}
                    startIcon={<SaveIcon />}
                  >
                    {loading ? '保存中...' : isFirstTime ? '登録して診断を始める' : '保存する'}
                  </Button>
                </Box>
              </CardContent>
            </Card>
          </Box>

          {/* 右カラム: GitHub スキル分析（ゲストアカウントは非表示） */}
          {!isGuest && (
            <Box sx={{ flex: 1, minWidth: 0 }}>
              <GitHubSkills userId={user.user_id} />
            </Box>
          )}
        </Box>
      </Container>

      {/* アカウント管理セクション */}
      <Container maxWidth="lg" sx={{ pb: 6 }}>
        <Divider sx={{ my: 4 }} />
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 2 }}>
          <Box>
            <Typography variant="body2" color="text.secondary">
              プライバシーポリシー・データ利用について
            </Typography>
            <Button size="small" onClick={() => router.push('/privacy')} sx={{ p: 0, minWidth: 0, textDecoration: 'underline' }}>
              プライバシーポリシーを確認
            </Button>
          </Box>
          <Button
            variant="outlined"
            color="error"
            size="small"
            onClick={() => setDeleteDialogOpen(true)}
          >
            アカウントを削除する
          </Button>
        </Box>
      </Container>

      {/* アカウント削除確認ダイアログ */}
      <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)} maxWidth="xs" fullWidth>
        <DialogTitle>アカウントを削除しますか？</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary">
            この操作は取り消せません。以下のデータがすべて完全に削除されます。
          </Typography>
          <Typography variant="body2" component="ul" sx={{ pl: 2, mt: 1, color: 'text.secondary' }}>
            <li>チャット履歴・就職軸スコア</li>
            <li>マッチング結果</li>
            <li>面接練習の録画・レポート</li>
            <li>職務経歴書</li>
            <li>アカウント情報</li>
          </Typography>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button onClick={() => setDeleteDialogOpen(false)} disabled={deleting}>
            キャンセル
          </Button>
          <Button
            onClick={handleDeleteAccount}
            color="error"
            variant="contained"
            disabled={deleting}
          >
            {deleting ? '削除中...' : '削除する'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 保存成功トースト */}
      <Snackbar
        open={saved}
        autoHideDuration={3000}
        onClose={() => setSaved(false)}
        message="プロフィールを保存しました"
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      />
    </Box>
  )
}
