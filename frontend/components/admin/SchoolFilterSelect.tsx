'use client'

import { useEffect, useState } from 'react'
import { MenuItem, TextField } from '@mui/material'
import { getAdminSchoolAccess, type AdminSchool } from '@/lib/admin-school-access'

interface SchoolFilterSelectProps {
  value: number | undefined
  onChange: (schoolId: number | undefined) => void
  /**
   * 学校アクセスの解決が終わったときに一度だけ呼ばれる(#1157)。
   * schoolScope が必須のAPIは、解決前に呼ぶと school_id 未指定で 400 になるため、
   * 呼び出し側が初回ロードを待たせるのに使う。error は取得に失敗した場合のみ。
   */
  onResolved?: (error?: string) => void
}

// 学校ごとの絞り込みUI(#798)。
// 担当校が1校のみの管理者には表示せず自動適用、複数校ならドロップダウン、
// システム管理者(未割当)には「全学校」を含むドロップダウンを表示する。
export function SchoolFilterSelect({ value, onChange, onResolved }: SchoolFilterSelectProps) {
  const [schools, setSchools] = useState<AdminSchool[]>([])
  const [restricted, setRestricted] = useState(false)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    let cancelled = false
    let failure: string | undefined
    getAdminSchoolAccess()
      .then((access) => {
        if (cancelled) return
        setSchools(access.schools)
        setRestricted(access.restricted)
        if (access.restricted && access.schools.length > 0) {
          onChange(access.schools[0].id)
        }
      })
      .catch((e: unknown) => {
        failure = e instanceof Error ? e.message : '学校アクセス情報の取得に失敗しました'
      })
      .finally(() => {
        if (cancelled) return
        setLoaded(true)
        onResolved?.(failure)
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  if (!loaded || (restricted && schools.length <= 1)) {
    return null
  }

  return (
    <TextField
      select
      label="学校"
      value={value ?? ''}
      onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
      size="small"
      sx={{ minWidth: 200 }}
    >
      {!restricted && <MenuItem value="">全学校</MenuItem>}
      {schools.map((school) => (
        <MenuItem key={school.id} value={school.id}>
          {school.name}
        </MenuItem>
      ))}
    </TextField>
  )
}
