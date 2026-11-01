/**
 * @jest-environment jsdom
 */
/**
 * 学校独自の適性検査ページで、後から届いた古い応答が現在の表示を上書きしないこと。
 *
 * 学校や検査を切り替えても進行中の要求は止まらない。応答順は保証されないので、
 * 切替前の応答が後に届くと「選択中はB校なのに一覧はA校」「B校の検査の受検状況として
 * A校の学生名が並ぶ」という取り違えが起きる。学生の氏名が別の学校の画面に出るため、
 * 見た目の乱れではなく個人情報の混線になる。
 *
 * 注意: 「古い応答が反映されていない」ことは waitFor では確かめられない。
 * 否定のアサーションは最初の判定で通ってしまい、応答が適用される前に成功する。
 * settle() で応答を act 内で解決し、反映を終わらせてから判定する。
 */
import { act, fireEvent, render, screen } from '@testing-library/react'
import PageContent from '@/app/admin/assessments/page-content'
import { listSchoolTemplates, listStudentStatuses } from '@/lib/admin/assessment-templates'

jest.mock('@/lib/admin/assessment-templates', () => ({
  ...jest.requireActual('@/lib/admin/assessment-templates'),
  listSchoolTemplates: jest.fn(),
  listStudentStatuses: jest.fn(),
}))

// 学校の選択は外部fetchに依存するので、テストから学校を切り替えられる形に差し替える。
jest.mock('@/components/admin/SchoolFilterSelect', () => ({
  SchoolFilterSelect: ({ onChange }: { onChange: (id: number) => void }) => (
    <div>
      <button onClick={() => onChange(1)}>A校を選ぶ</button>
      <button onClick={() => onChange(2)}>B校を選ぶ</button>
    </div>
  ),
}))

const mockTemplates = listSchoolTemplates as jest.MockedFunction<typeof listSchoolTemplates>
const mockStatuses = listStudentStatuses as jest.MockedFunction<typeof listStudentStatuses>

type TemplateList = Awaited<ReturnType<typeof listSchoolTemplates>>
type StatusView = Awaited<ReturnType<typeof listStudentStatuses>>

/** 外から解決できる Promise。応答の到着順をテストで作るために使う。 */
function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => { resolve = r })
  return { promise, resolve }
}

/** 応答を解決し、コンポーネントへの反映を終わらせる。 */
async function settle<T>(d: { promise: Promise<T>; resolve: (v: T) => void }, value: T) {
  await act(async () => {
    d.resolve(value)
    await d.promise
  })
}

const click = async (el: HTMLElement) => { await act(async () => { fireEvent.click(el) }) }

const template = (id: number, title: string) => ({
  template_id: id, title, description: '', question_count: 1, created_at: '2026-01-01T00:00:00Z',
})
const statusList = (id: number, title: string, studentName: string): StatusView => ({
  template_id: id, title, students: [{
    student_id: 1, name: studentName, status: 'submitted', overdue: false,
    answered_count: 1, question_count: 1, top_categories: [],
  }],
})

describe('学校独自の適性検査: 後着応答の上書き', () => {
  beforeEach(() => jest.clearAllMocks())

  it('A校の一覧応答がB校より後に届いても、B校の一覧を上書きしない', async () => {
    const a = deferred<TemplateList>()
    const b = deferred<TemplateList>()
    mockTemplates.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)

    render(<PageContent />)
    await click(screen.getByRole('button', { name: 'A校を選ぶ' }))
    await click(screen.getByRole('button', { name: 'B校を選ぶ' }))

    await settle(b, { templates: [template(20, 'B校の検査')], categories: [] })
    expect(screen.getByText('B校の検査')).toBeInTheDocument()

    await settle(a, { templates: [template(10, 'A校の検査')], categories: [] })
    expect(screen.queryByText('A校の検査')).not.toBeInTheDocument()
    expect(screen.getByText('B校の検査')).toBeInTheDocument()
  })

  it('A校の一覧応答が先に届いても、B校の読み込み中にA校の一覧や空状態を見せない', async () => {
    const a = deferred<TemplateList>()
    const b = deferred<TemplateList>()
    mockTemplates.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)

    render(<PageContent />)
    await click(screen.getByRole('button', { name: 'A校を選ぶ' }))
    await click(screen.getByRole('button', { name: 'B校を選ぶ' }))

    await settle(a, { templates: [template(10, 'A校の検査')], categories: [] })
    expect(screen.queryByText('A校の検査')).not.toBeInTheDocument()
    // 古い応答で Loading を解除すると、B校の読み込み中に空状態が一瞬出る
    expect(screen.getByLabelText('検査を読み込み中')).toBeInTheDocument()
    expect(screen.queryByText('学校独自の検査はまだありません')).not.toBeInTheDocument()

    await settle(b, { templates: [template(20, 'B校の検査')], categories: [] })
    expect(screen.getByText('B校の検査')).toBeInTheDocument()
  })

  it('検査Aの受検状況応答が検査Bより後に届いても、Bの受検状況を上書きしない', async () => {
    mockTemplates.mockResolvedValue({
      templates: [template(10, '検査A'), template(20, '検査B')], categories: [],
    })
    const a = deferred<StatusView>()
    const b = deferred<StatusView>()
    mockStatuses.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)

    render(<PageContent />)
    await click(screen.getByRole('button', { name: 'A校を選ぶ' }))
    const buttons = await screen.findAllByRole('button', { name: '受検状況を見る' })
    await click(buttons[0]) // 検査A
    await click(buttons[1]) // 検査B

    await settle(b, statusList(20, '検査B', 'Bを受けた学生'))
    expect(screen.getByText('Bを受けた学生')).toBeInTheDocument()

    await settle(a, statusList(10, '検査A', 'Aを受けた学生'))
    expect(screen.queryByText('Aを受けた学生')).not.toBeInTheDocument()
    expect(screen.getByText('Bを受けた学生')).toBeInTheDocument()
  })

  it('学校を切り替えた後に届いたA校の受検状況を、B校の検査の受検状況として見せない', async () => {
    mockTemplates.mockResolvedValue({ templates: [template(10, '検査')], categories: [] })
    const a = deferred<StatusView>()
    const b = deferred<StatusView>()
    mockStatuses.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise)

    render(<PageContent />)
    await click(screen.getByRole('button', { name: 'A校を選ぶ' }))
    await click(await screen.findByRole('button', { name: '受検状況を見る' }))

    // A校の応答を待たずにB校へ移り、そこで検査を選ぶ
    await click(screen.getByRole('button', { name: 'B校を選ぶ' }))
    await click(await screen.findByRole('button', { name: '受検状況を見る' }))

    await settle(a, statusList(10, '検査', 'A校の学生'))
    expect(screen.queryByText('A校の学生')).not.toBeInTheDocument()

    await settle(b, statusList(10, '検査', 'B校の学生'))
    expect(screen.getByText('B校の学生')).toBeInTheDocument()
  })
})
