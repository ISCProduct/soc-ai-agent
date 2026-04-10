'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Box } from '@mui/material'
import { AnalysisSidebar } from '@/components/analysis-sidebar'
import { MuiChat } from '@/components/mui-chat'
import { authService, User } from '@/lib/auth'

export default function Home() {
  const router = useRouter()
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const storedUser = authService.getStoredUser()
    if (!storedUser) {
      router.replace('/login')
      return
    }
    setUser(storedUser)
    setLoading(false)
  }, [router])

  const handleLogout = () => {
    authService.logout()
    setUser(null)
    router.push('/login')
  }

  if (loading || !user) {
    return null
  }

  return (
    <Box sx={{ display: 'flex', height: '100vh', overflow: 'hidden' }}>
      <AnalysisSidebar user={user} onLogout={handleLogout} />
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          height: '100vh',
        }}
      >
        <MuiChat />
      </Box>
    </Box>
  )
}
