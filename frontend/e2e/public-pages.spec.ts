import { test, expect } from '@playwright/test'

// 未ログインで到達できるべきページ。
//
// 14315aa7 の一括 Server Component 化で /company-entry にだけ requireSessionUser() が
// 混入し、人事ゲストが企業情報を1件も投稿できなくなっていた（#1074）。
// 画面には「ログイン不要でご利用いただけます」と表示されたまま /login へ飛ばされていた。
// 同種の巻き添えを次からは CI で捕まえる。
const PUBLIC_PAGES = [
  { path: '/company-entry', mustSee: '企業情報登録フォーム' },
  { path: '/login', mustSee: 'ログイン' },
  { path: '/privacy', mustSee: 'プライバシー' },
]

// h1 を1つ持つべき公開ページ。スクリーンリーダーと検索エンジンは h1 で
// 「このページは何か」を判断するため、見出しが h4/h5 のままだと構造が伝わらない（#1479）。
const PAGES_WITH_H1 = [
  ...PUBLIC_PAGES.map(({ path }) => path),
  '/forgot-password',
  '/reset-password',
  '/register/confirm',
  '/company-portal/sign-in',
  '/company-portal/forgot-password',
  '/company-portal/reset-password',
]

test.describe('公開ページは未ログインで到達できる', () => {
  for (const { path, mustSee } of PUBLIC_PAGES) {
    test(`${path} が /login へリダイレクトされない`, async ({ page }) => {
      // Cookie を明示的に空にして未ログイン状態を作る。
      await page.context().clearCookies()

      await page.goto(path)

      // リダイレクト先ではなく、要求したパスに留まっていること。
      // 正規表現ではなく述語で比較する（パスにメタ文字が入っても誤判定しない）。
      await expect(page).toHaveURL((url) => url.pathname === path)
      await expect(page.getByText(mustSee).first()).toBeVisible({ timeout: 10000 })
    })
  }

  test('/company-entry は未ログインで投稿フォームを操作できる', async ({ page }) => {
    await page.context().clearCookies()
    await page.goto('/company-entry')

    // 「ログイン不要」と書いてある以上、実際に入力できる必要がある。
    await expect(page.getByText('ログイン不要でご利用いただけます')).toBeVisible()
    const nameField = page.getByLabel('企業名 *')
    await nameField.fill('テスト株式会社')
    await expect(nameField).toHaveValue('テスト株式会社')
  })
})

test.describe('主要な公開ページには h1 が1つある', () => {
  for (const path of PAGES_WITH_H1) {
    test(`${path} の h1 はちょうど1つ`, async ({ page }) => {
      await page.context().clearCookies()
      await page.goto(path)

      // getByRole は display:none の要素を除くので、実際に読み上げられる見出しだけを数える。
      const h1 = page.getByRole('heading', { level: 1 })
      await expect(h1.first()).toBeVisible({ timeout: 10000 })
      await expect(h1).toHaveCount(1)
    })
  }
})
