import { Suspense } from 'react'
import { LoginContent } from '@/components/login-content'
import { PageLoading } from '@/components/common/PageLoading'

export default function Login() {
  return (
    <Suspense fallback={<PageLoading message="ログイン画面を準備しています..." />}>
      <LoginContent />
    </Suspense>
  )
}
