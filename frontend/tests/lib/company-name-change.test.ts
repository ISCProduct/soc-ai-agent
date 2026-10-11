import { companyNameChangeNotice } from '@/lib/company/profile'

describe('companyNameChangeNotice', () => {
  it('変更前と変更後の名前を出す', () => {
    expect(companyNameChangeNotice('旧株式会社', '新株式会社')).toBe(
      '企業名を「旧株式会社」から「新株式会社」に変更しました',
    )
  })

  it('同じ名前では出さない', () => {
    expect(companyNameChangeNotice('同じ株式会社', '  同じ株式会社  ')).toBeNull()
  })

  it('空の名前では出さない', () => {
    expect(companyNameChangeNotice('旧株式会社', '   ')).toBeNull()
  })
})
