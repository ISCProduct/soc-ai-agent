/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import CompanyEntryPage from '@/app/company-entry/page'

describe('CompanyEntryPage', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('担当者メールと同意なしでは送信できない', async () => {
    render(<CompanyEntryPage />)

    fireEvent.change(screen.getByLabelText(/企業名/), { target: { value: 'テスト株式会社' } })
    fireEvent.click(screen.getByRole('button', { name: '送信する' }))

    expect(await screen.findByText('担当者メールアドレスは必須です')).toBeInTheDocument()
    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('同意チェック後に API へ送信する', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({ message: 'ok' }),
    })

    render(<CompanyEntryPage />)

    fireEvent.change(screen.getByLabelText(/企業名/), { target: { value: 'テスト株式会社' } })
    fireEvent.change(screen.getByLabelText(/担当者メールアドレス/), {
      target: { value: 'hr@example.com' },
    })
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(screen.getByRole('button', { name: '送信する' }))

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        '/api/company-entry',
        expect.objectContaining({ method: 'POST' }),
      )
    })

    const body = JSON.parse((global.fetch as jest.Mock).mock.calls[0][1].body as string)
    expect(body.contact_email).toBe('hr@example.com')
    expect(body.privacy_consent).toBe(true)
    expect(await screen.findByText(/会員登録のご案内をお送りしました/)).toBeInTheDocument()
  })
})
