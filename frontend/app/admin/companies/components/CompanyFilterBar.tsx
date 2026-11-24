'use client'

import type { Dispatch, SetStateAction } from 'react'
import {
  Box,
  Button,
  Checkbox,
  FormControl,
  FormControlLabel,
  InputAdornment,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import { SchoolFilterSelect } from '@/components/admin/SchoolFilterSelect'
import { INDUSTRY_UNSET, type FilterReadiness, type FilterStatus } from '../company-list-model'

type Props = {
  searchInput: string
  setSearchInput: Dispatch<SetStateAction<string>>
  filterIndustry: string
  setFilterIndustry: Dispatch<SetStateAction<string>>
  industries: string[]
  schoolId: number | undefined
  setSchoolId: Dispatch<SetStateAction<number | undefined>>
  hasActiveFilters: boolean
  resetFilters: () => void
  filterStatus: FilterStatus
  setFilterStatus: Dispatch<SetStateAction<FilterStatus>>
  filterReadiness: FilterReadiness
  setFilterReadiness: Dispatch<SetStateAction<FilterReadiness>>
  groupByIndustry: boolean
  setGroupByIndustry: Dispatch<SetStateAction<boolean>>
  setPage: Dispatch<SetStateAction<number>>
}

/** 企業一覧の検索・絞り込み（企業名・業界・学校・公開の状態・情報のそろい具合・業界別表示）。 */
export function CompanyFilterBar({
  searchInput,
  setSearchInput,
  filterIndustry,
  setFilterIndustry,
  industries,
  schoolId,
  setSchoolId,
  hasActiveFilters,
  resetFilters,
  filterStatus,
  setFilterStatus,
  filterReadiness,
  setFilterReadiness,
  groupByIndustry,
  setGroupByIndustry,
  setPage,
}: Props) {
  return (
    <Box
      sx={{
        px: 2.5,
        py: 1.75,
        borderBottom: '1px solid',
        borderColor: 'divider',
        bgcolor: 'grey.50',
      }}
    >
      <Stack spacing={1.5}>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} alignItems={{ sm: 'center' }}>
          <TextField
            size="small"
            placeholder="企業名で探す"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            sx={{ minWidth: { sm: 240 }, bgcolor: 'background.paper' }}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <SearchIcon fontSize="small" color="action" />
                </InputAdornment>
              ),
            }}
          />
          <FormControl size="small" sx={{ minWidth: { sm: 220 }, bgcolor: 'background.paper' }}>
            <InputLabel id="filter-industry-label">業界</InputLabel>
            <Select
              labelId="filter-industry-label"
              label="業界"
              value={filterIndustry}
              onChange={(e) => {
                setFilterIndustry(e.target.value)
                setPage(1)
              }}
            >
              <MenuItem value="">すべての業界</MenuItem>
              <MenuItem value={INDUSTRY_UNSET}>業界未設定</MenuItem>
              {industries.map((industry) => (
                <MenuItem key={industry} value={industry}>
                  {industry}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          <SchoolFilterSelect value={schoolId} onChange={(id) => { setSchoolId(id); setPage(1) }} />
          {hasActiveFilters && (
            <Button size="small" variant="text" onClick={resetFilters} sx={{ alignSelf: { xs: 'flex-start', sm: 'center' } }}>
              絞り込みを解除
            </Button>
          )}
        </Stack>
        <Stack
          direction={{ xs: 'column', md: 'row' }}
          spacing={1.5}
          alignItems={{ md: 'center' }}
          flexWrap="wrap"
          useFlexGap
        >
          <Box>
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
              公開の状態
            </Typography>
            <ToggleButtonGroup
              exclusive
              size="small"
              value={filterStatus}
              onChange={(_, v: FilterStatus | null) => {
                if (!v) return
                setFilterStatus(v)
                setPage(1)
              }}
              sx={{
                bgcolor: 'background.paper',
                '& .MuiToggleButton-root': {
                  px: 1.5,
                  textTransform: 'none',
                  borderColor: 'divider',
                },
              }}
            >
              <ToggleButton value="all">すべて</ToggleButton>
              <ToggleButton value="draft">公開前</ToggleButton>
              <ToggleButton value="published">学生に公開中</ToggleButton>
            </ToggleButtonGroup>
          </Box>
          <Box>
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
              情報のそろい具合
            </Typography>
            <ToggleButtonGroup
              exclusive
              size="small"
              value={filterReadiness}
              onChange={(_, v: FilterReadiness | null) => {
                if (!v) return
                setFilterReadiness(v)
                setPage(1)
              }}
              sx={{
                bgcolor: 'background.paper',
                '& .MuiToggleButton-root': {
                  px: 1.5,
                  textTransform: 'none',
                  borderColor: 'divider',
                },
              }}
            >
              <ToggleButton value="all">すべて</ToggleButton>
              <ToggleButton value="missing">足りない</ToggleButton>
              <ToggleButton value="ready">そろっている</ToggleButton>
            </ToggleButtonGroup>
          </Box>
          <FormControlLabel
            control={
              <Checkbox
                checked={groupByIndustry}
                onChange={(e) => {
                  setGroupByIndustry(e.target.checked)
                  setPage(1)
                }}
                size="small"
              />
            }
            label="業界別に分けて表示"
            sx={{ ml: 0, mr: 0, alignSelf: { xs: 'flex-start', md: 'flex-end' }, pb: { md: 0.25 } }}
          />
        </Stack>
      </Stack>
    </Box>
  )
}
