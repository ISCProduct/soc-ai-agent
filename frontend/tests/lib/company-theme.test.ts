import { createCompanyMuiTheme } from '@/lib/company-theme'

describe('company-theme', () => {
  it('学生の青とも管理画面の藍とも違う緑を主操作にする', () => {
    const theme = createCompanyMuiTheme()
    expect(theme.palette.primary.main.toUpperCase()).toBe('#1F5136')
    expect(theme.palette.error.main.toUpperCase()).toBe('#A4303F')
    expect(theme.palette.background.default.toUpperCase()).toBe('#F4F3EF')
    expect(theme.shape.borderRadius).toBe(4)
  })
})