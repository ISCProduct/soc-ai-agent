'use client'

import { FormControl, FormControlLabel, FormLabel, Radio, RadioGroup } from '@mui/material'
import { useStudentTheme } from '@/components/mui-provider'
import { STUDENT_THEME_LABELS, type StudentThemeMode } from '@/lib/student-theme'

export function StudentThemeToggle() {
  const { mode, setMode } = useStudentTheme()
  return (
    <FormControl component="fieldset" variant="standard">
      <FormLabel component="legend">画面の見え方</FormLabel>
      <RadioGroup
        value={mode}
        onChange={(e) => setMode(e.target.value as StudentThemeMode)}
      >
        {(Object.keys(STUDENT_THEME_LABELS) as StudentThemeMode[]).map((key) => (
          <FormControlLabel
            key={key}
            value={key}
            control={<Radio />}
            label={STUDENT_THEME_LABELS[key]}
          />
        ))}
      </RadioGroup>
    </FormControl>
  )
}
