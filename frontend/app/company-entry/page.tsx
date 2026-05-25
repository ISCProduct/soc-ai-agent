'use client'

import { useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Divider,
  IconButton,
  MenuItem,
  Slider,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'

type JobPositionForm = {
  title: string
  description: string
  employment_type: string
  work_location: string
  remote_option: boolean
  min_salary: string
  max_salary: string
  required_skills: string
  preferred_skills: string
}

type GraduateForm = {
  graduate_name: string
  graduation_year: string
  school_name: string
  department: string
  hired_at: string
  note: string
}

const defaultJob = (): JobPositionForm => ({
  title: '',
  description: '',
  employment_type: '',
  work_location: '',
  remote_option: false,
  min_salary: '',
  max_salary: '',
  required_skills: '',
  preferred_skills: '',
})

const defaultGraduate = (): GraduateForm => ({
  graduate_name: '',
  graduation_year: '',
  school_name: '',
  department: '',
  hired_at: '',
  note: '',
})

const weightLabels: { key: string; label: string }[] = [
  { key: 'technical_orientation', label: '技術志向' },
  { key: 'teamwork_orientation', label: 'チームワーク志向' },
  { key: 'leadership_orientation', label: 'リーダーシップ志向' },
  { key: 'creativity_orientation', label: '創造性志向' },
  { key: 'stability_orientation', label: '安定志向' },
  { key: 'growth_orientation', label: '成長志向' },
  { key: 'work_life_balance', label: 'ワークライフバランス' },
  { key: 'challenge_seeking', label: 'チャレンジ志向' },
  { key: 'detail_orientation', label: '細部志向' },
  { key: 'communication_skill', label: 'コミュニケーション力' },
]

export default function CompanyEntryPage() {
  const [submitted, setSubmitted] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // 企業基本情報
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [industry, setIndustry] = useState('')
  const [location, setLocation] = useState('')
  const [websiteUrl, setWebsiteUrl] = useState('')
  const [logoUrl, setLogoUrl] = useState('')
  const [corporateNumber, setCorporateNumber] = useState('')
  const [employeeCount, setEmployeeCount] = useState('')
  const [foundedYear, setFoundedYear] = useState('')
  const [averageAge, setAverageAge] = useState('')
  const [femaleRatio, setFemaleRatio] = useState('')
  const [culture, setCulture] = useState('')
  const [workStyle, setWorkStyle] = useState('')
  const [welfareDetails, setWelfareDetails] = useState('')
  const [techStack, setTechStack] = useState('')
  const [developmentStyle, setDevelopmentStyle] = useState('')
  const [mainBusiness, setMainBusiness] = useState('')

  // 求人情報
  const [jobPositions, setJobPositions] = useState<JobPositionForm[]>([defaultJob()])

  // WeightProfile
  const [weights, setWeights] = useState<Record<string, number>>(
    Object.fromEntries(weightLabels.map(({ key }) => [key, 50]))
  )

  // 卒業生就職情報
  const [graduates, setGraduates] = useState<GraduateForm[]>([])

  const updateJob = (index: number, field: keyof JobPositionForm, value: string | boolean) => {
    setJobPositions((prev) => prev.map((j, i) => (i === index ? { ...j, [field]: value } : j)))
  }

  const updateGraduate = (index: number, field: keyof GraduateForm, value: string) => {
    setGraduates((prev) => prev.map((g, i) => (i === index ? { ...g, [field]: value } : g)))
  }

  const handleSubmit = async () => {
    setError('')
    if (!name.trim()) {
      setError('企業名は必須です')
      return
    }
    setLoading(true)
    try {
      const payload = {
        name: name.trim(),
        description,
        industry,
        location,
        website_url: websiteUrl,
        logo_url: logoUrl,
        corporate_number: corporateNumber,
        employee_count: employeeCount ? Number(employeeCount) : 0,
        founded_year: foundedYear ? Number(foundedYear) : 0,
        average_age: averageAge ? Number(averageAge) : 0,
        female_ratio: femaleRatio ? Number(femaleRatio) : 0,
        culture,
        work_style: workStyle,
        welfare_details: welfareDetails,
        tech_stack: techStack,
        development_style: developmentStyle,
        main_business: mainBusiness,
        job_positions: jobPositions
          .filter((j) => j.title.trim())
          .map((j) => ({
            ...j,
            min_salary: j.min_salary ? Number(j.min_salary) : 0,
            max_salary: j.max_salary ? Number(j.max_salary) : 0,
          })),
        weight_profile: weights,
        graduates: graduates.filter((g) => g.graduate_name.trim()).map((g) => ({
          ...g,
          graduation_year: g.graduation_year ? Number(g.graduation_year) : 0,
        })),
      }
      const res = await fetch('/api/company-entry', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      if (!res.ok) {
        const data = await res.json()
        setError(data?.error || '送信に失敗しました')
        return
      }
      setSubmitted(true)
    } finally {
      setLoading(false)
    }
  }

  if (submitted) {
    return (
      <Box sx={{ p: 4, maxWidth: 700, mx: 'auto', textAlign: 'center' }}>
        <Alert severity="success" sx={{ mb: 3 }}>
          <Typography variant="h6" gutterBottom>
            送信が完了しました
          </Typography>
          <Typography>
            内容を確認の上、掲載審査を行います。審査完了後に公開いたします。
          </Typography>
        </Alert>
      </Box>
    )
  }

  return (
    <Box sx={{ p: { xs: 2, sm: 4 }, maxWidth: 800, mx: 'auto' }}>
      <Typography variant="h4" fontWeight="bold" gutterBottom>
        企業情報登録フォーム
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 4 }}>
        貴社の情報を入力してください。送信後、内容を確認の上、掲載審査を行います。ログイン不要でご利用いただけます。
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {/* 企業基本情報 */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            企業基本情報
          </Typography>
          <Stack spacing={2}>
            <TextField label="企業名 *" value={name} onChange={(e) => setName(e.target.value)} required />
            <TextField label="企業紹介" value={description} onChange={(e) => setDescription(e.target.value)} multiline minRows={3} />
            <TextField label="業種" value={industry} onChange={(e) => setIndustry(e.target.value)} />
            <TextField label="所在地" value={location} onChange={(e) => setLocation(e.target.value)} />
            <TextField label="公式サイトURL" value={websiteUrl} onChange={(e) => setWebsiteUrl(e.target.value)} />
            <TextField label="ロゴURL" value={logoUrl} onChange={(e) => setLogoUrl(e.target.value)} />
            <TextField label="法人番号（13桁）" value={corporateNumber} onChange={(e) => setCorporateNumber(e.target.value)} />
          </Stack>
        </CardContent>
      </Card>

      {/* 従業員情報 */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            従業員情報
          </Typography>
          <Stack spacing={2}>
            <TextField label="従業員数" value={employeeCount} onChange={(e) => setEmployeeCount(e.target.value)} type="number" />
            <TextField label="設立年" value={foundedYear} onChange={(e) => setFoundedYear(e.target.value)} type="number" />
            <TextField label="平均年齢" value={averageAge} onChange={(e) => setAverageAge(e.target.value)} type="number" />
            <TextField label="女性比率（%）" value={femaleRatio} onChange={(e) => setFemaleRatio(e.target.value)} type="number" />
          </Stack>
        </CardContent>
      </Card>

      {/* 企業文化・働き方 */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            企業文化・働き方
          </Typography>
          <Stack spacing={2}>
            <TextField label="企業文化" value={culture} onChange={(e) => setCulture(e.target.value)} multiline minRows={3} />
            <TextField
              select
              label="働き方"
              value={workStyle}
              onChange={(e) => setWorkStyle(e.target.value)}
            >
              <MenuItem value="">未設定</MenuItem>
              <MenuItem value="remote">フルリモート</MenuItem>
              <MenuItem value="hybrid">ハイブリッド</MenuItem>
              <MenuItem value="office">オフィス</MenuItem>
            </TextField>
            <TextField label="福利厚生詳細" value={welfareDetails} onChange={(e) => setWelfareDetails(e.target.value)} multiline minRows={3} />
          </Stack>
        </CardContent>
      </Card>

      {/* 技術情報・事業内容 */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            技術情報・事業内容
          </Typography>
          <Stack spacing={2}>
            <TextField label="技術スタック" value={techStack} onChange={(e) => setTechStack(e.target.value)} multiline minRows={2} placeholder="例: Go, React, MySQL, AWS" />
            <TextField label="開発スタイル" value={developmentStyle} onChange={(e) => setDevelopmentStyle(e.target.value)} placeholder="例: アジャイル、スクラム" />
            <TextField label="主要事業内容" value={mainBusiness} onChange={(e) => setMainBusiness(e.target.value)} multiline minRows={3} />
          </Stack>
        </CardContent>
      </Card>

      {/* 求人情報 */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
            <Typography variant="h6">求人情報</Typography>
            <Button startIcon={<AddIcon />} onClick={() => setJobPositions((prev) => [...prev, defaultJob()])}>
              求人を追加
            </Button>
          </Stack>
          <Stack spacing={3}>
            {jobPositions.map((job, idx) => (
              <Box key={idx} sx={{ border: '1px solid #eee', borderRadius: 1, p: 2 }}>
                <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
                  <Typography variant="subtitle2">求人 {idx + 1}</Typography>
                  {jobPositions.length > 1 && (
                    <IconButton size="small" onClick={() => setJobPositions((prev) => prev.filter((_, i) => i !== idx))}>
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  )}
                </Stack>
                <Stack spacing={2}>
                  <TextField label="職種名 *" value={job.title} onChange={(e) => updateJob(idx, 'title', e.target.value)} />
                  <TextField label="募集内容" value={job.description} onChange={(e) => updateJob(idx, 'description', e.target.value)} multiline minRows={2} />
                  <TextField label="雇用形態" value={job.employment_type} onChange={(e) => updateJob(idx, 'employment_type', e.target.value)} placeholder="例: 正社員" />
                  <TextField label="勤務地" value={job.work_location} onChange={(e) => updateJob(idx, 'work_location', e.target.value)} />
                  <TextField
                    select
                    label="リモート可否"
                    value={job.remote_option ? 'yes' : 'no'}
                    onChange={(e) => updateJob(idx, 'remote_option', e.target.value === 'yes')}
                  >
                    <MenuItem value="no">不可</MenuItem>
                    <MenuItem value="yes">可</MenuItem>
                  </TextField>
                  <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
                    <TextField label="最低年収（万円）" value={job.min_salary} onChange={(e) => updateJob(idx, 'min_salary', e.target.value)} type="number" fullWidth />
                    <TextField label="最高年収（万円）" value={job.max_salary} onChange={(e) => updateJob(idx, 'max_salary', e.target.value)} type="number" fullWidth />
                  </Stack>
                  <TextField label="必須スキル" value={job.required_skills} onChange={(e) => updateJob(idx, 'required_skills', e.target.value)} multiline minRows={2} />
                  <TextField label="歓迎スキル" value={job.preferred_skills} onChange={(e) => updateJob(idx, 'preferred_skills', e.target.value)} multiline minRows={2} />
                </Stack>
              </Box>
            ))}
          </Stack>
        </CardContent>
      </Card>

      {/* 求める人物像（WeightProfile） */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            求める人物像
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            各項目に対してどの程度重視するかをスライダーで設定してください（0: 重視しない ／ 100: 非常に重視する）
          </Typography>
          <Stack spacing={3}>
            {weightLabels.map(({ key, label }) => (
              <Box key={key}>
                <Stack direction="row" justifyContent="space-between">
                  <Typography variant="body2">{label}</Typography>
                  <Typography variant="body2" color="primary">{weights[key]}</Typography>
                </Stack>
                <Slider
                  value={weights[key]}
                  min={0}
                  max={100}
                  step={5}
                  onChange={(_, val) => setWeights((prev) => ({ ...prev, [key]: val as number }))}
                />
              </Box>
            ))}
          </Stack>
        </CardContent>
      </Card>

      {/* 卒業生就職情報 */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
            <Typography variant="h6">卒業生就職情報（任意）</Typography>
            <Button startIcon={<AddIcon />} onClick={() => setGraduates((prev) => [...prev, defaultGraduate()])}>
              追加
            </Button>
          </Stack>
          <Stack spacing={3}>
            {graduates.map((g, idx) => (
              <Box key={idx} sx={{ border: '1px solid #eee', borderRadius: 1, p: 2 }}>
                <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
                  <Typography variant="subtitle2">卒業生 {idx + 1}</Typography>
                  <IconButton size="small" onClick={() => setGraduates((prev) => prev.filter((_, i) => i !== idx))}>
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </Stack>
                <Stack spacing={2}>
                  <TextField label="氏名" value={g.graduate_name} onChange={(e) => updateGraduate(idx, 'graduate_name', e.target.value)} />
                  <TextField label="卒業年度" value={g.graduation_year} onChange={(e) => updateGraduate(idx, 'graduation_year', e.target.value)} type="number" />
                  <TextField label="学校名" value={g.school_name} onChange={(e) => updateGraduate(idx, 'school_name', e.target.value)} />
                  <TextField label="学科/専攻" value={g.department} onChange={(e) => updateGraduate(idx, 'department', e.target.value)} />
                  <TextField label="就職日 (YYYY-MM-DD)" value={g.hired_at} onChange={(e) => updateGraduate(idx, 'hired_at', e.target.value)} />
                  <TextField label="メモ" value={g.note} onChange={(e) => updateGraduate(idx, 'note', e.target.value)} multiline minRows={2} />
                </Stack>
              </Box>
            ))}
            {graduates.length === 0 && (
              <Typography variant="body2" color="text.secondary">
                卒業生就職情報を追加する場合は「追加」ボタンを押してください。
              </Typography>
            )}
          </Stack>
        </CardContent>
      </Card>

      <Divider sx={{ my: 3 }} />

      <Box sx={{ textAlign: 'center' }}>
        <Button
          variant="contained"
          size="large"
          onClick={handleSubmit}
          disabled={loading}
          sx={{ px: 6 }}
        >
          {loading ? '送信中...' : '送信する'}
        </Button>
        <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 1 }}>
          送信後、内容を確認の上、掲載審査を行います。
        </Typography>
      </Box>
    </Box>
  )
}
