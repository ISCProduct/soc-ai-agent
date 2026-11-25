import { readFileSync } from 'fs'
import { join } from 'path'

// 破壊的な操作の確認は MUI の Dialog（components/common/ConfirmDialog）で行う（SOCAIAGENT-534）。
// ブラウザ標準の confirm() はスタイルが当たらずテストもできないため、対象の画面に戻さない。
const TARGETS = [
  'app/schedule/page-content.tsx',
  'app/admin/users/page-content.tsx',
  'app/admin/job-positions/page-content.tsx',
  'app/admin/companies/page-content.tsx',
  // 一括公開の確認はフックに移った（develop の分割のあと）
  'app/admin/companies/hooks/useCompanyRowActions.ts',
  'components/admin/CompanyUsersPanel.tsx',
]

// window.confirm( と素の confirm( の両方。onConfirm( などの識別子の一部は含めない
const CONFIRM_CALL = /(?<![\w$])(?:window\.|globalThis\.)?confirm\s*\(/

describe('破壊的な操作で window.confirm を使わない', () => {
  it.each(TARGETS)('%s に confirm( の呼び出しが無い', (file) => {
    const source = readFileSync(join(__dirname, '..', '..', file), 'utf8')
    expect(source).not.toMatch(CONFIRM_CALL)
  })

  it('検査の正規表現が confirm の呼び出しを見つけられる', () => {
    expect("if (!confirm('x')) return").toMatch(CONFIRM_CALL)
    expect('window.confirm(`x`)').toMatch(CONFIRM_CALL)
    expect('onConfirm()').not.toMatch(CONFIRM_CALL)
    expect('handleConfirm ()').not.toMatch(CONFIRM_CALL)
  })
})
