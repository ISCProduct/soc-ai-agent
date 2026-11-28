import { test, expect, type Page } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { setupAuth, TEST_USER } from './fixtures/auth'
import { company, sampleMap, subsidiary } from '../tests/fixtures/correlation-map'

for (const contentHeight of [900, 2000]) {
  test(`PC wheel scroll continues from graph to page with ${contentHeight}px content`, async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    // 実際のCSSで、詳細が長い画面のスクロール構造を再現する。サーバーや認証には依存しない。
    const css = readFileSync(resolve('components/diagram/explorer/explorer.module.css'), 'utf8')
    await page.setContent(`<style>${css}
      body { margin:0; } main { height:900px; overflow:auto; }
    </style><main><div class="explorer"><div class="workspace" style="--normal-graph-height:1400px">
      <div class="graphColumn"><div class="graphViewport"><div style="height:${contentHeight}px">相関図</div></div></div>
      <aside class="detailPanel" style="height:1400px">企業詳細</aside>
    </div></div></main>`)
    const graph = page.locator('.graphViewport')
    const main = page.locator('main')
    await page.mouse.move(200, 200)
    await page.mouse.wheel(0, 300)
    if (contentHeight > 1400) {
      await expect.poll(() => graph.evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
      expect(await main.evaluate((el) => el.scrollTop)).toBe(0)
      await graph.evaluate((el) => { el.scrollTop = el.scrollHeight })
      await page.mouse.move(210, 210)
      await page.mouse.wheel(0, 300)
    }
    await expect.poll(() => main.evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
    await main.evaluate((el) => { el.scrollTop = 300 })
    await graph.evaluate((el) => { el.scrollTop = 0 })
    await page.mouse.move(220, 220)
    await page.mouse.wheel(0, -300)
    await expect.poll(() => main.evaluate((el) => el.scrollTop)).toBeLessThan(300)
  })
}

async function mockDiagram(page: Page) {
  await setupAuth(page, TEST_USER)
  const map = sampleMap()
  await page.route('**/api/**', (route) => route.fulfill({ json: {} }))
  await page.route('**/api/notifications/unread-count', (route) => route.fulfill({ json: { count: 0 } }))
  await page.route('**/api/notifications?*', (route) => route.fulfill({ json: { notifications: [], unread_count: 0 } }))
  await page.route('**/api/bff/auth/user', (route) => route.fulfill({ json: TEST_USER }))
  await page.route('**/api/companies/*/correlation-map', (route) => route.fulfill({ json: map }))
  await page.route('**/api/companies/*/correlation-map/compare?*', (route) => {
    const selected = Number(new URL(route.request().url()).searchParams.get('with'))
    return route.fulfill({ json: { focus_company_id: 1, selected_company_id: selected, focus: map.nodes[0], selected: map.nodes.find((node) => node.id === selected), compare_text_available: false } })
  })
}

test('PC: keyboard selection, close focus, expansion and a textual relationship view', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockDiagram(page)
  await page.goto('/correlation-diagram?company_id=1')
  const menu = page.getByRole('navigation', { name: 'メニュー' })
  await expect(menu.getByRole('link', { name: 'ホーム', exact: true })).toBeVisible()
  await expect(menu.getByRole('link', { name: '企業相関図', exact: true })).toHaveAttribute('aria-current', 'page')
  await expect(menu.getByRole('link', { name: 'お気に入り', exact: true })).toBeVisible()
  const card = page.getByRole('button', { name: 'あおばクラウドの情報を見る' })
  await expect(card).toBeVisible()
  await page.screenshot({ path: 'test-results/correlation-pc-initial.png' })
  await card.focus()
  await page.keyboard.press('Enter')
  const detail = page.getByRole('complementary', { name: 'あおばクラウド' })
  await expect(detail).toBeVisible()
  await expect(detail.getByRole('heading', { name: 'あおばクラウド', level: 2 })).toBeInViewport()
  await expect(detail.getByRole('link', { name: '企業詳細を見る' })).toHaveAttribute('href', '/company/4')
  expect((await detail.boundingBox())!.width).toBeGreaterThanOrEqual(360)
  expect((await detail.getByRole('region', { name: '企業情報の本文' }).boundingBox())!.height).toBeGreaterThanOrEqual(280)
  const aiBox = (await detail.getByRole('button', { name: 'AIに質問する' }).boundingBox())!
  const detailBox = (await detail.getByRole('link', { name: '企業詳細を見る' }).boundingBox())!
  expect(Math.abs(aiBox.y - detailBox.y)).toBeLessThan(1)
  const body = detail.getByRole('region', { name: '企業情報の本文' })
  expect(await body.evaluate((el) => el.scrollHeight <= el.clientHeight + 1)).toBe(true)
  expect(await body.evaluate((el) => getComputedStyle(el).overflowY)).toBe('visible')
  await detail.getByRole('link', { name: '企業詳細を見る' }).scrollIntoViewIfNeeded()
  expect(await page.getByRole('main').evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
  expect(await detail.evaluate((el) => el.scrollTop)).toBe(0)
  expect(await body.evaluate((el) => el.scrollTop)).toBe(0)
  await detail.getByRole('button', { name: 'AIに質問する' }).click()
  const aiNotice = page.getByRole('complementary', { name: '企業研究AI' })
  await expect(aiNotice).toBeVisible()
  await expect(aiNotice.getByText('AIチャットは準備中です')).toBeVisible()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.screenshot({ path: 'test-results/correlation-pc-selected.png' })
  await aiNotice.getByRole('button', { name: '閉じる' }).click()
  await expect(card).toBeFocused()
  const sizeBefore = await card.evaluate((el) => getComputedStyle(el.querySelector('span')!).fontSize)
  await page.getByRole('button', { name: '残り1社を見る' }).click()
  await expect(page.getByRole('button', { name: 'あおば保守の情報を見る' })).toBeVisible()
  expect(await card.evaluate((el) => getComputedStyle(el.querySelector('span')!).fontSize)).toBe(sizeBefore)
  await expect(page.getByRole('button', { name: '企業を一覧で見る' })).toHaveCount(0)
  await page.getByRole('button', { name: 'クラウド', exact: true }).click()
  await expect(page.getByRole('tabpanel')).toContainText('東日本クラウドはあおばクラウドの子会社')
  const edge = page.locator('[data-edge-from="4"][data-edge-to="9"]')
  await expect(edge).toBeVisible()
  expect(await edge.evaluate((el) => getComputedStyle(el, '::after').borderTopStyle)).toBe('solid')
})

test('PC: graph scroll is independent while details use page scrolling', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await mockDiagram(page)
  const map = sampleMap()
  const children = Array.from({ length: 40 }, (_, i) => company(i + 100, `関連企業${i + 1}`))
  map.nodes = [map.nodes[0], ...children]
  map.capital_edges = children.map(node => subsidiary(1, node.id))
  map.business_edges = []
  map.meta = { ...map.meta, direct_parent_ids: [], direct_child_ids: children.map(node => node.id), direct_child_total: 40, initial_child_ids: children.slice(0, 4).map(node => node.id), remaining_child_count: 36 }
  await page.route('**/api/companies/1/correlation-map', route => route.fulfill({ json: map }))
  await page.goto('/correlation-diagram?company_id=1')
  await page.addStyleTag({ content: 'nextjs-portal{display:none!important}' })
  await page.getByRole('button', { name: '残り36社を見る' }).click()
  const graph = page.getByRole('tabpanel')
  const lower = graph.locator('[data-company-id]').last()
  const selectedName = (await lower.getAttribute('aria-label'))!.replace('の情報を見る', '')
  const main = page.getByRole('main')
  const before = await main.evaluate((el) => el.scrollTop)
  await lower.click()
  const details = page.getByRole('complementary', { name: selectedName! })
  await expect(details).toBeInViewport()
  const bottomGap = async () => {
    const a = (await graph.boundingBox())!, b = (await details.boundingBox())!
    return Math.abs(a.y + a.height - b.y - b.height)
  }
  await expect.poll(bottomGap).toBeLessThan(1)
  expect(await graph.evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
  await expect(details.getByRole('heading', { name: selectedName!, exact: true, level: 2 })).toBeInViewport()
  const graphScroll = await graph.evaluate((el) => el.scrollTop)
  await details.getByRole('button', { name: 'AIに質問する' }).scrollIntoViewIfNeeded()
  expect(await details.evaluate((el) => el.scrollTop)).toBe(0)
  expect(await details.evaluate((el) => getComputedStyle(el).overflowY)).toBe('visible')
  expect(await graph.evaluate((el) => el.scrollTop)).toBe(graphScroll)
  const readingPosition = await main.evaluate((el) => el.scrollTop)
  expect(readingPosition).toBeGreaterThan(before)
  await expect(lower).toBeInViewport()
  await details.locator('summary').click()
  await expect.poll(bottomGap).toBeLessThan(1)
  await details.locator('summary').click()
  await expect.poll(bottomGap).toBeLessThan(1)
  await page.screenshot({ path: 'test-results/correlation-independent-scroll.png' })
  await page.getByRole('button', { name: '拡大表示', exact: true }).scrollIntoViewIfNeeded()
  const beforeExpanded = await main.evaluate((el) => el.scrollTop)
  await page.getByRole('button', { name: '拡大表示', exact: true }).click()
  const expanded = page.getByRole('dialog', { name: /企業相関図/ })
  await expect(expanded.getByRole('complementary', { name: selectedName! })).toBeInViewport()
  const expandedArea = expanded.getByRole('region', { name: '拡大した相関図' })
  await expect(expandedArea.locator('[data-company-id]').last()).toBeInViewport()
  await page.screenshot({ path: 'test-results/correlation-expanded-details.png' })
  await expanded.getByRole('button', { name: '通常の相関図に戻る' }).click()
  await expect.poll(bottomGap).toBeLessThan(1)
  expect(await graph.evaluate((el) => el.scrollTop)).toBe(graphScroll)
  expect(await main.evaluate((el) => el.scrollTop)).toBe(beforeExpanded)
})

test('mobile: outside trunk, nested expansion, drawer details and menu', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await mockDiagram(page)
  await page.goto('/correlation-diagram?company_id=1')
  await expect(page.getByRole('button', { name: 'メニュー', exact: true })).toBeVisible()
  const bell = page.getByRole('button', { name: '通知', exact: true })
  await expect(bell).toHaveCount(1)
  const menuBox = (await page.getByRole('button', { name: 'メニュー', exact: true }).boundingBox())!
  const bellBox = (await bell.boundingBox())!
  expect(bellBox.height).toBeGreaterThanOrEqual(44)
  expect(Math.abs(menuBox.y - bellBox.y)).toBeLessThan(1)
  expect(bellBox.x - menuBox.x - menuBox.width).toBeGreaterThanOrEqual(8)
  await bell.click()
  const notifications = page.getByRole('dialog', { name: '通知一覧' })
  await expect(notifications.getByText(/新しい通知はありません/)).toBeVisible()
  await expect.poll(async () => (await notifications.boundingBox())!.y).toBeGreaterThan(bellBox.y + bellBox.height)
  await notifications.getByRole('button', { name: '通知を閉じる' }).click()
  await expect(bell).toBeFocused()
  await page.screenshot({ path: 'test-results/correlation-mobile-initial.png' })
  await page.getByRole('button', { name: '子会社1社を見る' }).click()
  const grandchild = page.getByRole('button', { name: '東日本クラウドの情報を見る' })
  await grandchild.scrollIntoViewIfNeeded()
  await expect(grandchild).toBeVisible()
  const noOverflow = await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)
  expect(noOverflow).toBe(true)
  await page.screenshot({ path: 'test-results/correlation-mobile-expanded.png' })
  await grandchild.click()
  const sheet = page.getByRole('dialog', { name: '東日本クラウド' })
  await expect(sheet).toBeVisible()
  await expect.poll(async () => (await sheet.boundingBox())?.y ?? 844).toBeLessThan(85)
  const detailBody = sheet.getByRole('region', { name: '企業情報の本文' })
  expect((await detailBody.boundingBox())!.height).toBeGreaterThan(500)
  await detailBody.evaluate((el) => { el.scrollTop = el.scrollHeight })
  await expect(sheet.getByRole('button', { name: '閉じる', exact: true })).toBeInViewport()
  const detailLink = sheet.getByRole('link', { name: '企業詳細を見る' })
  await expect.poll(async () => { const box = await detailLink.boundingBox(); return box ? box.y + box.height : 900 }).toBeLessThanOrEqual(844)
  await page.screenshot({ path: 'test-results/correlation-mobile-details.png' })
  await sheet.getByRole('button', { name: 'AIに質問する' }).click()
  const aiNotice = page.getByRole('dialog', { name: '企業研究AI' })
  await expect(aiNotice).toBeVisible()
  await expect(aiNotice.getByText('AIチャットは準備中です')).toBeVisible()
  await expect(page.getByRole('dialog')).toHaveCount(1)
  await aiNotice.getByRole('button', { name: '閉じる' }).click()
  await expect(sheet).not.toBeVisible()
  await page.getByRole('button', { name: 'メニュー', exact: true }).click()
  await expect(page.getByRole('navigation', { name: 'メニュー' })).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'メニュー' }).getByRole('link', { name: '企業を探す', exact: true })).toBeVisible()
})

test('business relations show no provider direction; tag filtering includes folded companies', async ({ page }) => {
  await mockDiagram(page)
  await page.goto('/correlation-diagram?company_id=1')
  await page.getByRole('button', { name: 'クラウド', exact: true }).click()
  await expect(page.getByRole('button', { name: '東日本クラウドの情報を見る' })).toBeVisible()
  await page.getByRole('tab', { name: '取引・協力関係' }).click()
  await expect(page.getByText('線は取引・協力関係を表します。提供側・利用側の向きは確認できていません。')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'あおば取引先の情報を見る' })).toBeVisible()
  await page.getByRole('button', { name: 'あおば取引先の情報を見る' }).click()
  await expect(page.getByText('システムの保守を共同で行う。')).toBeVisible()
})

test('29 companies stay readable and a company can be selected from the entry screen', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1080 })
  await mockDiagram(page)
  const map = sampleMap()
  map.nodes = [map.nodes[0], map.nodes[1], ...Array.from({ length: 27 }, (_, index) => company(index + 3, `関連企業${index + 1}`))]
  map.capital_edges = [subsidiary(2, 1), subsidiary(28, 1), subsidiary(29, 1), ...map.nodes.slice(2, -2).map((node) => subsidiary(1, node.id))]
  map.business_edges = []
  await page.route('**/api/companies/1/correlation-map', (route) => route.fulfill({ json: map }))
  await page.route('**/api/companies/relations', (route) => route.fulfill({ json: [{ id: 1, parent_id: 1, child_id: 3, parent: map.nodes[0], child: map.nodes[2], relation_type: 'capital_subsidiary', description: '' }] }))
  await page.route('**/api/companies/market-info', (route) => route.fulfill({ json: [] }))
  await page.goto('/correlation-diagram')
  await expect(page.getByRole('navigation', { name: 'メニュー' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '気になる企業から探す' })).toBeVisible()
  await page.getByRole('textbox', { name: '企業名で検索' }).fill('青葉テクノロジー')
  await page.getByRole('button', { name: /青葉テクノロジー/ }).click()
  const panel = page.getByRole('tabpanel')
  await expect(panel.getByRole('button', { name: /情報を見る/ })).toHaveCount(8)
  const first = panel.getByRole('button', { name: '関連企業1の情報を見る' })
  const font = await first.evaluate((element) => getComputedStyle(element).fontSize)
  await page.getByRole('button', { name: '残り21社を見る' }).click()
  await expect(panel.getByRole('button', { name: /情報を見る/ })).toHaveCount(29)
  expect(await first.evaluate((element) => getComputedStyle(element).fontSize)).toBe(font)
  await panel.getByRole('button', { name: '関連企業25の情報を見る' }).scrollIntoViewIfNeeded()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.getByRole('button', { name: '拡大表示', exact: true }).click()
  const expanded = page.getByRole('dialog', { name: '企業相関図' })
  await expect(expanded.getByRole('button', { name: /情報を見る/ })).toHaveCount(29)
  await expanded.getByRole('button', { name: '表示を少なくする' }).click()
  await expanded.getByRole('button', { name: '図を縮小', exact: true }).click()
  await expanded.getByRole('button', { name: '残り21社を見る' }).click()
  await expect(expanded.getByLabel('表示倍率')).toHaveText('75％')
  await expect(expanded.getByRole('button', { name: /情報を見る/ })).toHaveCount(29)
  await expanded.getByRole('button', { name: '関連企業25の情報を見る' }).scrollIntoViewIfNeeded()
  expect(await expanded.getByRole('region', { name: '拡大した相関図' }).evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
  await expanded.getByRole('button', { name: '通常の相関図に戻る', exact: true }).click()
  await page.getByRole('button', { name: '企業を変更', exact: true }).click()
  await expect(page.getByRole('textbox', { name: '企業名で検索' })).toBeVisible()
  await page.getByRole('textbox', { name: '企業名で検索' }).fill('青葉テクノロジー')
  await page.getByRole('button', { name: /青葉テクノロジー/ }).click()
  await expect(page.getByRole('tab', { name: '親会社・子会社' })).toBeVisible()
})

test('back goes from a directly opened diagram to the picker, then home without returning to the diagram', async ({ page }) => {
  await mockDiagram(page)
  await page.route('**/api/companies/relations', (route) => route.fulfill({ json: [] }))
  await page.goto('/correlation-diagram?company_id=1&diagram_type=business')
  await expect(page.getByRole('tab', { name: '取引・協力関係' })).toBeVisible()
  const historyLength = await page.evaluate(() => history.length)
  await page.getByRole('button', { name: '戻る', exact: true }).click()
  await expect(page).toHaveURL(/\/correlation-diagram$/)
  expect(await page.evaluate(() => history.length)).toBe(historyLength)
  await expect(page.getByRole('heading', { name: '気になる企業から探す' })).toBeVisible()
  await page.getByRole('button', { name: '戻る', exact: true }).click()
  await expect(page).toHaveURL(/\/$/, { timeout: 30000 })
})

test('a company with only business relations opens its business graph', async ({ page }) => {
  await mockDiagram(page)
  const map = sampleMap()
  await page.route('**/api/companies/relations', (route) => route.fulfill({ json: [{ id: 1, from_id: 1, to_id: 10, from: map.nodes[0], to: map.nodes[9], relation_type: 'business', description: '' }] }))
  await page.route('**/api/companies/market-info', (route) => route.fulfill({ json: [] }))
  await page.goto('/correlation-diagram')
  await page.getByRole('button', { name: /青葉テクノロジー/ }).click()
  await expect(page.getByRole('tab', { name: '取引・協力関係' })).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByRole('button', { name: 'あおば取引先の情報を見る' })).toBeVisible()
})

for (const width of [375, 768, 1024, 1440]) {
  test(`arrow tips align with stems and card edges at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 })
    await mockDiagram(page)
    await page.goto('/correlation-diagram?company_id=1')
    await expect(page.getByRole('button', { name: '青葉テクノロジーの情報を見る（起点）' })).toBeVisible()
    async function checkArrows() {
      const offsets = await page.getByRole('tabpanel').getByRole('button', { name: /情報を見る/ }).evaluateAll((cards) => cards.flatMap((card) => {
        const arrow = getComputedStyle(card, '::after')
        if (arrow.content === 'none' || arrow.display === 'none') return []
        const wrapper = card.parentElement!
        const parentConnection = wrapper.tagName !== 'LI'
        const line = getComputedStyle(wrapper, parentConnection ? '::before' : '::after')
        const cardBox = card.getBoundingClientRect()
        const wrapperBox = wrapper.getBoundingClientRect()
        const style = getComputedStyle(card)
        const n = (value: string) => Number.parseFloat(value) || 0
        const arrowX = cardBox.x + n(style.borderLeftWidth) + n(arrow.left)
        const arrowY = cardBox.y + n(style.borderTopWidth) + n(arrow.top)
        const downward = n(arrow.borderTopWidth) === 7
        if (downward) {
          const left = wrapperBox.x + n(line.left)
          const lineX = n(line.borderLeftWidth) > 0 ? left + n(line.borderLeftWidth) / 2 : left + n(line.width) - n(line.borderRightWidth) / 2
          return [{ name: card.getAttribute('aria-label'), center: arrowX + n(arrow.borderLeftWidth) - lineX, edge: arrowY + n(arrow.borderTopWidth) - cardBox.y }]
        }
        return [{ name: card.getAttribute('aria-label'), center: arrowY + n(arrow.borderTopWidth) - (wrapperBox.y + n(line.top) + n(line.borderTopWidth) / 2), edge: arrowX + n(arrow.borderLeftWidth) - cardBox.x }]
      }))
      expect(offsets.length).toBeGreaterThan(0)
      for (const offset of offsets) {
        expect(Math.abs(offset.center), `${offset.name}: line center`).toBeLessThan(0.1)
        expect(Math.abs(offset.edge), `${offset.name}: card edge`).toBeLessThan(0.1)
      }
    }
    await checkArrows()
    await page.getByRole('button', { name: '子会社1社を見る' }).click()
    await checkArrows()
    if (width >= 1200) {
      await page.getByRole('button', { name: 'あおばクラウドの情報を見る' }).click()
      await expect(page.getByRole('complementary', { name: 'あおばクラウド' })).toBeVisible()
      await checkArrows()
    }
  })
  test(`connections stay attached without stray borders at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 })
    await mockDiagram(page)
    await page.goto('/correlation-diagram?company_id=1&diagram_type=business')
    const card = page.getByRole('button', { name: 'あおば取引先の情報を見る' })
    await expect(card).toBeVisible()
    const connection = await card.evaluate((element) => {
      const branch = element.parentElement!
      const group = branch.parentElement!
      const line = getComputedStyle(branch, '::after')
      const stem = getComputedStyle(group, '::after')
      const arrow = getComputedStyle(element, '::after')
      const graph = group.parentElement!
      const origin = graph.querySelector('button')!
      const rootBox = origin.getBoundingClientRect()
      const cardBox = element.getBoundingClientRect()
      return {
        lineBottom: line.borderBottomWidth, stemRight: stem.borderRightWidth,
        stemTop: stem.borderTopWidth, stemStyle: stem.borderLeftStyle,
        arrow: arrow.display,
        centered: Math.abs(rootBox.x + rootBox.width / 2 - cardBox.x - cardBox.width / 2),
        wide: group.parentElement!.parentElement!.clientWidth >= 640,
      }
    })
    expect(connection.lineBottom).toBe('0px')
    expect(connection.stemRight).toBe('0px')
    expect(connection.stemTop).toBe('0px')
    expect(connection.stemStyle).toBe('dashed')
    expect(connection.arrow).toBe('none')
    if (connection.wide) expect(connection.centered).toBeLessThan(1)
    // 選択による線の付け替え・折り返し後も、枠幅の変更でカードが伸縮しない。
    await card.click()
    // モバイルの詳細シートが開くと背面は aria-hidden になるため、背面カードも明示して測定する。
    const border = await page.getByRole('button', { name: 'あおば取引先の情報を見る', includeHidden: true }).evaluate((element) => getComputedStyle(element).borderLeftWidth)
    expect(border).toBe('1px')
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  })
  test(`no page overflow at ${width}px, no data and error states are distinct`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await mockDiagram(page)
    const map = sampleMap()
    await page.route('**/api/companies/1/correlation-map', (route) => route.fulfill({ json: { ...map, nodes: [map.nodes[0]], capital_edges: [], business_edges: [] } }))
    await page.goto('/correlation-diagram?company_id=1')
    await expect(page.getByRole('tabpanel').locator('[data-company-id]').first()).toBeVisible()
    await expect(page.getByText('この企業の子会社はまだ登録されていません。')).toHaveCount(0)
    await expect(page.getByRole('button', { name: /残り\d+社を見る|表示を少なくする/ })).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.route('**/api/companies/1/correlation-map', (route) => route.fulfill({ status: 503, json: {} }))
    await page.reload()
    await expect(page.getByRole('button', { name: '再試行' })).toBeVisible()
    await expect(page.getByText('この企業の子会社はまだ登録されていません。')).not.toBeVisible()
  })
}

for (const width of [390, 1440]) {
  test(`unselected diagram uses page scrolling and restores it after closing details at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await mockDiagram(page)
    await page.goto('/correlation-diagram?company_id=1')
    await page.addStyleTag({ content: 'nextjs-portal{display:none!important}' })
    // The mobile modal hides its background from the accessibility tree.
    const graph = page.getByRole('tabpanel', { includeHidden: true })
    await expect(page.getByRole('button', { name: 'あおばクラウドの情報を見る' })).toBeAttached()
    expect(await graph.evaluate(el => getComputedStyle(el).overflowY)).toBe('visible')
    await graph.locator('[data-company-id]').last().scrollIntoViewIfNeeded()
    expect(await page.getByRole('main').evaluate(el => el.scrollTop)).toBeGreaterThan(0)
    expect(await graph.evaluate(el => el.scrollTop)).toBe(0)
    await page.getByRole('button', { name: 'あおばクラウドの情報を見る' }).click()
    const details = page.getByRole(width >= 1200 ? 'complementary' : 'dialog', { name: 'あおばクラウド' })
    await expect(details.getByRole('heading', { name: 'あおばクラウド', level: 2 })).toBeInViewport()
    expect(await graph.evaluate(el => getComputedStyle(el).overflowY)).toBe('auto')
    await details.getByRole('button', { name: '閉じる', exact: true }).click()
    expect(await graph.evaluate(el => getComputedStyle(el).overflowY)).toBe('visible')
    await graph.locator('[data-company-id]').last().scrollIntoViewIfNeeded()
    expect(await graph.evaluate(el => el.scrollTop)).toBe(0)
    expect(await page.getByRole('main').evaluate(el => el.scrollTop)).toBeGreaterThan(0)
  })

  test(`expanded graph preserves selection and branches, zooms and pans at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await mockDiagram(page)
    await page.goto('/correlation-diagram?company_id=1')
    // The development-only badge overlaps the bottom-left action on narrow screens.
    await page.addStyleTag({ content: 'nextjs-portal{display:none!important}' })
    await page.getByRole('button', { name: '子会社1社を見る' }).click()
    if (width >= 1200) await page.getByRole('button', { name: 'あおばクラウドの情報を見る' }).click()
    await page.getByRole('button', { name: '拡大表示', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: '企業相関図' })
    const area = dialog.getByRole('region', { name: '拡大した相関図' })
    const origin = dialog.getByRole('button', { name: '青葉テクノロジーの情報を見る（起点）' })
    await expect(dialog).toBeVisible()
    await expect(dialog.getByRole('button', { name: '東日本クラウドの情報を見る' })).toBeAttached()
    await expect(dialog.getByLabel('表示倍率')).toHaveText('100％')
    const originalWidth = (await origin.boundingBox())!.width
    if (width >= 600) await dialog.getByRole('button', { name: '残り1社を見る' }).click()
    else await expect(dialog.getByRole('button', { name: /残り\d+社を見る|表示を少なくする/ })).toHaveCount(0)
    await expect(dialog.getByRole('button', { name: 'あおば保守の情報を見る' })).toBeAttached()
    expect((await origin.boundingBox())!.width).toBe(originalWidth)
    await expect(dialog.getByLabel('表示倍率')).toHaveText('100％')
    for (let i = 0; i < 4; i++) await dialog.getByRole('button', { name: '図を拡大', exact: true }).click()
    await expect(dialog.getByLabel('表示倍率')).toHaveText('200％')
    expect((await origin.boundingBox())!.width).toBeCloseTo(originalWidth * 2, 0)
    await expect(dialog.getByRole('button', { name: '図を拡大', exact: true })).toBeDisabled()
    await area.evaluate((el) => { el.scrollTop = 0; el.scrollLeft = 0 })
    const box = (await area.boundingBox())!
    await page.mouse.move(box.x + 24, box.y + 240)
    await page.mouse.down()
    await page.mouse.move(box.x + 8, box.y + 80, { steps: 5 })
    await page.mouse.up()
    expect(await area.evaluate((el) => el.scrollTop)).toBeGreaterThan(100)
    await dialog.getByRole('button', { name: '起点の会社へ戻る' }).click()
    const originBox = (await origin.boundingBox())!
    expect(originBox.y).toBeGreaterThanOrEqual(box.y)
    expect(originBox.x + originBox.width / 2).toBeCloseTo(box.x + box.width / 2, -1)
    await dialog.getByRole('button', { name: '100％に戻す' }).click()
    await expect(dialog.getByLabel('表示倍率')).toHaveText('100％')
    expect((await origin.boundingBox())!.width).toBe(originalWidth)
    for (let i = 0; i < 2; i++) await dialog.getByRole('button', { name: '図を縮小', exact: true }).click()
    await expect(dialog.getByLabel('表示倍率')).toHaveText('50％')
    await expect(dialog.getByRole('button', { name: '図を縮小', exact: true })).toBeDisabled()
    await dialog.getByRole('button', { name: '100％に戻す' }).click()
    await dialog.getByRole('button', { name: 'あおばクラウドの情報を見る' }).click()
    const expandedDetails = dialog.getByRole('complementary', { name: 'あおばクラウド' })
    await expect(expandedDetails).toBeVisible()
    await expect(page.getByRole('dialog')).toHaveCount(1)
    const selectedCard = dialog.getByRole('button', { name: 'あおばクラウドの情報を見る' })
    await expect(selectedCard).toBeInViewport()
    const areaBox = (await area.boundingBox())!
    const panelBox = (await expandedDetails.boundingBox())!
    expect(panelBox.y + panelBox.height).toBeLessThanOrEqual(900)
    if (width >= 1200) expect(panelBox.x).toBeGreaterThanOrEqual(areaBox.x + areaBox.width)
    else expect(panelBox.y).toBeGreaterThanOrEqual(areaBox.y + areaBox.height)
    await expandedDetails.getByRole('button', { name: 'AIに質問する' }).click()
    const aiDetails = dialog.getByRole('complementary', { name: '企業研究AI' })
    await expect(aiDetails.getByText('AIチャットは準備中です')).toBeVisible()
    await expect(dialog).toBeVisible()
    await aiDetails.getByRole('button', { name: '閉じる', exact: true }).focus()
    await page.keyboard.press('Escape')
    await expect(dialog).toBeVisible()
    await expect(dialog.getByRole('complementary')).toHaveCount(0)
    await expect(selectedCard).toBeFocused()
    await selectedCard.click()
    await expect(expandedDetails).toBeVisible()
    await dialog.getByRole('button', { name: '通常の相関図に戻る' }).click()
    await expect(dialog).not.toBeVisible()
    const details = page.getByRole(width >= 1200 ? 'complementary' : 'dialog', { name: 'あおばクラウド' })
    await expect(details).toBeVisible()
    await expect(page.getByRole('button', { name: '東日本クラウドの情報を見る', includeHidden: true })).toBeAttached()
    await details.getByRole('button', { name: '閉じる', exact: true }).click()
    if (width >= 600) await expect(page.getByRole('button', { name: '表示を少なくする' })).toBeVisible()
    else await expect(page.getByRole('button', { name: '表示を少なくする' })).toHaveCount(0)
    await page.getByRole('button', { name: '拡大表示', exact: true }).click()
    await expect(dialog.getByRole('button', { name: '東日本クラウドの情報を見る' })).toBeAttached()
    await expect(dialog.getByRole('button', { name: 'あおば保守の情報を見る' })).toBeAttached()
    await page.keyboard.press('Escape')
    await expect(dialog).not.toBeVisible()
    await expect(page.getByRole('button', { name: '拡大表示', exact: true })).toBeFocused()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  })
}
