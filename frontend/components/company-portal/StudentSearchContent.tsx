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
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { PageContainer } from '@/components/admin/PageContainer'
import { companyAuthService } from '@/lib/company-auth'
import {
  companyStudentService,
  StudentFilters,
  StudentListItem,
} from '@/lib/company-students'

type IndustryOption = { id: number; name: string; level: number }

const PAGE_SIZE = 20

export function StudentSearchContent() {
  const router = useRouter()

  const [items, setItems] = useState<StudentListItem[]>([])
  const [total, setTotal] = useState(0)
  const [industries, setIndustries] = useState<IndustryOption[]>([])
  const [tagNames, setTagNames] = useState<string[]>([])

  const [industryId, setIndustryId] = useState('')
  const [location, setLocation] = useState('')
  const [skill, setSkill] = useState('')
  const [tag, setTag] = useState('')
  const [semanticQuery, setSemanticQuery] = useState('')
  const [page, setPage] = useState(0)

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  // セマンティック検索中は関連度順のためページングを無効化する
  const [isSemantic, setIsSemantic] = useState(false)

  const buildFilters = useCallback(
    (offset: number): StudentFilters => ({
      industryId: industryId ? Number(industryId) : undefined,
      location: location.trim() || undefined,
      skill: skill.trim() || undefined,
      tag: tag.trim() || undefined,
      limit: PAGE_SIZE,
      offset,
    }),
    [industryId, location, skill, tag],
  )

  const runSearch = useCallback(
    async (nextPage: number) => {
      setLoading(true)
      setError('')
      try {
        const filters = buildFilters(nextPage * PAGE_SIZE)
        const query = semanticQuery.trim()
        const result = query
          ? await companyStudentService.semanticSearch(query, { ...filters, offset: 0 })
          : await companyStudentService.search(filters)
        setItems(result.items)
        setTotal(result.total)
        setIsSemantic(Boolean(query))
        setPage(query ? 0 : nextPage)
      } catch (e) {
        setError(e instanceof Error ? e.message : '検索に失敗しました')
      } finally {
        setLoading(false)
      }
    },
    [buildFilters, semanticQuery],
  )

  useEffect(() => {
    if (!companyAuthService.getStoredUser()) {
      router.replace('/company-portal/sign-in')
      return
    }
    Promise.all([
      companyStudentService.search({ limit: PAGE_SIZE, offset: 0 }),
      companyStudentService.listTagNames().catch(() => ({ items: [] })),
      fetch('/api/company-portal/industries', {
        headers: companyAuthService.getAuthHeaders(),
      })
        .then((r) => (r.ok ? r.json() : { items: [] }))
        .catch(() => ({ items: [] })),
    ])
      .then(([result, tags, industryRes]) => {
        setItems(result.items)
        setTotal(result.total)
        setTagNames(tags.items || [])
        setIndustries((industryRes.items as IndustryOption[]) || [])
      })
      .catch(() => setError('学生一覧の取得に失敗しました'))
      .finally(() => setLoading(false))
  }, [router])

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    void runSearch(0)
  }

  const clearFilters = () => {
    setIndustryId('')
    setLocation('')
    setSkill('')
    setTag('')
    setSemanticQuery('')
  }

  return (
    <PageContainer maxWidth={1080}>
      <Typography variant="h4" fontWeight="bold" gutterBottom>
        学生を探す
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        公開に同意した学生のみが表示されます。
      </Typography>

      <Card
        component="form"
        onSubmit={onSubmit}
        elevation={0}
        sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px', mb: 3 }}
      >
        <CardContent>
          <TextField
            fullWidth
            label="自然文で検索"
            placeholder="例: リーダーシップ経験があってReactができる学生"
            value={semanticQuery}
            onChange={(e) => setSemanticQuery(e.target.value)}
            helperText="入力すると意味の近い学生を関連度順で表示します（下のフィルタと併用できます）"
            sx={{ mb: 2 }}
          />
          <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mb: 2 }}>
            <TextField
              select
              fullWidth
              label="希望業界"
              value={industryId}
              onChange={(e) => setIndustryId(e.target.value)}
            >
              <MenuItem value="">指定なし</MenuItem>
              {industries.map((industry) => (
                <MenuItem key={industry.id} value={String(industry.id)}>
                  {industry.level > 0 ? `　${industry.name}` : industry.name}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              fullWidth
              label="希望勤務地"
              value={location}
              onChange={(e) => setLocation(e.target.value)}
            />
            <TextField
              fullWidth
              label="スキル・資格"
              value={skill}
              onChange={(e) => setSkill(e.target.value)}
            />
            <TextField
              select
              fullWidth
              label="自社タグ"
              value={tag}
              onChange={(e) => setTag(e.target.value)}
            >
              <MenuItem value="">指定なし</MenuItem>
              {tagNames.map((name) => (
                <MenuItem key={name} value={name}>
                  {name}
                </MenuItem>
              ))}
            </TextField>
          </Stack>
          <Stack direction="row" spacing={1}>
            <Button type="submit" variant="contained" disabled={loading}>
              検索
            </Button>
            <Button type="button" variant="text" onClick={clearFilters} disabled={loading}>
              条件をクリア
            </Button>
          </Stack>
        </CardContent>
      </Card>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {loading ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      ) : (
        <>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
            {isSemantic ? `関連度順 ${items.length} 件` : `${total} 件`}
          </Typography>

          {items.length === 0 ? (
            <Alert severity="info">条件に一致する学生が見つかりませんでした。</Alert>
          ) : (
            <Stack spacing={2}>
              {items.map((student) => (
                <Card
                  key={student.user_id}
                  elevation={0}
                  sx={{
                    border: '1px solid',
                    borderColor: 'divider',
                    borderRadius: '10px',
                    cursor: 'pointer',
                  }}
                  onClick={() => router.push(`/company-portal/students/${student.user_id}`)}
                >
                  <CardContent>
                    <Typography variant="h6">{student.name || '（氏名未設定）'}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      {[student.school_name, student.target_level].filter(Boolean).join(' / ')}
                    </Typography>
                    <Divider sx={{ my: 1 }} />
                    <Stack spacing={0.5}>
                      {student.desired_industry_name && (
                        <Typography variant="body2">希望業界: {student.desired_industry_name}</Typography>
                      )}
                      {student.desired_location && (
                        <Typography variant="body2">希望勤務地: {student.desired_location}</Typography>
                      )}
                      {student.certifications_acquired && (
                        <Typography variant="body2">取得資格: {student.certifications_acquired}</Typography>
                      )}
                    </Stack>
                    {student.tags.length > 0 && (
                      <Stack direction="row" spacing={1} sx={{ mt: 1, flexWrap: 'wrap', gap: 1 }}>
                        {student.tags.map((t) => (
                          <Chip key={t.id} label={t.tag_name} size="small" />
                        ))}
                      </Stack>
                    )}
                  </CardContent>
                </Card>
              ))}
            </Stack>
          )}

          {!isSemantic && total > PAGE_SIZE && (
            <Stack direction="row" spacing={1} justifyContent="center" sx={{ mt: 3 }}>
              <Button disabled={page === 0} onClick={() => void runSearch(page - 1)}>
                前へ
              </Button>
              <Button
                disabled={(page + 1) * PAGE_SIZE >= total}
                onClick={() => void runSearch(page + 1)}
              >
                次へ
              </Button>
            </Stack>
          )}
        </>
      )}
    </PageContainer>
  )
}
