'use client'

import { FormEvent, useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { PageContainer } from '@/components/admin/PageContainer'
import { companyAuthService } from '@/lib/company-auth'
import { companyStudentService, StudentDetail, StudentTag } from '@/lib/company-students'

/** 面接レポートのJSON文字列配列を安全にパースする（不正なJSONは無視する） */
function parseJsonList(raw: string): string[] {
  if (!raw) return []
  try {
    const parsed: unknown = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === 'string') : []
  } catch {
    return []
  }
}

export function StudentDetailContent({ userId }: { userId: number }) {
  const router = useRouter()
  const [detail, setDetail] = useState<StudentDetail | null>(null)
  const [tags, setTags] = useState<StudentTag[]>([])
  const [newTag, setNewTag] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [tagError, setTagError] = useState('')

  const load = useCallback(() => {
    companyStudentService
      .detail(userId)
      .then((data) => {
        setDetail(data)
        setTags(data.tags || [])
      })
      .catch((e) => setError(e instanceof Error ? e.message : '学生情報の取得に失敗しました'))
      .finally(() => setLoading(false))
  }, [userId])

  useEffect(() => {
    if (!companyAuthService.getStoredUser()) {
      router.replace('/company-portal/sign-in')
      return
    }
    load()
  }, [router, load])

  const addTag = async (e: FormEvent) => {
    e.preventDefault()
    const name = newTag.trim()
    if (!name) return
    setTagError('')
    try {
      const res = await companyStudentService.addTag(userId, name)
      setTags(res.tags)
      setNewTag('')
    } catch {
      setTagError('タグの追加に失敗しました')
    }
  }

  const removeTag = async (tagId: number) => {
    setTagError('')
    try {
      await companyStudentService.removeTag(userId, tagId)
      setTags((prev) => prev.filter((t) => t.id !== tagId))
    } catch {
      setTagError('タグの削除に失敗しました')
    }
  }

  if (loading) {
    return (
      <PageContainer maxWidth={880}>
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      </PageContainer>
    )
  }

  if (error || !detail) {
    return (
      <PageContainer maxWidth={880}>
        <Alert severity="error">{error || '学生情報を表示できません'}</Alert>
        <Button sx={{ mt: 2 }} onClick={() => router.push('/company-portal/students')}>
          一覧へ戻る
        </Button>
      </PageContainer>
    )
  }

  const summary = detail.analysis.chat_summary

  return (
    <PageContainer maxWidth={880}>
      <Button sx={{ mb: 2 }} onClick={() => router.push('/company-portal/students')}>
        ← 一覧へ戻る
      </Button>

      <Typography variant="h4" fontWeight="bold" gutterBottom>
        学生プロフィール
      </Typography>

      <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px', mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            自社タグ
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            タグは自社内でのみ表示され、他社からは見えません。
          </Typography>
          {tagError && (
            <Alert severity="error" sx={{ mb: 1 }}>
              {tagError}
            </Alert>
          )}
          <Stack direction="row" spacing={1} sx={{ mb: 2, flexWrap: 'wrap', gap: 1 }}>
            {tags.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                タグはまだありません
              </Typography>
            ) : (
              tags.map((t) => (
                <Chip key={t.id} label={t.tag_name} onDelete={() => void removeTag(t.id)} />
              ))
            )}
          </Stack>
          <Stack component="form" onSubmit={addTag} direction="row" spacing={1}>
            <TextField
              size="small"
              label="タグを追加"
              placeholder="例: 即戦力"
              value={newTag}
              onChange={(e) => setNewTag(e.target.value)}
              inputProps={{ maxLength: 64 }}
            />
            <Button type="submit" variant="outlined" disabled={!newTag.trim()}>
              追加
            </Button>
          </Stack>
        </CardContent>
      </Card>

      {summary && (
        <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px', mb: 3 }}>
          <CardContent>
            <Typography variant="h6" gutterBottom>
              AI分析サマリー
            </Typography>
            {summary.strengths && (
              <>
                <Typography variant="subtitle2">強み</Typography>
                <Typography variant="body2" sx={{ mb: 1 }}>
                  {summary.strengths}
                </Typography>
              </>
            )}
            {summary.weaknesses && (
              <>
                <Typography variant="subtitle2">課題</Typography>
                <Typography variant="body2" sx={{ mb: 1 }}>
                  {summary.weaknesses}
                </Typography>
              </>
            )}
            {summary.recommended_working_style && (
              <>
                <Typography variant="subtitle2">向いている働き方</Typography>
                <Typography variant="body2">{summary.recommended_working_style}</Typography>
              </>
            )}
          </CardContent>
        </Card>
      )}

      <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            面接実績
          </Typography>
          {detail.analysis.interview_reports.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              面接レポートはまだありません
            </Typography>
          ) : (
            detail.analysis.interview_reports.map((report, index) => (
              <Box key={report.session_id}>
                {index > 0 && <Divider sx={{ my: 2 }} />}
                <Typography variant="body2" sx={{ mb: 1 }}>
                  {report.summary_text}
                </Typography>
                {parseJsonList(report.strengths_json).length > 0 && (
                  <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', gap: 1 }}>
                    {parseJsonList(report.strengths_json).map((s) => (
                      <Chip key={s} label={s} size="small" color="primary" variant="outlined" />
                    ))}
                  </Stack>
                )}
              </Box>
            ))
          )}
        </CardContent>
      </Card>
    </PageContainer>
  )
}
