'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Box, Typography } from '@mui/material'
import { getResultsPathOrChat } from '@/lib/results-navigation'

export default function CompanyResultsRedirectPage() {
  const router = useRouter()

  useEffect(() => {
    router.replace(getResultsPathOrChat())
  }, [router])

  return (
    <Box sx={{ p: 4 }}>
      <Typography variant="body2" color="text.secondary">
        結果ページへ移動中です...
      </Typography>
    </Box>
  )
}
