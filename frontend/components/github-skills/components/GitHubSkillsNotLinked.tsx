import { Alert, Box, Button, CircularProgress, Paper, Typography } from '@mui/material'
import GitHubIcon from '@mui/icons-material/GitHub'

type GitHubSkillsNotLinkedProps = {
  connecting: boolean
  onConnect: () => void
}

export function GitHubSkillsNotLinked({ connecting, onConnect }: GitHubSkillsNotLinkedProps) {
  return (
    <Paper sx={{ p: 3, mt: 3, bgcolor: '#0f172a', borderRadius: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
        <GitHubIcon sx={{ color: '#94a3b8' }} />
        <Typography variant="h6" sx={{ fontWeight: 700, color: '#f1f5f9' }}>
          GitHub スキル分析
        </Typography>
      </Box>
      <Alert
        severity="info"
        sx={{ mb: 2, bgcolor: '#1e293b', color: '#94a3b8', '& .MuiAlert-icon': { color: '#4FC3F7' } }}
      >
        GitHubアカウントが連携されていません。GitHubでログインするとスキル分析が自動的に生成されます。
      </Alert>
      <Button
        variant="contained"
        startIcon={connecting ? <CircularProgress size={16} /> : <GitHubIcon />}
        onClick={onConnect}
        disabled={connecting}
        sx={{ bgcolor: '#24292e', '&:hover': { bgcolor: '#444d56' } }}
      >
        {connecting ? '連携中...' : 'GitHubと連携する'}
      </Button>
    </Paper>
  )
}
