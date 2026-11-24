'use client'

import type { Dispatch, SetStateAction } from 'react'
import { Box, Button, Checkbox, CircularProgress, FormControlLabel, Stack, Typography } from '@mui/material'
import type { Company } from '../company-list-model'

type Props = {
  selectableOnPage: Company[]
  selectedCount: number
  selectedSelectableCount: number
  allSelectableSelected: boolean
  toggleSelectAllSelectable: () => void
  setSelectedIds: Dispatch<SetStateAction<number[]>>
  handleBulkPublish: () => void
  busy: boolean
  bulkPublishing: boolean
}

/** このページで公開できる企業をまとめて選んで公開するバー。対象がなければ案内文だけ出す。 */
export function BulkPublishBar({
  selectableOnPage,
  selectedCount,
  selectedSelectableCount,
  allSelectableSelected,
  toggleSelectAllSelectable,
  setSelectedIds,
  handleBulkPublish,
  busy,
  bulkPublishing,
}: Props) {
  if (selectableOnPage.length === 0) {
    return (
      <Box
        sx={{
          px: 2.5,
          py: 1.25,
          borderBottom: '1px solid',
          borderColor: 'divider',
          bgcolor: 'grey.50',
        }}
      >
        <Typography variant="body2" color="text.secondary">
          このページに、まとめて公開できる企業はありません（公開前かつ情報がそろっている企業だけ選べます）。
        </Typography>
      </Box>
    )
  }
  return (
    <Box
      sx={{
        px: 2.5,
        py: 1.25,
        borderBottom: '1px solid',
        borderColor: 'divider',
        bgcolor: selectedCount > 0 ? 'action.selected' : 'grey.50',
      }}
    >
      <Stack
        direction={{ xs: 'column', sm: 'row' }}
        spacing={1.5}
        alignItems={{ sm: 'center' }}
        justifyContent="space-between"
      >
        <FormControlLabel
          control={
            <Checkbox
              checked={allSelectableSelected}
              indeterminate={selectedSelectableCount > 0 && !allSelectableSelected}
              onChange={toggleSelectAllSelectable}
              disabled={busy}
            />
          }
          label={
            selectedSelectableCount > 0
              ? `公開できる企業 ${selectedSelectableCount} 社を選択中`
              : `このページで公開できる企業（${selectableOnPage.length} 社）を選択`
          }
        />
        <Stack direction="row" spacing={1}>
          {selectedCount > 0 && (
            <Button size="small" variant="text" disabled={busy} onClick={() => setSelectedIds([])}>
              選択解除
            </Button>
          )}
          <Button
            size="small"
            variant="contained"
            color="success"
            disabled={busy || selectedSelectableCount === 0}
            onClick={() => handleBulkPublish()}
            startIcon={bulkPublishing ? <CircularProgress size={14} color="inherit" /> : null}
            disableElevation
          >
            {bulkPublishing
              ? '公開中…'
              : selectedSelectableCount > 0
                ? `選択した ${selectedSelectableCount} 社を学生に公開`
                : '選択して学生に公開'}
          </Button>
        </Stack>
      </Stack>
    </Box>
  )
}
