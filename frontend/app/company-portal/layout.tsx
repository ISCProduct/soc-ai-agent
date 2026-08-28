'use client'

import { ReactNode } from 'react'
import { Box, Container, Typography } from '@mui/material'

export default function CompanyPortalLayout({ children }: { children: ReactNode }) {
  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#f5f7fb' }}>
      <Box sx={{ bgcolor: '#0d47a1', color: '#fff', py: 2 }}>
        <Container maxWidth="md">
          <Typography variant="h6" component="div">
            企業ポータル
          </Typography>
        </Container>
      </Box>
      <Container maxWidth="md" sx={{ py: 4 }}>
        {children}
      </Container>
    </Box>
  )
}
