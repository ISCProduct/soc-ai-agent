import { validateNewPassword } from '@/lib/company-auth'

describe('validateNewPassword', () => {
  const cases: { name: string; password: string; confirm: string; expected: string | null }[] = [
    { name: '8文字以上で一致していればエラーなし', password: 'password123', confirm: 'password123', expected: null },
    { name: 'ちょうど8文字は許可される', password: '12345678', confirm: '12345678', expected: null },
    { name: '8文字未満はエラー', password: '1234567', confirm: '1234567', expected: 'パスワードは8文字以上で入力してください。' },
    { name: '確認用と不一致はエラー', password: 'password123', confirm: 'password124', expected: 'パスワードが一致しません。' },
    { name: '空文字は文字数エラーを優先', password: '', confirm: '', expected: 'パスワードは8文字以上で入力してください。' },
  ]

  it.each(cases)('$name', ({ password, confirm, expected }) => {
    expect(validateNewPassword(password, confirm)).toBe(expected)
  })
})
