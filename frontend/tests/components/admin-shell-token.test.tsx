/** @jest-environment jsdom */

import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { AdminShell } from '@/components/admin/AdminShell'
import { authService } from '@/lib/auth'
import { getAdminSchoolAccess } from '@/lib/admin/school-access'

let mockPathname = '/admin'
jest.mock('next/navigation', () => ({
  usePathname: () => mockPathname,
}))

jest.mock('@/lib/admin/school-access', () => ({
  getAdminSchoolAccess: jest.fn().mockResolvedValue({ restricted: true, schools: [] }),
}))

const getStoredToken = authService.getStoredToken as jest.Mock
const ensureFreshAdminToken = authService.ensureFreshAdminToken as jest.Mock

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredToken: jest.fn(),
    ensureFreshAdminToken: jest.fn(),
  },
}))

describe('AdminShell の管理者トークン確認', () => {
  beforeEach(() => {
    ;(getAdminSchoolAccess as jest.Mock).mockResolvedValue({ restricted: true, schools: [] })
  })

  afterEach(() => {
    mockPathname = '/admin'
    jest.resetAllMocks()
  })

  it('期限が近いでなければ本文を出す', async () => {
    getStoredToken.mockReturnValue(`${Math.floor(Date.now() / 1000)}.current`)
    render(<AdminShell><p>管理の本文</p></AdminShell>)
    expect(await screen.findByText('管理の本文')).toBeInTheDocument()
    expect(ensureFreshAdminToken).not.toHaveBeenCalled()
  })

  it('期限が近い間は本文を出さず、更新できてから出す', async () => {
    getStoredToken.mockReturnValue(`${Math.floor(Date.now() / 1000) - 8 * 24 * 60 * 60}.old`)
    let resolveRefresh: () => void = () => {}
    ensureFreshAdminToken.mockReturnValue(new Promise<void>((resolve) => {
      resolveRefresh = resolve
    }))
    render(<AdminShell><p>管理の本文</p></AdminShell>)
    expect(screen.getByRole('status')).toHaveTextContent('管理者セッションを確認しています')
    expect(screen.queryByText('管理の本文')).not.toBeInTheDocument()
    resolveRefresh()
    expect(await screen.findByText('管理の本文')).toBeInTheDocument()
  })

  it('更新に失敗したら理由と再試行を出し、本文は出さない', async () => {
    getStoredToken.mockReturnValue(null)
    ensureFreshAdminToken.mockRejectedValue(new Error('down'))
    render(<AdminShell><p>管理の本文</p></AdminShell>)
    expect(await screen.findByText(/管理画面の操作が拒否されます/)).toBeInTheDocument()
    expect(screen.queryByText('管理の本文')).not.toBeInTheDocument()
    ensureFreshAdminToken.mockResolvedValue(undefined)
    fireEvent.click(screen.getByRole('button', { name: 'もう一度試す' }))
    await waitFor(() => {
      expect(screen.getByText('管理の本文')).toBeInTheDocument()
    })
  })
  it('表示中の画面遷移で期限が近づいても、本文を隠さず背景で取り直す', async () => {
    getStoredToken.mockReturnValue(`${Math.floor(Date.now() / 1000)}.current`)
    const { rerender } = render(<AdminShell><p>管理の本文</p></AdminShell>)
    expect(await screen.findByText('管理の本文')).toBeInTheDocument()

    getStoredToken.mockReturnValue(`${Math.floor(Date.now() / 1000) - 8 * 24 * 60 * 60}.old`)
    ensureFreshAdminToken.mockReturnValue(new Promise<void>(() => {}))
    mockPathname = '/admin/other'
    rerender(<AdminShell><p>管理の本文</p></AdminShell>)

    expect(ensureFreshAdminToken).toHaveBeenCalledTimes(1)
    expect(screen.getByText('管理の本文')).toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('表示中の背景更新が失敗しても、本文はエラー画面に切り替えない', async () => {
    getStoredToken.mockReturnValue(`${Math.floor(Date.now() / 1000)}.current`)
    const { rerender } = render(<AdminShell><p>管理の本文</p></AdminShell>)
    expect(await screen.findByText('管理の本文')).toBeInTheDocument()

    getStoredToken.mockReturnValue(`${Math.floor(Date.now() / 1000) - 8 * 24 * 60 * 60}.old`)
    ensureFreshAdminToken.mockRejectedValue(new Error('down'))
    mockPathname = '/admin/other'
    rerender(<AdminShell><p>管理の本文</p></AdminShell>)

    await waitFor(() => expect(ensureFreshAdminToken).toHaveBeenCalledTimes(1))
    expect(screen.getByText('管理の本文')).toBeInTheDocument()
  })
})
