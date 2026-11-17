/**
 * @jest-environment jsdom
 */
import { render, screen, waitFor, fireEvent } from '@testing-library/react'
import { ConversionFunnelCard, formatRate } from '@/components/admin/ConversionFunnelCard'

// #2039: 機能横断のコンバージョンファネル。

jest.mock('@/lib/auth', () => ({
  authService: { getStoredToken: () => 'token' },
}))

const FUNNEL = {
  funnel: {
    students: 4,
    stages: [
      { stage: 'diagnosis', reached: 3, rate_of_students: 0.75, continued_from_previous: 3, continuation_rate: 0.75 },
      { stage: 'interview', reached: 2, rate_of_students: 0.5, continued_from_previous: 2, continuation_rate: 2 / 3 },
      { stage: 'es', reached: 2, rate_of_students: 0.5, continued_from_previous: 1, continuation_rate: 0.5 },
      { stage: 'applied', reached: 2, rate_of_students: 0.5, continued_from_previous: 2, continuation_rate: 1 },
      { stage: 'offered', reached: 1, rate_of_students: 0.25, continued_from_previous: 1, continuation_rate: 0.5 },
    ],
  },
}

function mockFetch(status: number, body: unknown) {
  const fn = jest.fn().mockResolvedValue({ ok: status < 400, status, json: async () => body })
  global.fetch = fn as unknown as typeof fetch
  return fn
}

afterEach(() => {
  jest.restoreAllMocks()
})

test('割合は小数1桁のパーセントで表す', () => {
  expect(formatRate(2 / 3)).toBe('66.7%')
  expect(formatRate(0)).toBe('0%')
})

test('段階ごとの人数・割合・継続率を文字でも出し、学校を付けて取得する', async () => {
  const fetchMock = mockFetch(200, FUNNEL)
  render(<ConversionFunnelCard adminEmail="t@example.com" schoolId={3} />)

  expect(await screen.findByText('対象の学生 4 人')).toBeInTheDocument()
  expect(screen.getByRole('img', { name: '面接練習 2人、対象の学生の50%' })).toBeInTheDocument()
  expect(screen.getByText('前の段階から 66.7% が継続')).toBeInTheDocument()
  expect(String(fetchMock.mock.calls[0][0])).toContain('school_id=3')
})

test('期間を指定すると from / to を付けて取り直す', async () => {
  const fetchMock = mockFetch(200, FUNNEL)
  render(<ConversionFunnelCard adminEmail="t@example.com" schoolId={3} />)
  await screen.findByText('対象の学生 4 人')

  fireEvent.change(screen.getByLabelText('開始日'), { target: { value: '2026-04-01' } })
  await waitFor(() => expect(String(fetchMock.mock.calls.at(-1)?.[0])).toContain('from=2026-04-01'))
})

test('担当校以外は権限が無いことを伝え、再読み込みできる', async () => {
  mockFetch(403, { message: 'school access denied' })
  render(<ConversionFunnelCard adminEmail="t@example.com" schoolId={9} />)
  expect(await screen.findByText('この学校のデータを見る権限がありません')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '再読み込み' })).toBeInTheDocument()
})

test('担当校の選択が決まる前の school_id 無しの 400 はエラーにしない', async () => {
  const fetchMock = mockFetch(400, { message: 'school_id is required' })
  render(<ConversionFunnelCard adminEmail="t@example.com" schoolId={undefined} />)
  await waitFor(() => expect(fetchMock).toHaveBeenCalled())
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('対象の学生が0人なら、その旨を出す', async () => {
  mockFetch(200, { funnel: { students: 0, stages: [] } })
  render(<ConversionFunnelCard adminEmail="t@example.com" schoolId={3} />)
  expect(await screen.findByText('対象の学生がいません。学校の選択を確認してください。')).toBeInTheDocument()
})

test('学校を続けて切り替えたとき、古い条件の応答で表示を上書きしない', async () => {
  let resolveOld: (v: unknown) => void = () => {}
  const fn = jest.fn((url: string) => {
    if (url.includes('school_id=1')) {
      // 先に出したリクエストが、後から返ってくる
      return new Promise((r) => { resolveOld = r })
    }
    return Promise.resolve({ ok: true, status: 200, json: async () => ({ funnel: { students: 9, stages: FUNNEL.funnel.stages } }) })
  })
  global.fetch = fn as unknown as typeof fetch
  const { rerender } = render(<ConversionFunnelCard adminEmail="t@example.com" schoolId={1} />)
  rerender(<ConversionFunnelCard adminEmail="t@example.com" schoolId={2} />)
  expect(await screen.findByText('対象の学生 9 人')).toBeInTheDocument()

  resolveOld({ ok: true, status: 200, json: async () => FUNNEL })
  await new Promise((r) => setTimeout(r, 0))
  expect(screen.getByText('対象の学生 9 人')).toBeInTheDocument()
  expect(screen.queryByText('対象の学生 4 人')).not.toBeInTheDocument()
})
