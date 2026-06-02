'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Button,
  MenuItem,
  Stack,
  TextField,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { AdminFormContainer } from '@/components/admin/AdminFormContainer'
import { ErrorAlert } from '@/components/common/ErrorAlert'

type Company = { id: number; name: string }
type JobPosition = { id: number; title: string; company?: Company }

export default function AdminGraduateEmploymentNewPage() {
  const router = useRouter()

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) window.location.href = '/'
  }, [])

  const [companies, setCompanies] = useState<Company[]>([])
  const [jobPositions, setJobPositions] = useState<JobPosition[]>([])
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
    const headers = authService.getAdminFetchHeaders()
    fetch('/api/admin/companies/names', { headers }).then((r) => r.json()).then((d) => setCompanies(d || []))
    fetch('/api/admin/job-positions?limit=100', { headers }).then((r) => r.json()).then((d) => setJobPositions(d?.positions || []))
  }, [])

  const handleCreate = async () => {
    setError('')
    const res = await fetch('/api/admin/graduate-employments', {
      method: 'POST',
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
      setError(data?.error || '就職情報の登録に失敗しました')
      return
    }
    router.push('/admin/graduate-employments')
  }

  return (
    <AdminFormContainer
      title="就職情報の登録"
      maxWidth={700}
      backHref="/admin/graduate-employments"
      backLabel="一覧に戻る"
    >
      <ErrorAlert error={error} />
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
        <Button variant="contained" onClick={handleCreate} disabled={!companyId}>
          登録する
        </Button>
      </Stack>
    </AdminFormContainer>
  )
}
