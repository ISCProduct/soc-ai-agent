/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { TemplateForm } from '@/app/admin/assessments/template-form'
import * as api from '@/lib/admin/assessment-templates'

jest.mock('@/lib/auth', () => ({ authService: { getAdminFetchHeaders: () => ({}) } }))
jest.mock('@/lib/admin/assessment-templates', () => ({
  ...jest.requireActual('@/lib/admin/assessment-templates'),
  createSchoolTemplate: jest.fn(),
}))

const createMock = api.createSchoolTemplate as jest.Mock

const CATEGORIES = ['技術志向', 'チームワーク志向']

function setup() {
  const onCreated = jest.fn()
  const onCancel = jest.fn()
  render(<TemplateForm schoolId={10} categories={CATEGORIES} onCreated={onCreated} onCancel={onCancel} />)
  return { onCreated, onCancel }
}

// 最初の問題だけ埋め、残りの空の問題を消して「1問だけ」の有効な下書きにする
function fillOneQuestion() {
  fireEvent.change(screen.getByLabelText(/タイトル/), { target: { value: 'A校の検査' } })
  fireEvent.click(screen.getByRole('button', { name: '3問目を削除' }))
  fireEvent.click(screen.getByRole('button', { name: '2問目を削除' }))
  fireEvent.change(screen.getByLabelText(/問題文/), { target: { value: '新しい技術を試すのが好きだ' } })
  fireEvent.mouseDown(screen.getByLabelText(/測る傾向/))
  fireEvent.click(screen.getByRole('option', { name: '技術志向' }))
}

describe('TemplateForm', () => {
  beforeEach(() => createMock.mockReset())

  it('最上部で「おすすめ企業の計算には使われません」と明言する', () => {
    setup()
    expect(screen.getByTestId('template-form-notice')).toHaveTextContent('おすすめ企業の計算には使われません')
  })

  it('カテゴリの選択肢はサーバーが返した正典だけ（自由入力させない）', () => {
    setup()
    fireEvent.mouseDown(screen.getAllByLabelText(/測る傾向/)[0])
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual(CATEGORIES)
  })

  it('入力が足りなければ送信せずに理由を出す', () => {
    setup()
    fireEvent.click(screen.getByRole('button', { name: '検査を作成する' }))
    expect(screen.getByTestId('template-form-error')).toHaveTextContent('タイトルを入力してください')
    expect(createMock).not.toHaveBeenCalled()
  })

  it('作成に成功したら onCreated を呼ぶ', async () => {
    createMock.mockResolvedValue({})
    const { onCreated } = setup()
    fillOneQuestion()
    fireEvent.click(screen.getByRole('button', { name: '検査を作成する' }))
    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1))
    expect(createMock).toHaveBeenCalledWith(
      10,
      expect.objectContaining({
        title: 'A校の検査',
        questions: [{ body: '新しい技術を試すのが好きだ', weight_category: '技術志向', reverse_scored: false }],
      }),
    )
  })

  it('入力不正(400)はサーバーのメッセージを出し、それ以外は入力が残っていることを伝える', async () => {
    const { onCreated } = setup()
    fillOneQuestion()

    createMock.mockRejectedValueOnce(new api.AdminAssessmentError(400, '受検期限は現在より後の日時にしてください'))
    fireEvent.click(screen.getByRole('button', { name: '検査を作成する' }))
    expect(await screen.findByTestId('template-form-error')).toHaveTextContent('受検期限は現在より後の日時にしてください')

    createMock.mockRejectedValueOnce(new api.AdminAssessmentError(500, 'boom'))
    fireEvent.click(screen.getByRole('button', { name: '検査を作成する' }))
    await waitFor(() => expect(screen.getByTestId('template-form-error')).toHaveTextContent('入力した内容は残っています'))
    expect(onCreated).not.toHaveBeenCalled()
  })
})
