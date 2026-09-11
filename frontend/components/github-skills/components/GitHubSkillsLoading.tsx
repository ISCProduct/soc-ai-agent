import { Box, CircularProgress } from '@mui/material'

export function GitHubSkillsLoading() {
  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
      <CircularProgress size={32} />
    </Box>
  )
}
