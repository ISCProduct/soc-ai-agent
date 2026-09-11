import { authService } from '@/lib/auth'

export interface AdminSchool {
  id: number
  organization_id: number
  name: string
  status: string
}

export interface AdminSchoolAccess {
  restricted: boolean
  schools: AdminSchool[]
}

// 管理者が学校フィルタUIで選べる学校一覧と、無制限(システム管理者)かどうかを取得する。
export async function getAdminSchoolAccess(): Promise<AdminSchoolAccess> {
  const res = await fetch('/api/admin/me/school-access', {
    headers: authService.getAdminFetchHeaders(),
  })
  if (!res.ok) throw new Error('学校アクセス情報の取得に失敗しました')
  return res.json()
}
