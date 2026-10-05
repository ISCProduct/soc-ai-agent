import { test, expect } from '@playwright/test'

test.describe('認証フロー', () => {
  test('ログインページが表示される', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByRole('tab', { name: 'ログイン' })).toBeVisible()
    await expect(page.locator('input[type="email"]')).toBeVisible()
    await expect(page.locator('input[type="password"]')).toBeVisible()
  })

  test('メールアドレスとパスワードでログインできる', async ({ page }) => {
    await page.route('**/api/auth/login', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          user_id: 1,
          email: 'test@example.com',
          name: 'テストユーザー',
          token: 'mock-token',
          user_token: 'mock-user-token',
          is_guest: false,
        }),
      })
    })

    await page.goto('/login')
    await page.waitForLoadState('networkidle')
    await page.locator('input[type="email"]').fill('test@example.com')
    await page.locator('input[type="password"]').fill('password123')
    await page.getByRole('button', { name: 'ログイン', exact: true }).click()

    await expect(page).not.toHaveURL(/\/login/, { timeout: 10000 })
  })

  test('無効な認証情報でエラーメッセージが表示される', async ({ page }) => {
    await page.route('**/api/auth/login', async (route) => {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'UNAUTHORIZED', error: 'invalid email or password' }),
      })
    })

    await page.goto('/login')
    await page.waitForLoadState('networkidle')
    await page.locator('input[type="email"]').fill('wrong@example.com')
    await page.locator('input[type="password"]').fill('wrongpassword')
    await page.getByRole('button', { name: 'ログイン', exact: true }).click()

    const errorAlert = page.getByRole('alert').filter({
      hasText: /メールアドレスまたはパスワードが正しくありません/,
    })
    await expect(errorAlert).toBeVisible({ timeout: 5000 })
  })

  // 未ログインの訪問者に公開LPが出ること(#1653)。
  // 以前は / が認証必須で、説明なしにログインフォームへ飛ばされていた。
  test('未ログインで / を開くと公開LPが表示され、3つの入口が出る', async ({ page }) => {
    await page.goto('/')

    await expect(page).toHaveURL(/\/$/)
    // ファーストビューのキャッチコピー
    await expect(page.getByRole('heading', { level: 1 })).toContainText('最初の一歩から', {
      timeout: 15000,
    })

    // LPの骨格。索引ページに戻していないことを見る（#1653）。
    // 課題提起と解決策が無いと、できることを並べただけのページになる。
    for (const label of ['ISSUE', 'SOLUTION', 'ENTRANCE', 'FAQ']) {
      await expect(page.getByText(label, { exact: true })).toBeVisible()
    }

    // 3系統の入口。文言が重複するので入口セクションに絞る。
    // 「無料で診断を始める」は FV・入口・クロージングの3箇所に出る。
    const entrances = page.locator('#entrances')
    for (const name of ['学生', '企業', '学校・教員']) {
      await expect(entrances.getByRole('heading', { name, exact: true })).toBeVisible()
    }
    await expect(entrances.getByRole('link', { name: '無料で診断を始める' })).toHaveAttribute(
      'href',
      '/login',
    )
    await expect(entrances.getByRole('link', { name: '企業ポータルへ' })).toHaveAttribute(
      'href',
      '/company-portal/sign-in',
    )
    await expect(entrances.getByRole('link', { name: '管理画面へ' })).toHaveAttribute(
      'href',
      '/admin',
    )
  })

  // セッションCookieが失効し、ストレージだけが残った状態の回帰テスト(#1519)。
  //
  // 「ローディングとログイン画面がぐちゃぐちゃになる」と報告された症状。
  // / はサーバー側でCookieを見て /login へ戻し、その /login はストレージを見て
  // / へ送り返すため、2画面を往復し続けていた。
  test('Cookieが切れてストレージだけ残った状態でも、ログイン画面とホームを往復しない', async ({ page }) => {
    // Cookieは付けず、ストレージにだけログイン済みの痕跡を残す
    await page.addInitScript(() => {
      const userData = {
        user_id: 1,
        email: 'test@example.com',
        name: 'テストユーザー',
        is_guest: false,
        is_admin: false,
      }
      sessionStorage.setItem('user', JSON.stringify(userData))
      localStorage.setItem('user', JSON.stringify(userData))
      localStorage.setItem('user_token', 'expired-user-token')
    })

    const visited: string[] = []
    page.on('framenavigated', (frame) => {
      if (frame === page.mainFrame()) visited.push(new URL(frame.url()).pathname)
    })

    await page.goto('/')

    // / は公開LPになったので /login へは飛ばされない(#1653)。
    // ループの起点だった「サーバー側リダイレクト」自体が無くなっている。
    await expect(page.getByRole('heading', { level: 1 })).toContainText('最初の一歩から', {
      timeout: 15000,
    })
    await expect(page).toHaveURL(/\/$/)

    // LPからログイン画面へ進んでも、ストレージを見て / へ送り返されない。
    // 送り返しを止めているのは LoginContent 側の Cookie 確認
    // (/api/auth/session が401ならその場に留まる)。
    await page.locator('#entrances').getByRole('link', { name: '無料で診断を始める' }).click()
    await expect(page).toHaveURL(/\/login/, { timeout: 15000 })
    await expect(page.getByRole('tab', { name: 'ログイン' })).toBeVisible({ timeout: 10000 })

    // 往復が続いていないことを確かめる。修正前はここで / と /login を
    // 行き来し続けていた。RSC遷移で同一URLの framenavigated が複数回出るため、
    // 件数の増減ではなく「ホームへ戻された回数」で見る。
    await page.waitForTimeout(3000)
    const backToHome = visited.filter((p) => p === '/').length
    expect(backToHome, `遷移履歴: ${visited.join(' -> ')}`).toBeLessThanOrEqual(2)
    await expect(page).toHaveURL(/\/login/)
  })

  test('仮登録メールアドレス送信', async ({ page }) => {
    await page.route('**/api/auth/request-registration', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'confirmation email sent' }),
      })
    })

    await page.goto('/login')
    await page.waitForLoadState('networkidle')
    await page.getByRole('tab', { name: '新規登録' }).click()
    await page.locator('input[type="email"]').last().fill('newuser@example.com')
    await page.getByRole('button', { name: '確認メールを送る' }).click()

    await expect(page.getByText(/確認リンクを送りました/)).toBeVisible({ timeout: 5000 })
  })
})
