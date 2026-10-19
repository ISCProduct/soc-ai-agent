import { nextCompanyAction } from '@/lib/company/next-action'
import { applicationMark, jobPublishMark } from '@/lib/company/marks'

describe('nextCompanyAction', () => {
  it('求人も応募も無いときは求人作成が最初の行動になる', () => {
    const action = nextCompanyAction({ publishedJobs: 0, pendingApplications: 0 })
    expect(action.title).toBe('求人を公開すると、学生からの応募が届きます')
    expect(action.primary.label).toBe('求人を作成する')
    expect(action.secondary?.label).toBe('先に学生を探す')
  })

  it('未対応の応募があるときは応募確認が最初の行動になる', () => {
    const action = nextCompanyAction({ publishedJobs: 2, pendingApplications: 3 })
    expect(action.primary).toEqual({
      label: '応募を確認する',
      href: '/company-portal/applications',
    })
  })

  it('求人が公開されていて未対応が無いときは学生を探す', () => {
    const action = nextCompanyAction({ publishedJobs: 1, pendingApplications: 0 })
    expect(action.primary.label).toBe('学生を探す')
  })
})

describe('状態の字形', () => {
  it('公開と下書きを色ではなく字形で分ける', () => {
    expect(jobPublishMark(true)).toMatchObject({ mark: '●', label: '公開中' })
    expect(jobPublishMark(false)).toMatchObject({ mark: '○', label: '下書き' })
  })

  it('応募の段階は字形と文字を持つ', () => {
    expect(applicationMark('document_screening')).toMatchObject({ mark: '◐', label: '書類選考中' })
    expect(applicationMark('offered')).toMatchObject({ mark: '✓', label: '内定' })
    expect(applicationMark('rejected')).toMatchObject({ mark: '×', label: '不採用' })
  })
})
