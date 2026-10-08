/** @jest-environment node */

import { readFileSync, existsSync } from 'fs'
import { shouldShowStudentBottomNav } from '@/lib/sidebar-nav'

/**
 * 下部ナビが出る画面に余白があることを検査する（#1570）。
 *
 * StudentBottomNav は position: fixed の56px + safe-area で、余白が無い画面では
 * 最後のセクションや Snackbar が隠れて読めない・押せない。
 *
 * 画面ごとに手書きすると必ず取りこぼす。実際、BottomNavSpacer を使っていたのは
 * 5画面だけで、/resume・/company/[id]・/whats-new は余白が無く、
 * /results は pb: { xs: 7 } を手書きしていて共通部品と二重化していた。
 *
 * 新しい画面を足したときにここで気づけるようにする。
 */

/** 下部ナビが出る画面と、その実体ファイル。 */
const SCREENS: Array<{ path: string; file: string }> = [
  { path: '/', file: 'app/page-content.tsx' },
  { path: '/results', file: 'app/results/components/ResultsListView.tsx' },
  { path: '/resume', file: 'app/resume/page-content.tsx' },
  { path: '/schedule', file: 'app/schedule/page-content.tsx' },
  { path: '/applications', file: 'app/applications/page-content.tsx' },
  { path: '/chat-history', file: 'app/chat-history/page-content.tsx' },
  { path: '/correlation-diagram', file: 'app/correlation-diagram/page-content.tsx' },
  { path: '/es-rewrite', file: 'app/es-rewrite/page-content.tsx' },
  { path: '/company/[id]', file: 'app/company/[id]/page-content.tsx' },
  { path: '/whats-new', file: 'app/whats-new/whats-new-view.tsx' },
]

/**
 * 余白を持たない画面。理由を書くこと。
 * 「まだ直していない」で登録してはいけない。
 */
const EXEMPT: Record<string, string> = {
  // 100vh 固定のレイアウトで、そもそもページがスクロールしない。
  // レスポンシブ対応自体が別Issue（#1569）。
  '/interview': 'AI面接は100vh固定レイアウト。#1569 で別途対応',
  // チャット画面は独自に入力欄を下へ固定しており、共通の余白とは別管理。
  '/': 'チャットは入力欄を下部に固定しており独自管理',
}

describe('下部ナビが出る画面の余白 (#1570)', () => {
  it.each(SCREENS.filter((s) => !EXEMPT[s.path]))(
    '$path は下部ナビ分の余白を持つ',
    ({ path, file }) => {
      // 前提：この画面では実際に下部ナビが出る
      expect(shouldShowStudentBottomNav(path)).toBe(true)

      expect(existsSync(file)).toBe(true)
      const src = readFileSync(file, 'utf8')
      expect(src).toContain('BottomNavSpacer')
    },
  )

  it('画面下に固定表示する Snackbar はナビの上へ逃がす', () => {
    const withBottomSnackbar = [
      'app/applications/page-content.tsx',
      'app/results/components/ResultsListView.tsx',
    ]
    for (const file of withBottomSnackbar) {
      const src = readFileSync(file, 'utf8')
      // MUI 既定は下から24pxで、56pxのナビと重なる
      expect(src).toContain("anchorOrigin={{ vertical: 'bottom'")
      expect(src).toContain('ABOVE_BOTTOM_NAV_SX')
    }
  })

  it('余白は手書きせず共通部品に寄せる', () => {
    // pb: { xs: 7 } のような手書きは、ナビの高さが変わったときに追従しない
    const src = readFileSync('app/results/components/ResultsListView.tsx', 'utf8')
    expect(src).not.toMatch(/pb:\s*\{\s*xs:\s*7/)
  })
})
