/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConsultationThread } from '@/components/consultation/ConsultationThread'
import type { ConsultationThread as Thread } from '@/lib/consultation'

// #2028: 教員と学生の相談のスレッド。

const THREAD: Thread = {
  guidance_id: 1,
  student_user_id: 10,
  kind: 'resume',
  message: '履歴書を見直しましょう',
  created_at: '2026-11-13T01:00:00Z',
  messages: [
    { id: 1, guidance_id: 1, author_role: 'student', body: 'どこを直せばよいですか？', read_at: null, created_at: '2026-11-13T02:00:00Z' },
  ],
}

test('案内と、その後のやり取りを古い順に出し、返信を送ると表示を更新する', async () => {
  const send = jest.fn(async (body: string) => ({
    ...THREAD,
    messages: [...THREAD.messages, { id: 2, guidance_id: 1, author_role: 'teacher' as const, body, read_at: null, created_at: '2026-11-13T03:00:00Z' }],
  }))
  const onChanged = jest.fn()
  render(<ConsultationThread me="teacher" load={async () => THREAD} send={send} onChanged={onChanged} />)

  expect(await screen.findByText('履歴書を見直しましょう')).toBeInTheDocument()
  expect(screen.getByText('どこを直せばよいですか？')).toBeInTheDocument()

  fireEvent.change(screen.getByLabelText('学生への返信'), { target: { value: '自己PRを具体的にしましょう' } })
  fireEvent.click(screen.getByRole('button', { name: '送信する' }))
  expect(await screen.findByText('自己PRを具体的にしましょう')).toBeInTheDocument()
  expect(send).toHaveBeenCalledWith('自己PRを具体的にしましょう')
  expect(onChanged).toHaveBeenCalled()
})

test('送信に失敗しても入力は残し、エラーを出す', async () => {
  const send = jest.fn(async () => { throw new Error('返信を送れませんでした') })
  render(<ConsultationThread me="student" load={async () => THREAD} send={send} />)
  await screen.findByText('履歴書を見直しましょう')
  fireEvent.change(screen.getByLabelText('先生への返信'), { target: { value: '確認します' } })
  fireEvent.click(screen.getByRole('button', { name: '送信する' }))
  expect(await screen.findByText('返信を送れませんでした')).toBeInTheDocument()
  expect(screen.getByLabelText('先生への返信')).toHaveValue('確認します')
})

test('1000文字を超えると送信できない', async () => {
  render(<ConsultationThread me="student" load={async () => THREAD} send={jest.fn()} />)
  await screen.findByText('履歴書を見直しましょう')
  fireEvent.change(screen.getByLabelText('先生への返信'), { target: { value: 'あ'.repeat(1001) } })
  expect(screen.getByRole('button', { name: '送信する' })).toBeDisabled()
})

test('呼び出し側が毎回新しい onChanged を渡しても、読み込みを繰り返さない', async () => {
  const load = jest.fn(async () => THREAD)
  const { rerender } = render(<ConsultationThread me="teacher" load={load} send={jest.fn()} onChanged={() => {}} />)
  await screen.findByText('履歴書を見直しましょう')
  rerender(<ConsultationThread me="teacher" load={load} send={jest.fn()} onChanged={() => {}} />)
  await waitFor(() => expect(load).toHaveBeenCalledTimes(1))
})

test('学生の質問のスレッドは、本文を最初の発言と重ねて出さない', async () => {
  const question: Thread = { ...THREAD, kind: 'question', message: '面接の練習量は？', messages: [{ id: 1, guidance_id: 1, author_role: 'student', body: '面接の練習量は？', read_at: null, created_at: '2026-11-13T02:00:00Z' }] }
  render(<ConsultationThread me="student" load={async () => question} send={jest.fn()} />)
  expect(await screen.findAllByText('面接の練習量は？')).toHaveLength(1)
})
