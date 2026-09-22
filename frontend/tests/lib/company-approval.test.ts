import { approvalChipState, canToggleApproval } from '@/lib/admin/company-approval'

describe('学校別の企業承認チップの状態 (#1452)', () => {
  const cases: Array<{
    name: string
    approved: Set<number> | null
    companyId: number
    want: ReturnType<typeof approvalChipState>
  }> = [
    { name: '承認済みなら approved', approved: new Set([1, 2]), companyId: 1, want: 'approved' },
    { name: '含まれなければ unapproved', approved: new Set([1, 2]), companyId: 9, want: 'unapproved' },
    { name: '1社も承認していない学校は unapproved', approved: new Set<number>(), companyId: 1, want: 'unapproved' },
    { name: '取得できていない場合は unknown（unapproved と区別する）', approved: null, companyId: 1, want: 'unknown' },
  ]

  for (const c of cases) {
    it(c.name, () => {
      expect(approvalChipState(c.approved, c.companyId)).toBe(c.want)
    })
  }

  it('取得失敗と「承認0件」が同じ表示にならない', () => {
    expect(approvalChipState(null, 1)).not.toBe(approvalChipState(new Set<number>(), 1))
  })
})

describe('承認操作の可否 (#1452)', () => {
  it('学校が選ばれ承認状態も取れていれば操作できる', () => {
    expect(canToggleApproval(new Set<number>(), 3)).toBe(true)
  })

  it('承認状態を取得できていない間は操作させない', () => {
    expect(canToggleApproval(null, 3)).toBe(false)
  })

  it('学校が選ばれていなければ操作させない', () => {
    expect(canToggleApproval(new Set<number>(), undefined)).toBe(false)
  })
})
