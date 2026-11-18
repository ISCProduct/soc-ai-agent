/** @jest-environment jsdom */
import { fireEvent, render, screen } from '@testing-library/react'
import { ConsultHome } from '@/components/home/ConsultHome'
import { CONSULT_OPTIONS } from '@/lib/career-consult'

const progress = { valid: 0, required: 15, percent: 0 }

function setup(overrides: Partial<React.ComponentProps<typeof ConsultHome>> = {}) {
  const onSend = jest.fn()
  render(<ConsultHome displayName="山田 太郎" onSend={onSend} progress={progress} {...overrides} />)
  return { onSend }
}

describe('ConsultHome（診断を始める前のホーム）', () => {
  it('挨拶・4択・自由入力・進捗帯を、Figma と同じ構成で出す', () => {
    setup()
    expect(screen.getByRole('heading', { level: 1, name: '山田さん、こんにちは' })).toBeInTheDocument()
    expect(screen.getByText('進路について、気軽に相談できます。')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'はじめに、いまの気持ちに近いものを選んでください。' })).toBeInTheDocument()
    for (const option of CONSULT_OPTIONS) {
      expect(screen.getByRole('button', { name: option })).toBeInTheDocument()
    }
    expect(screen.getByText('入力した内容にもとづいて、次の質問が変わります。')).toBeInTheDocument()
    expect(screen.getByText('質問 0 / 15 問（0%）')).toBeInTheDocument()
    expect(screen.getByText('答えると、希望に近い企業が探せます。')).toBeInTheDocument()
  })

  it('4択は名前付きのまとまり（group）で、選択肢は4つだけ', () => {
    setup()
    const group = screen.getByRole('group', { name: 'はじめに、いまの気持ちに近いものを選んでください。' })
    expect(group.querySelectorAll('button')).toHaveLength(4)
  })

  it('選択肢を押すと、その文言を回答として送る', () => {
    const { onSend } = setup()
    fireEvent.click(screen.getByRole('button', { name: '就活の進め方がわからない' }))
    expect(onSend).toHaveBeenCalledTimes(1)
    expect(onSend).toHaveBeenCalledWith('就活の進め方がわからない')
  })

  it('自由入力は、入力した文章（前後の空白を除く）を送り、入力欄を空にする', () => {
    const { onSend } = setup()
    const input = screen.getByRole('textbox', { name: /自分の言葉で入力する/ })
    fireEvent.change(input, { target: { value: '  情報系の仕事に就きたい  ' } })
    fireEvent.click(screen.getByRole('button', { name: '送る' }))
    expect(onSend).toHaveBeenCalledWith('情報系の仕事に就きたい')
    expect(input).toHaveValue('')
  })

  it('入力が空・空白だけのときは「送る」を押せず、送らない', () => {
    const { onSend } = setup()
    const submit = screen.getByRole('button', { name: '送る' })
    expect(submit).toBeDisabled()
    fireEvent.change(screen.getByRole('textbox', { name: /自分の言葉で入力する/ }), { target: { value: '   ' } })
    expect(submit).toBeDisabled()
    fireEvent.submit(submit.closest('form') as HTMLFormElement)
    expect(onSend).not.toHaveBeenCalled()
  })

  it('送信中は、選択肢・入力・送るを無効にして、二重に送らない', () => {
    const { onSend } = setup({ disabled: true })
    for (const option of CONSULT_OPTIONS) {
      expect(screen.getByRole('button', { name: option })).toBeDisabled()
    }
    expect(screen.getByRole('textbox', { name: /自分の言葉で入力する/ })).toBeDisabled()
    expect(screen.getByRole('button', { name: '送る' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: CONSULT_OPTIONS[0] }))
    expect(onSend).not.toHaveBeenCalled()
  })

  it('進捗帯は、渡された進捗を出す', () => {
    setup({ progress: { valid: 6, required: 15, percent: 40 } })
    expect(screen.getByText('質問 6 / 15 問（40%）')).toBeInTheDocument()
  })
})
