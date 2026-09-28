/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import ESRewritePage from '@/app/es-rewrite/page-content'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() }),
  useSearchParams: () => new URLSearchParams('company_name=%E3%82%B5%E3%83%B3%E3%83%97%E3%83%AB%E6%A0%AA%E5%BC%8F%E4%BC%9A%E7%A4%BE'),
}))

describe('ESRewritePage', () => {
  it('company_nameクエリで志望企業名欄がプリフィルされる（結果カードからのES CTA導線）', () => {
    render(<ESRewritePage />)

    expect(screen.getByLabelText(/志望企業名/)).toHaveValue('サンプル株式会社')
  })
})

describe('ESRewritePage エラー表示 (#1015)', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('プロキシがJSONエラーを返した場合、日本語の短いメッセージを表示する（生JSONは見せない）', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      json: async () => ({ error: 'ログインの有効期限が切れました。再度ログインしてください。', status: 401 }),
    })

    render(<ESRewritePage />)

    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '書き直す' }))

    expect(
      await screen.findByText('ログインの有効期限が切れました。再度ログインしてください。'),
    ).toBeInTheDocument()
    expect(screen.queryByText(/^\{.*"error"/)).not.toBeInTheDocument()
  })

  it('プロキシがJSONを返さない場合はフォールバックの日本語文言を表示する', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      json: async () => {
        throw new Error('not json')
      },
    })

    render(<ESRewritePage />)

    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '書き直す' }))

    await waitFor(() => {
      expect(screen.getByText('リライトに失敗しました。再試行してください。')).toBeInTheDocument()
    })
  })
})


describe('ESRewritePage 企業適合性の表示 (#1524)', () => {
  const REVIEW_BASE = {
    specificity_score: 7,
    star_score: 6,
    length_balance_score: 5,
    feedback: 'フィードバック本文',
    improved_text: '改善後の文章',
  }

  const review = async (result: Record<string, unknown>, company?: string) => {
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => result })
    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '元の文章です。' },
    })
    if (company !== undefined) {
      fireEvent.change(screen.getByLabelText(/志望企業名/), { target: { value: company } })
    }
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))
    await screen.findByText('添削スコア')
  }

  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('企業情報を取得できなかったときは企業適合性を出さず、理由と次の操作を案内する', async () => {
    await review(
      { ...REVIEW_BASE, company_fit_score: null, company_strategy: null, company_context_source: 'none' },
      '株式会社サイバーエージェント',
    )

    // 0/10 に見える空のバーを出さない（低評価と誤解させるため）
    expect(screen.queryByText('企業適合性')).not.toBeInTheDocument()
    expect(
      screen.getByText(/企業情報を取得できなかったため、企業適合性は評価していません/),
    ).toBeInTheDocument()
    expect(screen.getByText(/株式会社サイバーエージェント/)).toBeInTheDocument()
    expect(screen.queryByText(/への対策アドバイス/)).not.toBeInTheDocument()
    // 他の項目は通常どおり評価されている
    expect(screen.getByText('具体性')).toBeInTheDocument()
  })

  it('企業名未入力のときは入力を促す', async () => {
    await review(
      { ...REVIEW_BASE, company_fit_score: null, company_strategy: null, company_context_source: 'none' },
      '',
    )

    expect(screen.getByText(/企業名を入力して添削すると、企業適合性も評価します/)).toBeInTheDocument()
    expect(screen.queryByText(/企業情報を取得できなかったため/)).not.toBeInTheDocument()
  })

  it('企業情報を取得できたときは企業適合性と対策アドバイスを表示する', async () => {
    await review(
      {
        ...REVIEW_BASE,
        company_fit_score: 8,
        company_strategy: '求める人物像に沿って準備しましょう。',
        company_context_source: 'web_search',
      },
      '株式会社サイバーエージェント',
    )

    expect(screen.getByText('企業適合性')).toBeInTheDocument()
    expect(screen.getByText('8 / 10')).toBeInTheDocument()
    expect(screen.getByText(/への対策アドバイス/)).toBeInTheDocument()
    expect(screen.queryByText(/企業情報を取得できなかったため/)).not.toBeInTheDocument()

    // 添削後に入力欄だけ書き換えても、結果カードは添削に使った企業名を出し続ける
    fireEvent.change(screen.getByLabelText(/志望企業名/), { target: { value: '別の会社' } })
    expect(screen.getByText('🏢 株式会社サイバーエージェントへの対策アドバイス')).toBeInTheDocument()
    expect(screen.queryByText(/別の会社への対策アドバイス/)).not.toBeInTheDocument()
  })

  it('企業情報は取得できたが点数だけ出せないときは「見つかりませんでした」と言わない (H-2)', async () => {
    // _clamp_company_fit がモデルの非数値回答を null にするため、
    // company_context_source が none 以外でも fit が null になるレスポンスが実在する
    await review(
      {
        ...REVIEW_BASE,
        company_fit_score: null,
        company_strategy: '企業情報に基づく対策',
        company_context_source: 'web_search',
      },
      '株式会社テスト',
    )

    expect(screen.getByText(/今回は企業適合性の点数を算出できませんでした/)).toBeInTheDocument()
    expect(screen.queryByText(/公開情報が見つかりませんでした/)).not.toBeInTheDocument()
    // 取得できた企業情報を根拠にした対策アドバイスは出したまま（説明と矛盾させない）
    expect(screen.getByText(/への対策アドバイス/)).toBeInTheDocument()
  })

  it('422は detail の案内文をそのまま表示する（BFFの一般文で上書きしない / #1521）', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      status: 422,
      json: async () => ({
        error: '処理に失敗しました。しばらくしてから再試行してください。',
        status: 422,
        detail: '文章が長すぎて添削できませんでした。文字数を減らしてお試しください。',
      }),
    })

    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '長い文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    expect(
      await screen.findByText('文章が長すぎて添削できませんでした。文字数を減らしてお試しください。'),
    ).toBeInTheDocument()
  })
})


describe('ESRewritePage 422の生JSON表示防止 (#1015 / #1521)', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('detailが文字列でない422では生の本文を表示せず、一般文にフォールバックする', async () => {
    const longText = 'あ'.repeat(10001)
    // api-proxy は本文に文字列 detail が無いとレスポンス本文そのものを detail に入れる
    const rawBody = JSON.stringify({
      detail: [
        {
          type: 'string_too_long',
          loc: ['body', 'es_text'],
          msg: 'String should have at most 10000 characters',
          input: longText,
        },
      ],
    })
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: false,
      status: 422,
      json: async () => ({
        error: '処理に失敗しました。しばらくしてから再試行してください。',
        status: 422,
        detail: rawBody,
      }),
    })

    render(<ESRewritePage />)
    fireEvent.click(screen.getByRole('button', { name: 'ES添削' }))
    fireEvent.change(screen.getByPlaceholderText(/チームで開発した経験があります/), {
      target: { value: '長い文章です。' },
    })
    fireEvent.click(screen.getByRole('button', { name: '添削する' }))

    expect(
      await screen.findByText('処理に失敗しました。しばらくしてから再試行してください。'),
    ).toBeInTheDocument()
    expect(screen.queryByText(/string_too_long/)).not.toBeInTheDocument()
    expect(screen.queryByText(new RegExp(longText.slice(0, 40)))).not.toBeInTheDocument()
  })

  it('ES本文の入力欄はAPIの上限(10,000字)で止める', () => {
    render(<ESRewritePage />)

    expect(screen.getByPlaceholderText(/チームで開発した経験があります/)).toHaveAttribute(
      'maxlength',
      '10000',
    )
  })
})
