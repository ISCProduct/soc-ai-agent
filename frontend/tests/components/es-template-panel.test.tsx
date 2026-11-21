/**
 * @jest-environment jsdom
 */
import { useState } from 'react'
import { render, screen, fireEvent, within } from '@testing-library/react'
import EsTemplatePanel from '@/components/es/EsTemplatePanel'
import { ES_GUIDES, PLACEHOLDER_PATTERN, exampleTags, guideFor, insertIntoDraft, type EsQuestionType } from '@/lib/es-templates'

// SOCAIAGENT-613: 履歴書・ES のひな形と例文。

describe('辞書', () => {
  const types = Object.keys(ES_GUIDES) as EsQuestionType[]

  test('どの設問の種類にも、書く順番・書き出し・【 】入りのひな形がある', () => {
    for (const t of types) {
      const g = ES_GUIDES[t]
      expect(g.steps.length).toBeGreaterThanOrEqual(3)
      expect(g.openings.length).toBeGreaterThan(0)
      expect(PLACEHOLDER_PATTERN.test(g.template)).toBe(true)
    }
  })

  test('例文は400字程度に収まり、自分で埋める【 】を含み、題名が重ならない', () => {
    for (const t of types) {
      const titles = new Set<string>()
      for (const e of ES_GUIDES[t].examples) {
        expect(e.text.length).toBeLessThanOrEqual(420)
        expect(PLACEHOLDER_PATTERN.test(e.text)).toBe(true)
        expect(titles.has(e.title)).toBe(false)
        titles.add(e.title)
      }
    }
  })

  test('志望動機は業界、自己PR は職種、ガクチカは経験の種類で絞り込む。学チカとガクチカは同じ', () => {
    expect(ES_GUIDES['志望動機'].tagLabel).toBe('業界')
    expect(exampleTags(ES_GUIDES['志望動機'])).toContain('IT・ソフトウェア')
    expect(ES_GUIDES['自己PR'].tagLabel).toBe('職種')
    expect(ES_GUIDES['学チカ']).toBe(ES_GUIDES['ガクチカ'])
    expect(guideFor('知らない種類')).toBe(ES_GUIDES['その他'])
  })

  test('空なら置き換え、書きかけなら末尾に足し、上限を超えるなら入れない', () => {
    expect(insertIntoDraft('  \n', '例文', 100)).toBe('例文')
    expect(insertIntoDraft('書きかけ\n\n', '例文', 100)).toBe('書きかけ\n\n例文')
    expect(insertIntoDraft('あ'.repeat(95), '例文です', 100)).toBeNull()
  })
})

function Harness({ initial = '', questionType = '志望動機', maxLength = 6000 }: { initial?: string; questionType?: string; maxLength?: number }) {
  const [draft, setDraft] = useState(initial)
  return (
    <>
      <EsTemplatePanel questionType={questionType} draft={draft} maxLength={maxLength} onInsert={setDraft} />
      <textarea aria-label="本文" value={draft} readOnly />
    </>
  )
}

describe('書き出しガイドと例文の欄', () => {
  test('白紙なら開いた状態で、ひな形を入力欄に入れられる', () => {
    render(<Harness />)
    expect(screen.getByText('書く順番')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'ひな形を入力欄に入れる' }))
    expect((screen.getByLabelText('本文') as HTMLTextAreaElement).value).toBe(ES_GUIDES['志望動機'].template)
  })

  test('書きかけなら閉じた状態で、開くと例文を末尾に足す（書いた文章を消さない）', () => {
    render(<Harness initial="私は" />)
    expect(screen.queryByText('書く順番')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /書き出しガイドと例文/ }))
    const first = ES_GUIDES['志望動機'].examples[0]
    fireEvent.click(screen.getByRole('button', { name: `例文「${first.title}」を入力欄の末尾に足す` }))
    expect((screen.getByLabelText('本文') as HTMLTextAreaElement).value).toBe(`私は\n\n${first.text}`)
  })

  test('例文を業界で絞り込める', async () => {
    render(<Harness />)
    expect(screen.getAllByRole('button', { name: /^例文「/ })).toHaveLength(ES_GUIDES['志望動機'].examples.length)
    fireEvent.mouseDown(screen.getByLabelText('業界'))
    fireEvent.click(await screen.findByRole('option', { name: '金融' }))
    const buttons = screen.getAllByRole('button', { name: /^例文「/ })
    expect(buttons).toHaveLength(1)
    expect(buttons[0]).toHaveAccessibleName(/金融/)
  })

  test('上限を超えるときは入れずに知らせる', () => {
    render(<Harness initial={'あ'.repeat(50)} maxLength={60} />)
    fireEvent.click(screen.getByRole('button', { name: /書き出しガイドと例文/ }))
    fireEvent.click(screen.getByRole('button', { name: 'ひな形を入力欄の末尾に足す' }))
    expect(screen.getByText(/上限（60文字）を超える/)).toBeInTheDocument()
    expect((screen.getByLabelText('本文') as HTMLTextAreaElement).value).toBe('あ'.repeat(50))
  })

  test('例文の無い種類（その他）でも、ひな形と注記は出す', () => {
    render(<Harness questionType="その他" />)
    const panel = screen.getByText('書く順番').closest('div') as HTMLElement
    expect(within(panel).queryByText('例文')).not.toBeInTheDocument()
    expect(screen.getByText(/そのまま提出しないでください/)).toBeInTheDocument()
  })
})
