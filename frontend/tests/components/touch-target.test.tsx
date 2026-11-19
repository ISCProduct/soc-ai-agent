/** @jest-environment jsdom */

import { createRef, type ReactNode } from 'react'
import { readFileSync } from 'fs'
import { join } from 'path'
import { fireEvent, render, screen } from '@testing-library/react'
import { IconButton, ThemeProvider, ToggleButton, ToggleButtonGroup, createTheme } from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import { createStudentMuiTheme, TOUCH_TARGET_MIN } from '@/lib/student-theme'
import ScoreUpdateBanner from '@/components/feedback/ScoreUpdateBanner'
import { ChatInputBar } from '@/components/mui-chat/components/ChatInputBar'
import { StudentThemeToggle } from '@/components/layout/StudentThemeToggle'
import ConsentDialog from '@/app/interview/components/ConsentDialog'

/**
 * 押す大きさが 44px 未満のボタンを直す（#2214 / SOCAIAGENT-636）。
 * jsdom はレイアウトを計算しないので、実際の大きさ（getBoundingClientRect）は測れない。
 * ここでは「44px 以上にする指定が付いていること」を、テーマの設定値・MUI のスタイル
 * （emotion が出す CSS を getComputedStyle で読む）・ソースの静的な検査で確かめる。
 */

const studentTheme = createStudentMuiTheme('comfortable')

function withStudentTheme(node: ReactNode) {
  return render(<ThemeProvider theme={studentTheme}>{node}</ThemeProvider>)
}

function source(path: string): string {
  return readFileSync(join(__dirname, '..', '..', path), 'utf8')
}

describe('学生のテーマ: IconButton / ToggleButton の押せる範囲', () => {
  it.each(['comfortable', 'high-contrast'] as const)('%s で最小の大きさが縦横 44px', (mode) => {
    const components = createStudentMuiTheme(mode).components
    expect(TOUCH_TARGET_MIN).toBe(44)
    expect(components?.MuiIconButton?.styleOverrides?.root).toEqual({ minWidth: 44, minHeight: 44 })
    expect(components?.MuiToggleButton?.styleOverrides?.root).toEqual({ minWidth: 44, minHeight: 44 })
  })

  it('size="small" の IconButton にも 44px の最小の大きさが付く（アイコンの大きさは変えない）', () => {
    withStudentTheme(
      <IconButton size="small" aria-label="閉じる">
        <CloseIcon fontSize="small" />
      </IconButton>,
    )
    const button = screen.getByRole('button', { name: '閉じる' })
    expect(button).toHaveStyle({ minWidth: '44px', minHeight: '44px' })
    expect(button).toHaveClass('MuiIconButton-sizeSmall')
    expect(button.querySelector('svg')).toHaveClass('MuiSvgIcon-fontSizeSmall')
  })

  it('学生のテーマを使わない画面（MUI 既定のテーマ）には付かない', () => {
    render(
      <ThemeProvider theme={createTheme()}>
        <IconButton size="small" aria-label="閉じる">
          <CloseIcon fontSize="small" />
        </IconButton>
      </ThemeProvider>,
    )
    expect(screen.getByRole('button', { name: '閉じる' })).not.toHaveStyle({ minHeight: '44px' })
  })

  it('size="small" の ToggleButton（相関図の切り替え）にも 44px の最小の大きさが付く', () => {
    withStudentTheme(
      <ToggleButtonGroup value="capital" exclusive size="small">
        <ToggleButton value="capital">資本関連図</ToggleButton>
        <ToggleButton value="business">ビジネス関連図</ToggleButton>
      </ToggleButtonGroup>,
    )
    expect(screen.getByRole('button', { name: '資本関連図' })).toHaveStyle({ minWidth: '44px', minHeight: '44px' })
    expect(screen.getByRole('button', { name: 'ビジネス関連図' })).toHaveStyle({ minHeight: '44px' })
  })

  it('スコア更新の通知を閉じるボタンが 44px 以上になる', () => {
    withStudentTheme(
      <ScoreUpdateBanner
        beforeScores={[{ weight_category: '論理性', score: 50 }]}
        afterScores={[{ weight_category: '論理性', score: 60 }]}
      />,
    )
    expect(screen.getByRole('button', { name: '通知を閉じる' })).toHaveStyle({ minWidth: '44px', minHeight: '44px' })
  })
})

describe('チャットの送信ボタン', () => {
  function renderBar() {
    return withStudentTheme(
      <ChatInputBar
        analysisComplete={false}
        showChoiceButtons={false}
        choiceOptions={[]}
        selectedChoiceValue={null}
        input="こんにちは"
        inputPlaceholder="メッセージを入力"
        isLoading={false}
        historyLoadError={null}
        canSend
        inputRef={createRef()}
        onInputChange={() => {}}
        onSelectChoice={() => {}}
        onSend={() => {}}
        onOtherChoice={() => {}}
        onShowCompletionModal={() => {}}
      />,
    )
  }

  it('44px の大きさで、名前は「メッセージを送信」のまま', () => {
    renderBar()
    const send = screen.getByRole('button', { name: 'メッセージを送信' })
    expect(send).toHaveStyle({ width: '44px', height: '44px' })
  })

  it('フォーカスすると Tooltip「送信」が出る', async () => {
    renderBar()
    const send = screen.getByRole('button', { name: 'メッセージを送信' })
    fireEvent.focus(send.parentElement as HTMLElement)
    fireEvent.mouseOver(send.parentElement as HTMLElement)
    expect(await screen.findByRole('tooltip')).toHaveTextContent('送信')
  })
})

describe('Checkbox / Radio の行の高さ', () => {
  it('画面の見え方の Radio は余白 10px（24px のアイコンと合わせて 44px）', () => {
    withStudentTheme(<StudentThemeToggle />)
    const radios = screen.getAllByRole('radio')
    expect(radios).toHaveLength(2)
    for (const radio of radios) {
      const root = radio.closest('.MuiRadio-root') as HTMLElement
      expect(root).toHaveStyle({ padding: '10px' })
    }
  })

  it('面接の同意の Checkbox は余白 10px', () => {
    withStudentTheme(
      <ConsentDialog open consentGiven={false} onConsentChange={() => {}} onClose={() => {}} onConfirm={() => {}} />,
    )
    const checkbox = screen.getByRole('checkbox', { name: '上記の内容に同意して面接練習を始める' })
    expect(checkbox.closest('.MuiCheckbox-root')).toHaveStyle({ padding: '10px' })
  })

  // 適性検査・能力検査の画面は読み込みに API が要るので、ソースで確かめる。
  it.each(['app/assessments/[sessionId]/page-content.tsx', 'app/ability-assessments/[sessionId]/page-content.tsx'])(
    '%s の選択肢の Radio は余白 10px',
    (path) => {
      const src = source(path)
      expect(src).toContain("control={<Radio sx={{ p: '10px' }} />}")
      expect(src).not.toContain('control={<Radio />}')
    },
  )
})

describe('ソースの静的な検査', () => {
  /** `<Tag` から次のタグの始まり（`<`）までの属性の部分を取り出す。 */
  function openingTags(src: string, tag: string): string[] {
    return src
      .split(`<${tag}`)
      .slice(1)
      .filter((rest) => /^[\s>]/.test(rest))
      .map((rest) => rest.slice(0, rest.indexOf('<') === -1 ? undefined : rest.indexOf('<')))
  }

  it('プロフィールの入力欄は size="small"（高さ約40px）にしない', () => {
    const tags = openingTags(source('app/profile/page-content.tsx'), 'TextField')
    expect(tags.length).toBeGreaterThanOrEqual(6)
    for (const tag of tags) {
      expect(tag).not.toContain('size="small"')
    }
  })

  it('面接履歴の期間の切り替えは文字 14px・高さ 44px', () => {
    const tags = openingTags(source('app/interview/history/page-content.tsx'), 'ToggleButton')
    expect(tags).toHaveLength(3)
    for (const tag of tags) {
      expect(tag).toContain('fontSize: 14')
      expect(tag).toContain('minHeight: 44')
    }
  })

  /** `<Chip` から `/>` までを取り出す。 */
  function chips(src: string): string[] {
    return src
      .split('<Chip')
      .slice(1)
      .filter((rest) => /^\s/.test(rest))
      .map((rest) => rest.slice(0, rest.indexOf('/>')))
  }

  it.each(['app/es-rewrite/components/EsInputForm.tsx', 'components/mui-chat/components/ChatMessageList.tsx'])(
    '%s の押せる Chip は高さ 44px、押せない Chip は変えない',
    (path) => {
      const all = chips(source(path))
      const clickable = all.filter((c) => c.includes('onClick'))
      expect(clickable.length).toBeGreaterThan(0)
      for (const c of clickable) {
        expect(c).toContain('height: 44')
      }
      for (const c of all.filter((c) => !c.includes('onClick'))) {
        expect(c).not.toContain('height: 44')
      }
    },
  )
})
