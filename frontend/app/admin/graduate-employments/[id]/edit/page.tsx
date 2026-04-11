'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'

type Company = { id: number; name: string }
type JobPosition = { id: number; title: string; company?: Company }

export default function AdminGraduateEmploymentEditPage() {
  const router = useRouter()
  const { id } = useParams<{ id: string }>()

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) window.location.href = '/'
  }, [])

  const [companies, setCompanies] = useState<Company[]>([])
  const [jobPositions, setJobPositions] = useState<JobPosition[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [companyId, setCompanyId] = useState('')
  const [jobPositionId, setJobPositionId] = useState('')
  const [graduateName, setGraduateName] = useState('')
  const [graduationYear, setGraduationYear] = useState('')
  const [schoolName, setSchoolName] = useState('')
  const [department, setDepartment] = useState('')
  const [hiredAt, setHiredAt] = useState('')
  const [note, setNote] = useState('')

  useEffect(() => {
    if (!id) return
    const headers = authService.getAdminFetchHeaders()
    Promise.all([
      fetch('/api/admin/companies', { headers }).then((r) => r.json()),
      fetch('/api/admin/job-positions?limit=100', { headers }).then((r) => r.json()),
      fetch(`/api/admin/graduate-employments/${id}`, { headers }).then((r) => r.json()),
    ]).then(([companiesData, positionsData, entry]) => {
      setCompanies(companiesData?.companies || [])
      setJobPositions(positionsData?.positions || [])
      if (entry?.id) {
        setCompanyId(String(entry.company_id || ''))
        setJobPositionId(String(entry.job_position_id || ''))
        setGraduateName(entry.graduate_name || '')
        setGraduationYear(String(entry.graduation_year || ''))
        setSchoolName(entry.school_name || '')
        setDepartment(entry.department || '')
        setHiredAt(entry.hired_at ? entry.hired_at.slice(0, 10) : '')
        setNote(entry.note || '')
      }
      setLoading(false)
    }).catch(() => setLoading(false))
  }, [id])

  const handleUpdate = async () => {
    setError('')
    const res = await fetch(`/api/admin/graduate-employments/${id}`, {
      method: 'PUT',
      headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({
        company_id: Number(companyId),
        job_position_id: jobPositionId ? Number(jobPositionId) : undefined,
        graduate_name: graduateName,
        graduation_year: graduationYear ? Number(graduationYear) : 0,
        school_name: schoolName,
        department,
        hired_at: hiredAt,
        note,
      }),
    })
    const data = await res.json()
    if (!res.ok) {
      setError(data?.error || '更新に失敗しました')
      return
    }
    router.push('/admin/graduate-employments')
  }

  if (loading) {
    return (
      <Box sx={{ p: 4, display: 'flex', justifyContent: 'center' }}>
        <CircularProgress />
      </Box>
    )
  }

  return (
    <Box sx={{ p: 4, maxWidth: 700, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 3 }}>
        <Typography variant="h4" fontWeight="bold">
          就職情報の編集
        </Typography>
        <Button variant="outlined" size="small" onClick={() => router.push('/admin/graduate-employments')}>
          一覧に戻る
        </Button>
      </Stack>

      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      <Card>
        <CardContent>
          <Stack spacing={2}>
            <TextField select label="企業" value={companyId} onChange={(e) => setCompanyId(e.target.value)} required>
              {companies.map((c) => (
                <MenuItem key={c.id} value={c.id}>{c.name}</MenuItem>
              ))}
            </TextField>
            <TextField select label="応募職種" value={jobPositionId} onChange={(e) => setJobPositionId(e.target.value)}>
              <MenuItem value="">未設定</MenuItem>
              {jobPositions.map((p) => (
                <MenuItem key={p.id} value={p.id}>{p.title} ({p.company?.name || '企業未設定'})</MenuItem>
              ))}
            </TextField>
            <TextField label="卒業生氏名" value={graduateName} onChange={(e) => setGraduateName(e.target.value)} />
            <TextField label="卒業年度" value={graduationYear} onChange={(e) => setGraduationYear(e.target.value)} type="number" />
            <TextField label="学校名" value={schoolName} onChange={(e) => setSchoolName(e.target.value)} />
            <TextField label="学科/専攻" value={department} onChange={(e) => setDepartment(e.target.value)} />
            <TextField label="就職日 (YYYY-MM-DD)" value={hiredAt} onChange={(e) => setHiredAt(e.target.value)} />
            <TextField label="メモ" value={note} onChange={(e) => setNote(e.target.value)} multiline minRows={2} />
            <Button variant="contained" onClick={handleUpdate} disabled={!companyId}>
              更新する
            </Button>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  )
}
